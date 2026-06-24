package tasks

import (
	"context"
	"encoding/json"
	"time"
)

type TaskResult struct {
	Status  string          `json:"status"`
	Error   string          `json:"error,omitempty"`
	Details json.RawMessage `json:"details,omitempty"`
}

type TaskRunner interface {
	Name() string
	Run(ctx context.Context, config json.RawMessage) (*TaskResult, error)
}

func SuccessResult(details any) *TaskResult {
	raw, _ := json.Marshal(details)
	return &TaskResult{Status: "success", Details: raw}
}

func FailedResult(err error) *TaskResult {
	return &TaskResult{Status: "failed", Error: err.Error()}
}

func FailedString(msg string) *TaskResult {
	return &TaskResult{Status: "failed", Error: msg}
}

func durationMs(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}
