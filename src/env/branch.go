package env

type Branch[T comparable] struct {
	Parent *Branch[T]
	Entry  T
}

func (self *Branch[T]) Contains(candidate T) bool {
	if self.Entry == candidate {
		return true
	}
	return self.Parent != nil && self.Parent.Contains(candidate)
}
