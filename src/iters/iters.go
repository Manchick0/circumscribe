package iters

import "iter"

func Fmap[A any, B any](seq iter.Seq[A], f func(a A) B) iter.Seq[B] {
	return func(yield func(B) bool) {
		seq(func(a A) bool {
			return yield(f(a))
		})
	}
}

func Bind[A any, B any](seq iter.Seq[A], f func(a A) iter.Seq[B]) iter.Seq[B] {
	return func(yield func(B) bool) {
		seq(func(a A) bool {
			for b := range f(a) {
				if yield(b) {
					continue
				}
				return false
			}
			return true
		})
	}
}

func Filter[A any](seq iter.Seq[A], f func(a A) bool) iter.Seq[A] {
	return func(yield func(A) bool) {
		seq(func(a A) bool {
			if f(a) {
				return yield(a)
			}
			return true
		})
	}
}
