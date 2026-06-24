package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	cron "github.com/netresearch/go-cron"
	"github.com/nicholasricci/caddy-dashboard/internal/config"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/repository"
	"github.com/nicholasricci/caddy-dashboard/internal/tasks"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Engine struct {
	mu        sync.Mutex
	cron      *cron.Cron
	repo      *repository.ScheduledTaskRepository
	logRepo   *repository.ScheduledTaskLogRepository
	runners   map[string]tasks.TaskRunner
	db        *gorm.DB
	cfg       config.SchedulerConfig
	logger    *zap.Logger
	entries   map[uuid.UUID]cron.EntryID
	taskLocks sync.Map
}

type EngineDeps struct {
	Repo    *repository.ScheduledTaskRepository
	LogRepo *repository.ScheduledTaskLogRepository
	Config  config.SchedulerConfig
	Logger  *zap.Logger
	DB      *gorm.DB
	Runners map[string]tasks.TaskRunner
}

func NewEngine(deps EngineDeps) *Engine {
	return &Engine{
		repo:    deps.Repo,
		logRepo: deps.LogRepo,
		runners: deps.Runners,
		db:      deps.DB,
		cfg:     deps.Config,
		logger:  deps.Logger.Named("scheduler"),
		entries: make(map[uuid.UUID]cron.EntryID),
	}
}

func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.cron = cron.New(
		cron.WithLocation(time.UTC),
		cron.WithObservability(cron.ObservabilityHooks{
			OnJobStart: func(id cron.EntryID, name string, scheduled time.Time) {
				e.logger.Debug("job firing",
					zap.String("entry", name),
					zap.Time("scheduled", scheduled),
				)
			},
			OnJobComplete: func(id cron.EntryID, name string, dur time.Duration, recovered any) {
				if recovered != nil {
					e.logger.Warn("job panicked",
						zap.String("entry", name),
						zap.Duration("dur", dur),
						zap.Any("panic", recovered),
					)
				}
			},
		}),
	)

	if err := e.refreshLocked(ctx); err != nil {
		return fmt.Errorf("initial task load: %w", err)
	}

	e.cron.Start()
	e.logger.Info("scheduler started")

	go e.logCleanupLoop(ctx)

	return nil
}

func (e *Engine) Stop(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cron == nil {
		return nil
	}

	e.logger.Info("stopping scheduler")
	stopCtx := e.cron.Stop()

	select {
	case <-stopCtx.Done():
		e.logger.Info("scheduler stopped")
		return nil
	case <-ctx.Done():
		e.logger.Warn("scheduler stop timed out")
		return ctx.Err()
	}
}

func (e *Engine) Refresh(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.refreshLocked(ctx)
}

func (e *Engine) refreshLocked(ctx context.Context) error {
	tasks, err := e.repo.ListEnabled(ctx)
	if err != nil {
		return fmt.Errorf("list enabled tasks: %w", err)
	}

	currentIDs := make(map[string]bool)
	for _, task := range tasks {
		currentIDs[task.ID.String()] = true
		task := task
		runner, ok := e.runners[task.TaskType]
		if !ok {
			continue
		}

		entryID, err := e.cron.UpsertJob(task.CronExpression,
			cron.FuncJobWithContext(func(ctx context.Context) {
				e.executeJob(ctx, task, runner)
			}),
			cron.WithName(task.ID.String()),
		)
		if err != nil {
			e.logger.Warn("invalid cron expression",
				zap.String("task_id", task.ID.String()),
				zap.String("task_name", task.Name),
				zap.String("cron", task.CronExpression),
				zap.Error(err),
			)
			continue
		}
		e.entries[task.ID] = entryID
		e.logger.Debug("task scheduled",
			zap.String("task_id", task.ID.String()),
			zap.String("task_name", task.Name),
			zap.String("cron", task.CronExpression),
		)
	}

	for taskID, entryID := range e.entries {
		if !currentIDs[taskID.String()] {
			e.cron.Remove(entryID)
			delete(e.entries, taskID)
		}
	}

	e.logger.Info("scheduler refresh complete",
		zap.Int("active_tasks", len(e.entries)),
	)
	return nil
}

