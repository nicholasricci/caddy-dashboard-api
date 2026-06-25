package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/repository"
	"github.com/nicholasricci/caddy-dashboard/internal/scheduler"
	"github.com/nicholasricci/caddy-dashboard/internal/services"
	"github.com/nicholasricci/caddy-dashboard/internal/tasks"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ScheduledTaskHandler struct {
	repo      *repository.ScheduledTaskRepository
	logRepo   *repository.ScheduledTaskLogRepository
	scheduler *scheduler.Engine
	runners   map[string]tasks.TaskRunner
	audit     *services.AuditService
	logger    *zap.Logger
}

func NewScheduledTaskHandler(
	repo *repository.ScheduledTaskRepository,
	logRepo *repository.ScheduledTaskLogRepository,
	scheduler *scheduler.Engine,
	runners map[string]tasks.TaskRunner,
	audit *services.AuditService,
	logger *zap.Logger,
) *ScheduledTaskHandler {
	return &ScheduledTaskHandler{
		repo:      repo,
		logRepo:   logRepo,
		scheduler: scheduler,
		runners:   runners,
		audit:     audit,
		logger:    nopLogger(logger),
	}
}

// ListScheduledTasks godoc
// @Summary List all scheduled tasks
// @Tags scheduler
// @Produce json
// @Success 200 {object} models.ListScheduledTasksResponse
// @Security BearerAuth
// @Router /api/v1/scheduled-tasks [get]
func (h *ScheduledTaskHandler) List(c *gin.Context) {
	tasks, err := h.repo.List(c.Request.Context())
	if err != nil {
		logRequestError(h.logger, c, "list scheduled tasks failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list tasks"})
		return
	}
	if tasks == nil {
		tasks = []models.ScheduledTask{}
	}
	c.JSON(http.StatusOK, models.ListScheduledTasksResponse{Items: tasks})
}

// GetScheduledTask godoc
// @Summary Get a scheduled task by ID
// @Tags scheduler
// @Produce json
// @Param id path string true "Task ID"
// @Success 200 {object} models.ScheduledTask
// @Failure 404 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/scheduled-tasks/{id} [get]
func (h *ScheduledTaskHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}

	task, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "task not found"})
			return
		}
		logRequestError(h.logger, c, "get scheduled task failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to get task"})
		return
	}
	c.JSON(http.StatusOK, task)
}

// CreateScheduledTask godoc
// @Summary Create a new scheduled task
// @Tags scheduler
// @Accept json
// @Produce json
// @Param body body models.CreateScheduledTaskInput true "Task input"
// @Success 201 {object} models.ScheduledTask
// @Failure 400 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/scheduled-tasks [post]
func (h *ScheduledTaskHandler) Create(c *gin.Context) {
	var input models.CreateScheduledTaskInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	task := &models.ScheduledTask{
		Name:           input.Name,
		Description:    input.Description,
		TaskType:       input.TaskType,
		CronExpression: input.CronExpression,
		Config:         input.Config,
		Enabled:        enabled,
	}

	if err := h.repo.Create(c.Request.Context(), task); err != nil {
		if repository.IsDuplicate(err) {
			c.JSON(http.StatusConflict, models.ErrorResponse{Error: "task name already exists"})
			return
		}
		logRequestError(h.logger, c, "create scheduled task failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create task"})
		return
	}

	logAuditFailure(h.logger, c, "create", "scheduled_task", task.ID.String(), h.audit.Record(
		c.Request.Context(), c.GetString("username"), "create", "scheduled_task", task.ID.String(), task,
	))

	if err := h.scheduler.Refresh(c.Request.Context()); err != nil {
		logRequestError(h.logger, c, "scheduler refresh failed", err)
	}

	c.JSON(http.StatusCreated, task)
}

