package tasks

import (
	"context"
	"encoding/json"
	"time"
)

type tokenCleanupTask struct {
	authSvc interface {
		CleanupExpiredRefreshTokens(ctx context.Context) error
	}
}

func NewTokenCleanupTask(authSvc interface {
	CleanupExpiredRefreshTokens(ctx context.Context) error
}) TaskRunner {
	return &tokenCleanupTask{authSvc: authSvc}
}

var _ TaskRunner = (*tokenCleanupTask)(nil)

func (t *tokenCleanupTask) Name() string {
	return "token_cleanup"
}

func (t *tokenCleanupTask) Run(ctx context.Context, _ json.RawMessage) (*TaskResult, error) {
	start := time.Now()
	if err := t.authSvc.CleanupExpiredRefreshTokens(ctx); err != nil {
		return FailedResult(err), nil
	}
	return SuccessResult(map[string]any{
		"duration_ms": durationMs(start),
	}), nil
}
