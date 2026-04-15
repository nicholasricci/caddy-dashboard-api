package models

import "errors"

// ErrNodeNotFound is returned when a Caddy node ID does not exist in persistence.
var ErrNodeNotFound = errors.New("node not found")
