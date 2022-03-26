package generic

import "fmt"

type Result[T any] struct {
	Value T
	Error error
}

// AsResult wraps a (T, error) return value from another function call as a Result[T].
func AsResult[T any](value T, err error) Result[T] {
	return Result[T]{Value: value, Error: err}
}

// Err transforms the Result[T] into an Option[error], either Some(Error) or None().
func (r Result[T]) Err() Option[error] {
	if r.IsErr() {
		return Some(r.Error)
	} else {
		return None[error]()
	}
}

// Expect returns the contained value if IsOk(), or panics with the supplied error message and the contained error
// if IsErr().
func (r Result[T]) Expect(msg string) T {
	if r.IsOk() {
		return r.Value
	} else {
		panic(fmt.Errorf("%s: %w", msg, r.Error))
	}
}

// ExpectErr returns the contained error if IsErr(), or panics with the supplied error message if IsOk().
func (r Result[T]) ExpectErr(msg string) error {
	if r.IsErr() {
		return r.Error
	} else {
		panic(msg)
	}
}

// IsErr returns true if the Result[T] contains an error.
func (r *Result[T]) IsErr() bool {
	return r.Error != nil
}

// IsOk returns true if the Result[T] contains a value.
func (r *Result[T]) IsOk() bool {
	return r.Error == nil
}

// Ok transforms the Result[T] into an Option[T], either Some(Value) or None().
func (r Result[T]) Ok() Option[T] {
	if r.IsOk() {
		return Some(r.Value)
	} else {
		return None[T]()
	}
}

// Unwrap returns the contained value, or panics if IsErr.
func (r Result[T]) Unwrap() T {
	return r.Expect("tried to Unwrap() an Err")
}

// UnwrapErr returns the contained error, or panics if IsOk.
func (r Result[T]) UnwrapErr() error {
	return r.ExpectErr("tried to UnwrapErr() an Ok")
}

// UnwrapOr returns the contained value, or other if IsErr.
func (r Result[T]) UnwrapOr(other T) T {
	if r.IsOk() {
		return r.Value
	} else {
		return other
	}
}

// UnwrapOrDefault returns the contained value, or the "zero value" of T if IsErr.
func (r Result[T]) UnwrapOrDefault() T {
	var other T
	return r.UnwrapOr(other)
}

// UnwrapOrElse returns the contained value, or the result of the callback if IsErr.
func (r Result[T]) UnwrapOrElse(f func() T) T {
	if r.IsOk() {
		return r.Value
	} else {
		return f()
	}
}

// Ok wraps a value as a Result[T] containing that value.
func Ok[T any](value T) Result[T] {
	return Result[T]{Value: value}
}

// Err wraps an error as a Result[T] containing that error.
func Err[T any](err error) Result[T] {
	return Result[T]{Error: err}
}

// UnwrapResult is a shortcut for AsResult(...).Unwrap().
func UnwrapResult[T any](value T, err error) T {
	return AsResult(value, err).Unwrap()
}
