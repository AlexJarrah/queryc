package runtime

import "errors"

// Errors shared by dialect runtimes.
var (
	ErrNoResults      = errors.New("no results")
	ErrTooManyResults = errors.New("expected one result, got many")
	ErrInvalidQuery   = errors.New("invalid query")
	ErrFailedBeginTx  = errors.New("failed to begin transaction")
	ErrFailedCommitTx = errors.New("failed to commit transaction")
)
