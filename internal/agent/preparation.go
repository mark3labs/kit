package agent

import "errors"

// PreparationError distinguishes host preparation failures from provider errors.
// Unwrap preserves the host's original error identity.
type PreparationError struct{ Err error }

func (e *PreparationError) Error() string { return e.Err.Error() }
func (e *PreparationError) Unwrap() error { return e.Err }

func preparationFailure(err, preparation error) error {
	if preparation != nil {
		return &PreparationError{Err: preparation}
	}
	return err
}

// IsPreparationError reports whether recovery must be skipped.
func IsPreparationError(err error) bool {
	var preparation *PreparationError
	return errors.As(err, &preparation)
}
