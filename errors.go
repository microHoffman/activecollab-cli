package activecollab

import (
	"errors"
	"fmt"

	"github.com/microHoffman/activecollab-cli/internal/transport"
)

type APIError struct {
	StatusCode int
	Type       string
	Message    string
}

func (e *APIError) Error() string {
	if e.Type != "" {
		return fmt.Sprintf("ActiveCollab API returned HTTP %d (%s): %s", e.StatusCode, e.Type, e.Message)
	}
	return fmt.Sprintf("ActiveCollab API returned HTTP %d: %s", e.StatusCode, e.Message)
}

func normalizeError(err error) error {
	var responseError *transport.ResponseError
	if errors.As(err, &responseError) {
		return &APIError{
			StatusCode: responseError.StatusCode,
			Type:       responseError.Type,
			Message:    responseError.Message,
		}
	}
	return err
}
