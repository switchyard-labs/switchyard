package artifacts

import (
	"errors"
	"net"
	"net/http"
)

// Error carries a safe upstream classification, never an upstream response body.
type Error struct {
	Code       string
	Status     int
	RetryAfter string
}

func (e *Error) Error() string { return e.Code }
func responseError(response *http.Response) error {
	e := &Error{Code: "artifacts_unavailable", Status: http.StatusBadGateway, RetryAfter: response.Header.Get("Retry-After")}
	switch response.StatusCode {
	case 404:
		e.Code = "repository_not_found"
		e.Status = 404
	case 429:
		e.Code = "upstream_rate_limited"
		e.Status = 429
	case 401, 403:
		e.Code = "artifacts_auth_failed"
		e.Status = 502
	case 408, 504:
		e.Code = "upstream_timeout"
		e.Status = 504
	case 503:
		e.Status = 503
	}
	return e
}
func transportError(err error) error {
	var timeout net.Error
	if errors.As(err, &timeout) && timeout.Timeout() {
		return &Error{Code: "upstream_timeout", Status: 504}
	}
	return &Error{Code: "artifacts_unavailable", Status: 502}
}
