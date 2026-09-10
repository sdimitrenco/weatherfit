package domain

// Opt is a value the API may omit. A missing value is rendered as a dash
// instead of being replaced by zero.
type Opt[T any] struct {
	value T
	ok    bool
}

// Some wraps a present value.
func Some[T any](value T) Opt[T] {
	return Opt[T]{value: value, ok: true}
}

// None returns an empty value.
func None[T any]() Opt[T] {
	return Opt[T]{}
}

// Valid reports whether a value is present.
func (o Opt[T]) Valid() bool {
	return o.ok
}

// Get returns the value and whether it is present.
func (o Opt[T]) Get() (T, bool) {
	return o.value, o.ok
}

// Or returns the value or the fallback when absent.
func (o Opt[T]) Or(fallback T) T {
	if !o.ok {
		return fallback
	}
	return o.value
}

// Map applies f to a present value, keeping emptiness.
func Map[T, R any](o Opt[T], f func(T) R) Opt[R] {
	value, ok := o.Get()
	if !ok {
		return None[R]()
	}
	return Some(f(value))
}
