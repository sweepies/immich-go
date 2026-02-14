package gen

import "slices"

func DeleteItem[T comparable](s []T, item T) []T {
	r := slices.Clone(s)
	if r == nil {
		r = make([]T, 0)
	}
	return slices.DeleteFunc(r, func(v T) bool { return v == item })
}

func Filter[T any](s []T, f func(i T) bool) []T {
	var r []T
	for _, i := range s {
		if f(i) {
			r = append(r, i)
		}
	}
	return r
}

func AddOnce[T comparable](s []T, vv ...T) []T {
	for _, v := range vv {
		if !slices.Contains(s, v) {
			s = append(s, v)
		}
	}
	return s
}
