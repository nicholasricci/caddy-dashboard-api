package services

import "errors"

// ErrInvalidNodePayload means the node fields are inconsistent with the chosen transport.
var ErrInvalidNodePayload = errors.New("invalid node payload")