// UpdateScheduledTask godoc
// @Summary Update a scheduled task
// @Tags scheduler
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Param body body models.UpdateScheduledTaskInput true "Update input"
// @Success 200 {object} models.ScheduledTask
// @Failure 404 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/scheduled-tasks/{id} [put]
func (h *ScheduledTaskHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}

	var input models.UpdateScheduledTaskInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	task, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "task not found"})
			return
		}
		logRequestError(h.logger, c, "get scheduled task failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to get task"})
		return
	}

	if input.Name != nil {
		task.Name = *input.Name
	}
	if input.Description != nil {
		task.Description = *input.Description
	}
	if input.TaskType != nil {
		task.TaskType = *input.TaskType
	}
	if input.CronExpression != nil {
		task.CronExpression = *input.CronExpression
	}
	if input.Config != nil {
		task.Config = input.Config
	}
	if input.Enabled != nil {
		task.Enabled = *input.Enabled
	}

	if err := h.repo.Update(c.Request.Context(), task); err != nil {
		if repository.IsDuplicate(err) {
			c.JSON(http.StatusConflict, models.ErrorResponse{Error: "task name already exists"})
			return
		}
		logRequestError(h.logger, c, "update scheduled task failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update task"})
		return
	}

	logAuditFailure(h.logger, c, "update", "scheduled_task", task.ID.String(), h.audit.Record(
		c.Request.Context(), c.GetString("username"), "update", "scheduled_task", task.ID.String(), task,
	))

	if err := h.scheduler.Refresh(c.Request.Context()); err != nil {
		logRequestError(h.logger, c, "scheduler refresh failed", err)
	}

	c.JSON(http.StatusOK, task)
}

// DeleteScheduledTask godoc
// @Summary Delete a scheduled task
// @Tags scheduler
// @Param id path string true "Task ID"
// @Success 200 {object} models.MessageResponse
// @Failure 404 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/scheduled-tasks/{id} [delete]
func (h *ScheduledTaskHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}

	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "task not found"})
			return
		}
		logRequestError(h.logger, c, "delete scheduled task failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete task"})
		return
	}

	logAuditFailure(h.logger, c, "delete", "scheduled_task", id.String(), h.audit.Record(
		c.Request.Context(), c.GetString("username"), "delete", "scheduled_task", id.String(), nil,
	))

	if err := h.scheduler.Refresh(c.Request.Context()); err != nil {
		logRequestError(h.logger, c, "scheduler refresh failed", err)
	}

	c.JSON(http.StatusOK, models.MessageResponse{Message: "task deleted"})
}

type toggleScheduledTaskRequest struct {
	Enabled bool `json:"enabled"`
}

// ToggleScheduledTask godoc
// @Summary Enable or disable a scheduled task
// @Tags scheduler
// @Accept json
// @Param id path string true "Task ID"
// @Param body body toggleScheduledTaskRequest true "Toggle payload"
// @Success 200 {object} models.MessageResponse
// @Security BearerAuth
// @Router /api/v1/scheduled-tasks/{id}/toggle [post]
func (h *ScheduledTaskHandler) Toggle(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}

	var body toggleScheduledTaskRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.repo.Toggle(c.Request.Context(), id, body.Enabled); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "task not found"})
			return
		}
		logRequestError(h.logger, c, "toggle scheduled task failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to toggle task"})
		return
	}

	logAuditFailure(h.logger, c, "toggle", "scheduled_task", id.String(), h.audit.Record(
		c.Request.Context(), c.GetString("username"), "toggle", "scheduled_task", id.String(), gin.H{"enabled": body.Enabled},
	))

	if err := h.scheduler.Refresh(c.Request.Context()); err != nil {
		logRequestError(h.logger, c, "scheduler refresh failed", err)
	}

	c.JSON(http.StatusOK, models.MessageResponse{Message: "task updated"})
}

