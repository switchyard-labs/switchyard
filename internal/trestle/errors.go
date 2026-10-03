package trestle

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var ErrResponseTooLarge = errors.New("Trestle response exceeds 16 MiB")
var ErrConflict = errors.New("coordination conflict")
var ErrValidation = errors.New("coordination validation failed")

type APIError struct {
	Status                               int
	Operation, Collection, Code, Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("trestle %s %s: %d %s: %s", e.Operation, e.Collection, e.Status, e.Code, e.Message)
}
func (e *APIError) Conflict() bool   { return e.Status == 409 || e.Status == 412 }
func (e *APIError) Validation() bool { return e.Status == 400 || e.Status == 422 }
func Status(err error) int {
	var api *APIError
	if errors.As(err, &api) {
		return api.Status
	}
	return 0
}
func responseError(op, collection string, status int, body []byte) error {
	var envelope struct {
		Error json.RawMessage `json:"error"`
	}
	var detail struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	json.Unmarshal(body, &envelope)
	json.Unmarshal(envelope.Error, &detail)
	if detail.Code == "" {
		detail.Code = "request_failed"
	}
	if detail.Message == "" {
		detail.Message = "coordination request failed"
	}
	return &APIError{Status: status, Operation: op, Collection: collection, Code: detail.Code, Message: detail.Message}
}
func readResponse(reader io.Reader) ([]byte, error) {
	const limit = 16 << 20
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if len(body) > limit {
		return nil, ErrResponseTooLarge
	}
	return body, nil
}

func (e *APIError) Is(target error) bool {
	return target == ErrConflict && e.Conflict() || target == ErrValidation && e.Validation()
}
