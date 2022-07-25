package http

import (
	"fmt"
	"strings"

	"net/http"
)

// ErrValidationFailed indicates that a parameter validation
// failed and the response should be a BadRequest.
type ErrValidationFailed struct {
	Param  string `json:"param"`
	Reason string `json:"reason"`
}

// Error implements the error interface
func (err *ErrValidationFailed) Error() string {
	return err.Reason
}

// NewErrMissingParam returns a new error idicating
// a missing query parameter.
func NewErrMissingParam(key string) *ErrValidationFailed {
	return &ErrValidationFailed{
		Param:  key,
		Reason: fmt.Sprintf("query parameter %s is missing", key),
	}
}

// NewErrAmbigousParam returns an ErrValidationFailed,
// indicating that the parameter was ambigous.
func NewErrAmbigousParam(key string) *ErrValidationFailed {
	return &ErrValidationFailed{
		Param:  key,
		Reason: fmt.Sprintf("query parameter %s is ambigous", key),
	}
}

// NewErrEmptyParam return an ErrValidationFailed if the
// provided paramter value is empty.
func NewErrEmptyParam(key string) *ErrValidationFailed {
	return &ErrValidationFailed{
		Param:  key,
		Reason: fmt.Sprintf("query parameter %s is empty", key),
	}
}

var (
	// ErrQueryTooShort will be returned when the query
	// is less than 2 characters.
	ErrQueryTooShort = &ErrValidationFailed{
		"q", "the query is too short",
	}

	// ErrQueryIncomplete will be returned when the
	// prefix query lacks a : or .
	ErrQueryIncomplete = &ErrValidationFailed{
		"q", "a prefix query must contain at least a '.' or ':'",
	}
)

// Helper: Validate source Id
func validateSourceID(id string) (string, error) {
	if len(id) > 42 {
		return "unknown", &ErrValidationFailed{
			Reason: fmt.Sprintf("source ID too long with length: %d", len(id)),
		}
	}
	return id, nil
}

// Helper: Validate query string
func validateQueryString(req *http.Request, key string) (string, error) {
	query := req.URL.Query()
	values, ok := query[key]
	if !ok {
		return "", NewErrMissingParam(key)
	}

	if len(values) != 1 {
		return "", NewErrAmbigousParam(key)
	}

	value := values[0]
	if value == "" {
		return "", NewErrEmptyParam(key)
	}

	return value, nil
}

// Helper: Validate prefix query. It should contain
// at least one dot or :
func validatePrefixQuery(value string) (string, error) {
	// We should at least provide 2 chars
	if len(value) < 2 {
		return "", ErrQueryTooShort
	}
	if !strings.Contains(value, ":") && !strings.Contains(value, ".") {
		return "", ErrQueryIncomplete
	}
	return value, nil
}

// Helper: Validate neighbors query. A valid query should have
// at least 4 chars.
func validateNeighborsQuery(value string) (string, error) {
	if len(value) < 3 {
		// Maybe make configurable,
		// A length of 3 would be sufficient for "DFN" and
		// other shorthands.
		return "", ErrQueryTooShort
	}
	return value, nil
}
