package tridentErrors

import "fmt"

type NotFoundError struct {
	message string
}

func (e *NotFoundError) Error() string { return e.message }

func NotFoundErr(format string, args ...interface{}) error {
	return &NotFoundError{fmt.Sprintf(format, args...)}
}

func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*NotFoundError)
	return ok
}
