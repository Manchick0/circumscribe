package expression

import (
	"circumscribe/pattern"
	"circumscribe/source"
	"fmt"
)

type Macro interface {
	// Evaluate the macro within the given environment,
	// with the procided arguments.
	// ---
	// The environment represents the identifiers accessible
	// from the place the macro was declared in, hence the *root*
	// of the environment:
	//
	// func.Evaluate(*environment.root, ...)
	//
	// Whether the arity and the shape of the arguments correspond
	// to the function's definition is validated when the application
	// of the function is folded. One does not need to further validate
	// this at runtme.
	Evaluate(Environment, []string) (string, error)
	// Infer the pattern denoting the return type
	// of the macro.
	// ---
	// If the pattern is known, it is returned directly.
	// Otherwise, the body of the function is used in order
	// to infer the return pattern.
	Infer(Environment, Branch) (pattern.Pattern, error)
	Parameters() []Parameter
	IsPure(Environment, Branch) (bool, error)
	Name() string
}

type Parameter struct {
	Name    string
	Pattern pattern.Pattern
}

type Source struct {
	Identifier string
	Body       struct {
		Source Expression // The original expression, as written in the source
		Folded Expression // The folded version of the original expression
	}
	Params  []Parameter
	Pattern *pattern.Pattern
	Pure    *bool
	Excerpt source.Excerpt
}

func (self *Source) Evaluate(environment Environment, arguments []string) (string, error) {
	buffer := map[string]Macro{}
	for n, parameter := range self.Params {
		buffer[parameter.Name] = Constant{
			identifier: parameter.Name,
			constant:   arguments[n],
		}
	}
	return (self.Body.Folded).Evaluate(Environment{
		Root:    environment.Root,
		Parent:  &environment,
		Entries: buffer,
	})
}

func (self *Source) Parameters() []Parameter {
	return self.Params
}

func (self *Source) Infer(environment Environment, branch Branch) (pattern.Pattern, error) {
	if self.Pattern == nil {
		if !branch.contains(self) {
			diagnostic := self.Fold(environment, branch)
			if diagnostic == nil {
				return *self.Pattern, nil
			}
			return pattern.Pattern{}, diagnostic
		}
		return pattern.Pattern{}, source.Diagnostic{
			Message: "A recursive function must have its pattern annotated",
			Excerpt: self.Excerpt,
		}
	}
	return *self.Pattern, nil
}

func (self *Source) Fold(environment Environment, branch Branch) error {
	if self.Body.Folded == nil {
		if !branch.contains(self) {
			folded, pattern, diagnostic := self.Body.Source.Fold(Environment{
				Root:    environment.Root,
				Parent:  environment.Root,
				Entries: map[string]Macro{},
			}, Branch{
				Parent: &branch,
				Entry:  self,
			})
			if diagnostic == nil {
				if self.Pattern == nil {
					self.Body.Folded = folded
					self.Pattern = &pattern
					return nil
				}
				if pattern.IsAssignable(*self.Pattern) {
					self.Body.Folded = folded
					return nil
				}
				return source.Diagnostic{
					Message: fmt.Sprintf("Pattern '%s' is not assignable to '%s'", pattern.Source, self.Pattern.Source),
					Excerpt: self.Excerpt,
				}
			}
			return diagnostic
		}
		panic("Recursion within Fold()")
	}
	return nil
}

func (self *Source) IsPure(environment Environment, branch Branch) (bool, error) {
	if self.Pure == nil {
		if branch.contains(self) {
			// "pure until proven otherwise"
			return true, nil
		}
		diagnostic := self.Fold(Environment{
			Root:    environment.Root,
			Parent:  environment.Root,
			Entries: map[string]Macro{},
		}, Branch{
			Parent: &branch,
			Entry:  self,
		})
		if diagnostic == nil {
			pure, diagnostic := self.Body.Folded.IsPure(Environment{
				Root:    environment.Root,
				Parent:  environment.Root,
				Entries: map[string]Macro{},
			}, Branch{
				Parent: &branch,
				Entry:  self,
			})
			if diagnostic == nil {
				self.Pure = &pure
				return pure, nil
			}
			return false, diagnostic
		}
		return false, diagnostic
	}
	return *self.Pure, nil
}

func (self Source) Name() string {
	return self.Identifier
}

type Native struct {
	Identifier string
	Params     []Parameter
	Pattern    pattern.Pattern
	Body       func(environment Environment, arguments []string) (string, error)
	Pure       bool
}

func (self Native) Evaluate(environment Environment, arguments []string) (string, error) {
	return self.Body(environment, arguments)
}

func (self Native) Infer(environment Environment, branch Branch) (pattern.Pattern, error) {
	return self.Pattern, nil
}

func (self Native) Name() string {
	return self.Identifier
}

func (self Native) IsPure(environment Environment, branch Branch) (bool, error) {
	return self.Pure, nil
}

func (self Native) Parameters() []Parameter {
	return self.Params
}

type Constant struct {
	identifier string
	constant   string
}

func (self Constant) Evaluate(environment Environment, arguments []string) (string, error) {
	return self.constant, nil
}

func (self Constant) Infer(environment Environment, branch Branch) (pattern.Pattern, error) {
	return pattern.Literal(self.constant), nil
}

func (self Constant) Name() string {
	return self.identifier
}

func (self Constant) IsPure(environment Environment, branch Branch) (bool, error) {
	return true, nil
}

func (self Constant) Parameters() []Parameter {
	return []Parameter{}
}

type Placeholder struct {
	identifier string
	pattern    pattern.Pattern
}

func (self Placeholder) Evaluate(environment Environment, arguments []string) (string, error) {
	panic("Attempted to Evaluate() a Placeholder")
}

func (self Placeholder) Name() string {
	return self.identifier
}

func (self Placeholder) Infer() pattern.Pattern {
	return self.pattern
}
