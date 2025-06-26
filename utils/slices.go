package utils

import "iter"

func Map[S any, T any](s []S, fn func(S) T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, v := range s {
			if !yield(fn(v)) {
				return
			}
		}
	}
}
