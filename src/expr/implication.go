package expr

import (
	"circumscribe/env"
	"circumscribe/pat"
	"fmt"
	"maps"
)

type Implication interface {
	Complement() Implication
	Compile() env.Layer[pat.Pattern]
}

func Refine(implication Implication, envm env.Environment[Macro], envp env.Environment[pat.Pattern]) (env.Environment[Macro], error) {
	layer := implication.Compile()
	buffer := env.Layer[Macro]{}
	for name, pattern := range layer {
		macro, ok := envm.Traverse(name)
		if ok {
			inferred, err := macro.Infer(envm, envp, env.Branch[Macro]{})
			if err == nil {
				buffer[name] = Placeholder{
					identifier: name,
					pattern:    pat.Intersection(inferred, pattern),
					delegate:   &macro,
				}
				continue
			}
			return env.Environment[Macro]{}, err
		}
		panic("envm.Traverse(name)")
	}
	return env.Environment[Macro]{
		Root:    envm.Root,
		Parent:  &envm,
		Entries: buffer,
	}, nil
}

type ImpFact struct {
	identifier string
	pattern    pat.Pattern
}

func (self *ImpFact) Compile() env.Layer[pat.Pattern] {
	return env.Layer[pat.Pattern]{
		self.identifier: self.pattern,
	}
}

func (self *ImpFact) Complement() Implication {
	return &ImpFact{
		identifier: self.identifier,
		pattern:    pat.Complement(self.pattern),
	}
}

func (self *ImpFact) String() string {
	return fmt.Sprintf("(%s => %s)", self.identifier, self.pattern)
}

type ImpOr struct {
	left  Implication
	right Implication
}

func (self *ImpOr) Compile() env.Layer[pat.Pattern] {
	left := self.left.Compile()
	right := self.right.Compile()
	buffer := maps.Clone(left)
	for name, pattern := range left {
		if other, ok := right[name]; ok {
			buffer[name] = pat.Sum(pattern, other)
			continue
		}
		delete(buffer, name)
		continue
	}
	return buffer
}

// ¬(P ∨ Q) = ¬P ∧ ¬Q
func (self *ImpOr) Complement() Implication {
	return &ImpAnd{
		left:  self.left.Complement(),
		right: self.right.Complement(),
	}
}

func (self *ImpOr) String() string {
	return fmt.Sprintf("(%s or %s)", self.left, self.right)
}

type ImpAnd struct {
	left  Implication
	right Implication
}

func (self *ImpAnd) Compile() env.Layer[pat.Pattern] {
	left := self.left.Compile()
	right := self.right.Compile()
	buffer := maps.Clone(left)
	for name, pattern := range right {
		if other, ok := left[name]; ok {
			buffer[name] = pat.Intersection(pattern, other)
			continue
		}
		buffer[name] = pattern
		continue
	}
	return buffer
}

func (self *ImpAnd) String() string {
	return fmt.Sprintf("(%s and %s)", self.left, self.right)
}

// ¬(P ∧ Q) = ¬P ∨ ¬Q
func (self *ImpAnd) Complement() Implication {
	return &ImpOr{
		left:  self.left.Complement(),
		right: self.right.Complement(),
	}
}

type ImpNot struct {
	imp Implication
}

func (self *ImpNot) Compile() env.Layer[pat.Pattern] {
	complement := self.imp.Complement()
	return complement.Compile()
}

func (self *ImpNot) Complement() Implication {
	return self.imp
}

func (self *ImpNot) String() string {
	return fmt.Sprintf("(!%s)", self.imp)
}

// ⊤:
// a ∨ ⊤ ⇒ ⊤ (a ∨ ⊤ ≠ ⊤)
// a ∧ ⊤ ⇒ a (a ∧ ⊤ ≠ a)
// ¬⊤ = ⊤
type ImpTop struct {
}

func (self *ImpTop) Compile() env.Layer[pat.Pattern] {
	return env.Layer[pat.Pattern]{}
}

func (self *ImpTop) Complement() Implication {
	return self
}

func (self *ImpTop) String() string {
	return "(⊤)"
}
