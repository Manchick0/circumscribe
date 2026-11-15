package env

import "maps"

type Layer[T any] map[string]T

type Environment[T any] struct {
	Root    *Environment[T]
	Parent  *Environment[T]
	Entries Layer[T]
}

func (self *Environment[T]) Flatten() Layer[T] {
	if self.Parent != nil {
		buffer := self.Parent.Flatten()
		maps.Copy(buffer, self.Entries)
		return buffer
	}
	return maps.Clone(self.Entries)
}

func (self *Environment[T]) Traverse(identifier string) (T, bool) {
	if entry, ok := self.Entries[identifier]; ok {
		return entry, true
	}
	if self.Parent != nil {
		return self.Parent.Traverse(identifier)
	}
	return *new(T), false
}
