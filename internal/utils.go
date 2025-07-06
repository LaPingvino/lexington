// Package internal provides simple generic utilities for the Lexington application
package internal

// Filter returns a new slice containing only elements that satisfy the predicate
func Filter[T any](slice []T, predicate func(T) bool) []T {
	result := make([]T, 0)
	for _, item := range slice {
		if predicate(item) {
			result = append(result, item)
		}
	}
	return result
}

// Map transforms each element of the slice using the provided function
func Map[T, U any](slice []T, transform func(T) U) []U {
	result := make([]U, len(slice))
	for i, item := range slice {
		result[i] = transform(item)
	}
	return result
}

// Find returns the first element that satisfies the predicate and true,
// or the zero value and false if no element is found
func Find[T any](slice []T, predicate func(T) bool) (T, bool) {
	for _, item := range slice {
		if predicate(item) {
			return item, true
		}
	}
	var zero T
	return zero, false
}

// Contains checks if the slice contains the specified value
func Contains[T comparable](slice []T, value T) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

// Unique returns a new slice with duplicate elements removed
func Unique[T comparable](slice []T) []T {
	seen := make(map[T]bool)
	result := make([]T, 0)

	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}

// GroupBy groups elements by the key returned by the keyFunc
func GroupBy[T any, K comparable](slice []T, keyFunc func(T) K) map[K][]T {
	groups := make(map[K][]T)

	for _, item := range slice {
		key := keyFunc(item)
		groups[key] = append(groups[key], item)
	}

	return groups
}
