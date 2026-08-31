// Package util adds useful methods such as easier filtering
package util

type List[T any] []T

func (list List[T]) Filter(predicate func(T) bool) []T {
	result := make([]T, 0, len(list))

	for _, item := range list {
		if predicate(item) {
			result = append(result, item)
		}
	}

	return result
}
