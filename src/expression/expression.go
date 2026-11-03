package expression

import (
	"circumscribe/pattern"
	"circumscribe/source"
	"fmt"
)

type Expression interface {
	Fold(Environment, Branch) (Expression, pattern.Pattern, error)
	IsPure(Environment, Branch) (bool, error)
	Evaluate(Environment) (string, error)
	Origin() source.Excerpt
}

type Literal struct {
	Literal string
	Excerpt source.Excerpt
}

func (self Literal) Evaluate(environment Environment) (string, error) {
	return self.Literal, nil
}

func (self Literal) Fold(environment Environment, branch Branch) (Expression, pattern.Pattern, error) {
	return self, pattern.Literal(self.Literal), nil
}

func (self Literal) IsPure(environment Environment, branch Branch) (bool, error) {
	return true, nil
}

func (self Literal) Origin() source.Excerpt {
	return self.Excerpt
}

type Application struct {
	Identifier string
	Arguments  []Expression
	Bare       bool
	Excerpt    source.Excerpt
}

func (self Application) Evaluate(environment Environment) (string, error) {
	var buffer []string
	for _, argument := range self.Arguments {
		snippet, diagnostic := argument.Evaluate(environment)
		if diagnostic == nil {
			buffer = append(buffer, snippet)
			continue
		}
		return "", diagnostic
	}
	entry := environment.traverse(self.Identifier)
	if _, ok := entry.(Native); ok {
		snippet, err := entry.Evaluate(*environment.Root, buffer)
		if err == nil {
			return snippet, nil
		}
		return "", source.Diagnostic{
			Message: err.Error(),
			Excerpt: self.Excerpt,
		}
	}
	return entry.Evaluate(*environment.Root, buffer)
}

func (self Application) Fold(environment Environment, branch Branch) (Expression, pattern.Pattern, error) {
	macro := environment.traverse(self.Identifier)
	if macro != nil {
		parameters := macro.Parameters()
		arguments, diagnostic := func() ([]Expression, error) {
			if len(self.Arguments) == len(parameters) {
				buffer := []Expression{}
				for n, argument := range self.Arguments {
					expression, pattern, diagnostic := argument.Fold(environment, branch)
					if diagnostic == nil {
						parameter := parameters[n]
						if pattern.IsAssignable(parameter.Pattern) {
							buffer = append(buffer, expression)
							continue
						}
						return nil, source.Diagnostic{
							Message: fmt.Sprintf("Pattern %s is not assignable to '%s'", pattern.Source, parameter.Pattern.Source),
							Excerpt: expression.Origin(),
						}
					}
					return nil, diagnostic
				}
				return buffer, nil
			}
			return nil, source.Diagnostic{
				Message: fmt.Sprintf("%s expected %d arguments, but %d were provided", self.Identifier, len(parameters), len(self.Arguments)),
				Excerpt: self.Origin(),
			}
		}()
		if diagnostic == nil {
			inferred, diagnostic := macro.Infer(environment, branch)
			if diagnostic == nil {
				pure, diagnostic := macro.IsPure(environment, branch)
				if diagnostic == nil {
					if !self.Bare || pure {
						return Application{
							Identifier: self.Identifier,
							Arguments:  arguments,
							Bare:       self.Bare,
							Excerpt:    self.Excerpt,
						}, inferred, nil
					}
					return nil, pattern.Pattern{}, source.Diagnostic{
						Message: "Bare application of an impure function",
						Excerpt: self.Excerpt,
					}
				}
				return nil, pattern.Pattern{}, diagnostic
			}
			return nil, pattern.Pattern{}, diagnostic
		}
		return nil, pattern.Pattern{}, diagnostic
	}
	return nil, pattern.Pattern{}, source.Diagnostic{
		Message: fmt.Sprintf("Could not resolve identifier '%s'", self.Identifier),
		Excerpt: self.Excerpt,
	}
}

func (self Application) IsPure(environment Environment, branch Branch) (bool, error) {
	for _, argument := range self.Arguments {
		pure, diagnostic := argument.IsPure(environment, branch)
		if diagnostic == nil {
			if pure {
				continue
			}
			return false, nil
		}
		return false, diagnostic
	}
	macro := environment.traverse(self.Identifier)
	return macro.IsPure(environment, branch)
}

func (self Application) Origin() source.Excerpt {
	return self.Excerpt
}

type Condition struct {
	Condition Expression
	Body      Expression
	Otherwise Expression
	Excerpt   source.Excerpt
}

func (self Condition) Evaluate(environment Environment) (string, error) {
	condition, diagnostic := self.Condition.Evaluate(environment)
	if diagnostic == nil {
		if condition == "true" {
			return self.Body.Evaluate(environment)
		}
		return self.Otherwise.Evaluate(environment)
	}
	return "", diagnostic
}

func (self Condition) Fold(environment Environment, branch Branch) (Expression, pattern.Pattern, error) {
	cond, cond_p, diagnostic := self.Condition.Fold(environment, branch)
	if diagnostic == nil {
		body, body_p, diagnostic := self.Body.Fold(environment, branch)
		if diagnostic == nil {
			other, other_p, diagnostic := self.Otherwise.Fold(environment, branch)
			if diagnostic == nil {
				if cond_p.IsAssignable(pattern.StdBool) {
					literal, ok := cond.(Literal)
					if ok {
						if literal.Literal == "true" {
							return body, body_p, nil
						}
						return other, other_p, nil
					}
					return Condition{
						Condition: cond,
						Body:      body,
						Otherwise: other,
						Excerpt:   self.Excerpt,
					}, pattern.Sum(body_p, other_p), nil
				}
				return nil, pattern.Pattern{}, source.Diagnostic{
					Message: fmt.Sprintf("Pattern '%s' is not assignable to '%s'", cond_p.Source, pattern.StdBool.Source),
					Excerpt: self.Condition.Origin(),
				}
			}
			return other, other_p, nil
		}
		return body, body_p, nil
	}
	return cond, cond_p, nil
}

func (self Condition) IsPure(environment Environment, branch Branch) (bool, error) {
	pure, err := self.Condition.IsPure(environment, branch)
	if err == nil && pure {
		pure, err := self.Body.IsPure(environment, branch)
		if err == nil && pure {
			return self.Otherwise.IsPure(environment, branch)
		}
		return pure, err
	}
	return pure, err
}

func (self Condition) Origin() source.Excerpt {
	return self.Excerpt
}
