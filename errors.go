package apishell

import "errors"

var (
	ErrInvalidConfig    = errors.New("invalid config")
	ErrInvalidCommand   = errors.New("invalid command")
	ErrDuplicateCommand = errors.New("duplicate command")
	ErrCommandNotFound  = errors.New("command not found")
)

func failure(code, message string, details any) Result {
	result := Result{OK: false}
	result.Error = &ResultError{Code: code, Message: message, Details: details}
	return result
}
