package off

import (
	"errors"
	"fmt"
)

var (
	// ErrProductNotFound is returned when OFF responds with status:0.
	ErrProductNotFound = errors.New("product not found")
	// ErrUpstreamTimeout is returned when the OFF request times out.
	ErrUpstreamTimeout = errors.New("upstream timeout")
	// ErrUpstreamServerError is returned for OFF 5xx responses.
	ErrUpstreamServerError = errors.New("upstream server error")
)

// UnexpectedStatusError is returned for OFF responses that are 4xx (other than
// the documented status:0 not-found) — anything we did not anticipate.
type UnexpectedStatusError struct {
	StatusCode int
}

func (e *UnexpectedStatusError) Error() string {
	return fmt.Sprintf("upstream unexpected response: HTTP %d", e.StatusCode)
}

// IsUnexpectedStatus reports whether err is an UnexpectedStatusError.
func IsUnexpectedStatus(err error) (*UnexpectedStatusError, bool) {
	var u *UnexpectedStatusError
	if errors.As(err, &u) {
		return u, true
	}
	return nil, false
}
