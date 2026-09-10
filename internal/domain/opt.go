package domain

// Opt — значение, которого может не быть в ответе API. Отсутствующее значение
// показывается в отчёте как «—», а не подменяется нулём.
type Opt[T any] struct {
	value T
	ok    bool
}

// Some возвращает заполненное значение.
func Some[T any](value T) Opt[T] {
	return Opt[T]{value: value, ok: true}
}

// None возвращает пустое значение.
func None[T any]() Opt[T] {
	return Opt[T]{}
}

// Valid сообщает, есть ли значение.
func (o Opt[T]) Valid() bool {
	return o.ok
}

// Get возвращает значение и признак его наличия.
func (o Opt[T]) Get() (T, bool) {
	return o.value, o.ok
}

// Or возвращает значение либо fallback, если значения нет.
func (o Opt[T]) Or(fallback T) T {
	if !o.ok {
		return fallback
	}
	return o.value
}

// Map применяет f к значению, сохраняя пустоту.
func Map[T, R any](o Opt[T], f func(T) R) Opt[R] {
	value, ok := o.Get()
	if !ok {
		return None[R]()
	}
	return Some(f(value))
}
