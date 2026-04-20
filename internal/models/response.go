package models

type ErrorResponse struct {
	Error string `json:"error"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

type BackfillSnapshotsResponse struct {
	RowsUpdated int64 `json:"rows_updated" example:"42"`
	DurationMs  int64 `json:"duration_ms" example:"123"`
}

type PaginationMeta struct {
	Total  int64 `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}