// RunNowScheduledTask godoc
// @Summary Execute a scheduled task immediately
// @Tags scheduler
// @Param id path string true "Task ID"
// @Success 200 {object} models.ScheduledTaskLog
// @Failure 404 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/scheduled-tasks/{id}/run-now [post]
func (h *ScheduledTaskHandler) RunNow(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}

	task, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "task not found"})
			return
		}
		logRequestError(h.logger, c, "get scheduled task failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to get task"})
		return
	}

	runner, ok := h.runners[task.TaskType]
	if !ok {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "unknown task type: " + task.TaskType})
		return
	}

	logEntry := &models.ScheduledTaskLog{
		ScheduledTaskID: task.ID,
		StartedAt:       time.Now().UTC(),
		Status:          models.ScheduledTaskStatusRunning,
	}
	if err := h.logRepo.Create(c.Request.Context(), logEntry); err != nil {
		logRequestError(h.logger, c, "create task log failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create task log"})
		return
	}

	result, err := runner.Run(c.Request.Context(), task.Config)
	now := time.Now().UTC()
	if err != nil {
		logEntry.Status = models.ScheduledTaskStatusFailed
		logEntry.Error = err.Error()
		logEntry.FinishedAt = &now
	} else {
		logEntry.Status = result.Status
		logEntry.Error = result.Error
		logEntry.Details = result.Details
		logEntry.FinishedAt = &now
	}
	_ = h.logRepo.Update(c.Request.Context(), logEntry)
	_ = h.repo.UpdateLastRun(c.Request.Context(), task.ID, now, logEntry.Status, logEntry.Error)

	logAuditFailure(h.logger, c, "run_now", "scheduled_task", task.ID.String(), h.audit.Record(
		c.Request.Context(), c.GetString("username"), "run_now", "scheduled_task", task.ID.String(), gin.H{
			"task_name": task.Name,
			"status":    logEntry.Status,
		},
	))

	c.JSON(http.StatusOK, logEntry)
}

// ListScheduledTaskLogs godoc
// @Summary List execution logs for a scheduled task
// @Tags scheduler
// @Produce json
// @Param id path string true "Task ID"
// @Param status query string false "Filter by status (running|success|failed)"
// @Param from query string false "Include logs started at or after (RFC3339)"
// @Param to query string false "Include logs started at or before (RFC3339)"
// @Param limit query int false "Page size (default 20, max 100)"
// @Param offset query int false "Page offset (default 0)"
// @Success 200 {object} models.ListScheduledTaskLogsResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/scheduled-tasks/{id}/logs [get]
func (h *ScheduledTaskHandler) ListLogs(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}

	if _, err := h.repo.GetByID(c.Request.Context(), id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "task not found"})
			return
		}
		logRequestError(h.logger, c, "get scheduled task failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to get task"})
		return
	}

	filter, err := parseScheduledTaskLogListFilter(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	limit, offset := parseLimitOffset(c)
	logs, total, err := h.logRepo.ListByTaskIDPaginated(c.Request.Context(), id, filter, limit, offset)
	if err != nil {
		logRequestError(h.logger, c, "list task logs failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list logs"})
		return
	}
	if logs == nil {
		logs = []models.ScheduledTaskLog{}
	}
	c.JSON(http.StatusOK, models.ListScheduledTaskLogsResponse{
		Items: logs,
		Meta:  models.PaginationMeta{Total: total, Limit: limit, Offset: offset},
	})
}

func parseScheduledTaskLogListFilter(c *gin.Context) (models.ScheduledTaskLogListFilter, error) {
	var filter models.ScheduledTaskLogListFilter

	if v := strings.TrimSpace(c.Query("status")); v != "" {
		if !models.IsValidScheduledTaskLogStatus(v) {
			return filter, errors.New("invalid status")
		}
		filter.Status = v
	}

	fromRaw := strings.TrimSpace(c.Query("from"))
	toRaw := strings.TrimSpace(c.Query("to"))
	if fromRaw != "" {
		from, err := time.Parse(time.RFC3339, fromRaw)
		if err != nil {
			return filter, errors.New("invalid from: expected RFC3339 timestamp")
		}
		filter.From = &from
	}
	if toRaw != "" {
		to, err := time.Parse(time.RFC3339, toRaw)
		if err != nil {
			return filter, errors.New("invalid to: expected RFC3339 timestamp")
		}
		filter.To = &to
	}
	if filter.From != nil && filter.To != nil && filter.From.After(*filter.To) {
		return filter, errors.New("from must be before or equal to to")
	}

	return filter, nil
}

var _ = (*ScheduledTaskHandler)(nil)
