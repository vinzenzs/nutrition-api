package cookidoo

import (
	"errors"
	"fmt"
)

var (
	// ErrNotCookidooURL is returned when the supplied URL is not a recognised
	// Cookidoo recipe URL. Returned before any outbound request is made.
	ErrNotCookidooURL = errors.New("not a cookidoo recipe url")
	// ErrNoRecipeJSONLD is returned when the fetched page contains no Schema.org
	// Recipe JSON-LD block (the page exists but isn't a parseable recipe).
	ErrNoRecipeJSONLD = errors.New("no recipe json-ld on page")
)

// ErrFetchFailed wraps a transport-level or HTTP-status failure fetching the
// page. Distinct from ErrNoRecipeJSONLD so the handler can tell "couldn't
// reach/​read the page" apart from "page had no recipe data".
type ErrFetchFailed struct {
	// StatusCode is the HTTP status when the failure was a non-2xx response;
	// 0 when the request never completed (timeout, DNS, connection refused).
	StatusCode int
	Err        error
}

func (e *ErrFetchFailed) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("cookidoo fetch failed: HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("cookidoo fetch failed: %v", e.Err)
}

func (e *ErrFetchFailed) Unwrap() error { return e.Err }
