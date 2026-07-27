package utils

// Coalesce returns the first non-zero value among value and fallbacks.
func Coalesce[T comparable](value T, fallbacks ...T) T {
	var zero T
	if value != zero {
		return value
	}

	for _, fb := range fallbacks {
		if fb != zero {
			return fb
		}
	}
	return zero
}
