package utils

func IndexOf(word string, data []string) int {
	for k, v := range data {
		if word == v {
			return k
		}
	}
	return -1
}

func RemoveFromSliceIndex[T any](s []T, index int) []T {
	return append(s[:index], s[index+1:]...)
}

func RemoveFromSlice[T comparable](s []T, t T) []T {
	var newT []T

	for k, v := range s {
		if v == t {
			newT = RemoveFromSliceIndex(s, k)
		}
	}

	return newT
}
