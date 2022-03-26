package generic

type Option[T any] struct {
	Value    T
	hasValue bool
}

// Expect returns the contained value, or panics with the supplied error message if there is no value.
func (o Option[T]) Expect(msg string) T {
	if o.hasValue {
		return o.Value
	} else {
		panic(msg)
	}
}

// IsNone returns true if this Option[T] does not have a value.
func (o *Option[T]) IsNone() bool {
	return !o.hasValue
}

// IsSome returns true if this Option[T] has a value.
func (o *Option[T]) IsSome() bool {
	return o.hasValue
}

// OkOr transforms the Option[T] into a Result[T] with the contained value, or the specified error if there is no value.
func (o Option[T]) OkOr(err error) Result[T] {
	if o.hasValue {
		return Ok(o.Value)
	} else {
		return Err[T](err)
	}
}

// OkOrElse transforms the Option[T] into a Result[T] with the contained value, or the (error) result of the callback
// if there is no value.
func (o Option[T]) OkOrElse(f func() error) Result[T] {
	if o.hasValue {
		return Ok(o.Value)
	} else {
		return Err[T](f())
	}
}

// Or returns the option itself if it has a value, otherwise it returns other.
func (o Option[T]) Or(other Option[T]) Option[T] {
	if o.hasValue {
		return o
	} else {
		return other
	}
}

// OrElse returns the option itself if it has a value, otherwise it returns the result of the callback.
func (o Option[T]) OrElse(f func() Option[T]) Option[T] {
	if o.hasValue {
		return o
	} else {
		return f()
	}
}

// Unwrap returns the contained value, or panics if there is no value.
func (o Option[T]) Unwrap() T {
	return o.Expect("tried to Unwrap() a None")
}

// UnwrapOr returns the contained value, or other if there is no value.
func (o Option[T]) UnwrapOr(other T) T {
	if o.hasValue {
		return o.Value
	} else {
		return other
	}
}

// UnwrapOrDefault returns the contained value, or the "zero value" for T if there is no value.
func (o Option[T]) UnwrapOrDefault() T {
	var other T
	return o.UnwrapOr(other)
}

// UnwrapOrElse returns the contained value, or the result of the callback if there is no value.
func (o Option[T]) UnwrapOrElse(f func() T) T {
	if o.hasValue {
		return o.Value
	} else {
		return f()
	}
}

// Some constructs an Option[T] that has a value.
func Some[T any](value T) Option[T] {
	return Option[T]{Value: value, hasValue: true}
}

// None constructs an Option[T] that does not have a value.
func None[T any]() Option[T] {
	return Option[T]{hasValue: false}
}