func (e *Engine) RunNow(ctx context.Context, taskID uuid.UUID) error {
	if entry := e.cron.EntryByName(taskID.String()); entry.Valid() && !entry.Paused {
		return e.cron.TriggerEntryByName(taskID.String())
	}

	task, err := e.repo.GetByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}

	runner, ok := e.runners[task.TaskType]
	if !ok {
		return fmt.Errorf("unknown task type: %s", task.TaskType)
	}

	e.executeJob(context.Background(), *task, runner)
	return nil
}

func (e *Engine) executeJob(ctx context.Context, task models.ScheduledTask, runner tasks.TaskRunner) {
	lock := e.getTaskLock(task.ID)
	if !lock.TryLock() {
		e.logger.Warn("task already running, skipping",
			zap.String("task_id", task.ID.String()),
			zap.String("task_name", task.Name),
		)
		return
	}
	defer lock.Unlock()

	taskCtx, cancel := context.WithTimeout(ctx, e.cfg.GlobalTimeout)
	defer cancel()

	locked, err := e.acquireLock(taskCtx, task.ID)
	if err != nil {
		e.logger.Error("mysql lock error",
			zap.String("task_id", task.ID.String()),
			zap.Error(err),
		)
		return
	}
	if !locked {
		e.logger.Debug("task locked by another replica, skipping",
			zap.String("task_id", task.ID.String()),
		)
		return
	}
	defer e.releaseLock(taskCtx, task.ID)

	logEntry := &models.ScheduledTaskLog{
		ScheduledTaskID: task.ID,
		StartedAt:       time.Now().UTC(),
		Status:          models.ScheduledTaskStatusRunning,
	}
	if err := e.logRepo.Create(ctx, logEntry); err != nil {
		e.logger.Error("failed to create task log", zap.Error(err))
		return
	}

	result, err := runner.Run(taskCtx, task.Config)

	now := time.Now().UTC()
	if err != nil {
		logEntry.Status = models.ScheduledTaskStatusFailed
		logEntry.Error = err.Error()
		logEntry.FinishedAt = &now
		_ = e.logRepo.Update(ctx, logEntry)
		_ = e.repo.UpdateLastRun(ctx, task.ID, now, models.ScheduledTaskStatusFailed, err.Error())
		e.logger.Error("task failed",
			zap.String("task_id", task.ID.String()),
			zap.String("task_name", task.Name),
			zap.Error(err),
		)
		return
	}

	logEntry.Status = result.Status
	logEntry.Error = result.Error
	logEntry.Details = result.Details
	logEntry.FinishedAt = &now
	_ = e.logRepo.Update(ctx, logEntry)
	_ = e.repo.UpdateLastRun(ctx, task.ID, now, result.Status, result.Error)

	if result.Status == models.ScheduledTaskStatusFailed {
		e.logger.Warn("task completed with errors",
			zap.String("task_id", task.ID.String()),
			zap.String("task_name", task.Name),
			zap.String("error", result.Error),
		)
	} else {
		e.logger.Info("task completed successfully",
			zap.String("task_id", task.ID.String()),
			zap.String("task_name", task.Name),
		)
	}
}

func (e *Engine) getTaskLock(taskID uuid.UUID) *sync.Mutex {
	actual, _ := e.taskLocks.LoadOrStore(taskID, &sync.Mutex{})
	return actual.(*sync.Mutex)
}

func (e *Engine) acquireLock(ctx context.Context, taskID uuid.UUID) (bool, error) {
	lockName := fmt.Sprintf("scheduler:task:%s", taskID.String())
	var result int
	err := e.db.WithContext(ctx).Raw("SELECT GET_LOCK(?, 0)", lockName).Scan(&result).Error
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (e *Engine) releaseLock(ctx context.Context, taskID uuid.UUID) {
	lockName := fmt.Sprintf("scheduler:task:%s", taskID.String())
	_ = e.db.WithContext(ctx).Exec("SELECT RELEASE_LOCK(?)", lockName)
}

func (e *Engine) logCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := e.logRepo.DeleteOlderThan(ctx, e.cfg.LogRetentionDays); err != nil {
				e.logger.Warn("log cleanup failed", zap.Error(err))
			}
		}
	}
}
