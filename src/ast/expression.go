package ast

import (
	"circumscribe/env"
	"circumscribe/pat"
	"circumscribe/source"
	"fmt"
	"math/big"
	"reflect"
	"strings"
)

// The 'recursion' limit
// ---
// Despite its name, the recursion limit
// does not inherently limit recursion,
// but merely the number of functions in
// a single evaluation of an expression.
const rlimit = 512

// An expression.
// ---
// An expression is a single piece of code that
// may be evaluated to a snippet.
type Expression interface {
	Origin() *source.Excerpt
	expr()
}

// Compile the given expression, expecting
// it to conform to the given pattern.
// ---
func Expect(self Expression, envm env.Environment[Macro], envp env.Environment[pat.Pattern], branch env.Branch[Macro], pattern pat.Pattern) (pat.Pattern, error) {
	from, err := Compile(self, envm, envp, branch)
	if err == nil {
		left, err := from.Compile(envp, env.Branch[pat.Pattern]{})
		if err == nil {
			right, err := pattern.Compile(envp, env.Branch[pat.Pattern]{})
			if err == nil {
				complement := right.Complement()
				intersection := left.Intersection(complement)
				if intersection.IsMeaningful() {
					return nil, source.Diagnostic{
						Message: fmt.Sprintf("Pattern '%s' is not assignable to '%s'", left, right),
						Excerpt: self.Origin(),
					}
				}
				return from, nil
			}
			return nil, err
		}
		return nil, err
	}
	return nil, err
}

// Evaluate the given expression in the provided
// environment.
// ---
// Attempts to produce a snippet by evalauting
// the given expression. If an error occurs,
// it is returned instead. An expression must
// be compiled before it may be evaluated.
func Evaluate(self Expression, envm env.Environment[Macro], envp env.Environment[pat.Pattern], recursion int) (string, error) {
	if self, ok := self.(Literal); ok {
		return self.Literal, nil
	}
	if self, ok := self.(Snippet); ok {
		var buffer strings.Builder
		for _, child := range self.Children {
			snippet, diagnostic := Evaluate(child, envm, envp, recursion)
			if diagnostic == nil {
				buffer.WriteString(snippet)
				continue
			}
			return "", diagnostic
		}
		return buffer.String(), nil
	}
	if self, ok := self.(Application); ok {
		var buffer []string
		for _, argument := range self.Arguments {
			snippet, diagnostic := Evaluate(argument, envm, envp, recursion)
			if diagnostic == nil {
				buffer = append(buffer, snippet)
				continue
			}
			return "", diagnostic
		}
		entry, _ := envm.Traverse(self.Identifier)
		snippet, err := entry.Evaluate(envm, envp, buffer, recursion)
		if err != nil {
			if _, ok := err.(source.Diagnostic); ok {
				return snippet, err
			}
			return snippet, source.Diagnose(err.Error(), self.Excerpt)
		}
		return snippet, nil
	}
	if self, ok := self.(Switch); ok {
		source, diagnostic := Evaluate(self.Source, envm, envp, recursion)
		if diagnostic == nil {
			for _, branch := range self.Branches {
				layer := branch.Grammar.Match(source)
				if layer != nil {
					buffer := map[string]Macro{}
					for name, snippet := range layer {
						buffer[name] = Constant{
							identifier: name,
							constant:   snippet,
						}
						continue
					}
					return Evaluate(branch.Body, env.Environment[Macro]{
						Root:    envm.Root,
						Parent:  &envm,
						Entries: buffer,
					}, env.Environment[pat.Pattern]{
						Root:    envp.Root,
						Parent:  &envp,
						Entries: env.Layer[pat.Pattern]{},
					}, recursion)
				}
				continue
			}
			return Evaluate(self.Otherwise, envm, envp, recursion)
		}
		return source, diagnostic
	}
	if self, ok := self.(If); ok {
		condition, diagnostic := Evaluate(self.Condition, envm, envp, recursion)
		if diagnostic == nil {
			if condition == "true" {
				return Evaluate(self.Body, envm, envp, recursion)
			}
			return Evaluate(self.Otherwise, envm, envp, recursion)
		}
		return condition, diagnostic
	}
	if self, ok := self.(With); ok {
		snippet, err := Evaluate(self.Expression, envm, envp, recursion)
		if err == nil {
			return Evaluate(self.Body, env.Environment[Macro]{
				Parent: &envm,
				Root:   envm.Root,
				Entries: env.Layer[Macro]{
					self.Identifier: Constant{
						identifier: self.Identifier,
						constant:   snippet,
					},
				},
			}, env.Environment[pat.Pattern]{
				Parent:  &envp,
				Root:    envp.Root,
				Entries: env.Layer[pat.Pattern]{},
			}, recursion)
		}
		return snippet, err
	}
	if self, ok := self.(Raise); ok {
		snippet, err := Evaluate(self.Expression, envm, envp, recursion)
		if err == nil {
			return "", source.Diagnose(snippet, self.Excerpt)
		}
		return snippet, err
	}
	if self, ok := self.(Compare); ok {
		left, dignostic := Evaluate(self.Left, envm, envp, recursion)
		if dignostic == nil {
			right, diagnostic := Evaluate(self.Right, envm, envp, recursion)
			if diagnostic == nil {
				if left == right {
					return "true", nil
				}
				return "false", nil
			}
			return right, diagnostic
		}
		return left, dignostic
	}
	if self, ok := self.(And); ok {
		left, diagnostic := Evaluate(self.Left, envm, envp, recursion)
		if diagnostic == nil {
			if left == "true" {
				return Evaluate(self.Right, envm, envp, recursion)
			}
			return "false", nil
		}
		return left, diagnostic
	}
	if self, ok := self.(Or); ok {
		left, diagnostic := Evaluate(self.Left, envm, envp, recursion)
		if diagnostic == nil {
			if left == "false" {
				return Evaluate(self.Right, envm, envp, recursion)
			}
			return "true", nil
		}
		return left, diagnostic
	}
	if self, ok := self.(Not); ok {
		snippet, err := Evaluate(self.Expression, envm, envp, recursion)
		if err == nil {
			if snippet == "true" {
				return "false", nil
			}
			return "true", nil
		}
		return snippet, err
	}
	if self, ok := self.(Satisfies); ok {
		snippet, err := Evaluate(self.Expression, envm, envp, recursion)
		if err == nil {
			machine, _ := self.Pattern.Compile(envp, env.Branch[pat.Pattern]{})
			if machine.Satisfies(snippet) {
				return "true", nil
			}
			return "false", nil
		}
		return snippet, err
	}
	if self, ok := self.(Add); ok {
		left, diagnostic := Evaluate(self.Left, envm, envp, recursion)
		if diagnostic == nil {
			right, diagnostic := Evaluate(self.Right, envm, envp, recursion)
			if diagnostic == nil {
				left, _ := new(big.Int).SetString(left, 10)
				right, _ := new(big.Int).SetString(right, 10)
				sum := new(big.Int).Add(left, right)
				return sum.Text(10), nil
			}
			return right, diagnostic
		}
		return left, diagnostic
	}
	if self, ok := self.(Sub); ok {
		left, diagnostic := Evaluate(self.Left, envm, envp, recursion)
		if diagnostic == nil {
			right, diagnostic := Evaluate(self.Right, envm, envp, recursion)
			if diagnostic == nil {
				left, _ := new(big.Int).SetString(left, 10)
				right, _ := new(big.Int).SetString(right, 10)
				if left.Cmp(right) > 0 {
					dif := new(big.Int).Sub(left, right)
					return dif.Text(10), nil
				}
				return "0", nil
			}
			return right, diagnostic
		}
		return left, diagnostic
	}
	if self, ok := self.(Mul); ok {
		left, diagnostic := Evaluate(self.Left, envm, envp, recursion)
		if diagnostic == nil {
			right, diagnostic := Evaluate(self.Right, envm, envp, recursion)
			if diagnostic == nil {
				left, _ := new(big.Int).SetString(left, 10)
				right, _ := new(big.Int).SetString(right, 10)
				product := new(big.Int).Mul(left, right)
				return product.Text(10), nil
			}
			return right, diagnostic
		}
		return left, diagnostic
	}
	if self, ok := self.(Div); ok {
		left, diagnostic := Evaluate(self.Left, envm, envp, recursion)
		if diagnostic == nil {
			right, diagnostic := Evaluate(self.Right, envm, envp, recursion)
			if diagnostic == nil {
				left, _ := new(big.Int).SetString(left, 10)
				right, _ := new(big.Int).SetString(right, 10)
				if left.Cmp(big.NewInt(0)) != 0 {
					quotient := new(big.Int).Div(left, right)
					return quotient.Text(10), nil
				}
				return "", source.Diagnose("Division by zero", self.Excerpt)
			}
			return right, diagnostic
		}
		return left, diagnostic
	}
	if self, ok := self.(Less); ok {
		left, err := Evaluate(self.Left, envm, envp, recursion)
		if err == nil {
			right, err := Evaluate(self.Right, envm, envp, recursion)
			if err == nil {
				left, _ := new(big.Int).SetString(left, 10)
				right, _ := new(big.Int).SetString(right, 10)
				if left.Cmp(right) < 0 {
					return "true", nil
				}
				return "false", nil
			}
			return right, err
		}
		return left, err
	}
	if self, ok := self.(Greater); ok {
		left, err := Evaluate(self.Left, envm, envp, recursion)
		if err == nil {
			right, err := Evaluate(self.Right, envm, envp, recursion)
			if err == nil {
				left, _ := new(big.Int).SetString(left, 10)
				right, _ := new(big.Int).SetString(right, 10)
				if left.Cmp(right) > 0 {
					return "true", nil
				}
				return "false", nil
			}
			return right, err
		}
		return left, err
	}
	if self, ok := self.(Add); ok {
		left, diagnostic := Evaluate(self.Left, envm, envp, recursion)
		if diagnostic == nil {
			right, diagnostic := Evaluate(self.Right, envm, envp, recursion)
			if diagnostic == nil {
				left, _ := new(big.Int).SetString(left, 10)
				right, _ := new(big.Int).SetString(right, 10)
				sum := new(big.Int).Add(left, right)
				return sum.Text(10), nil
			}
			return right, diagnostic
		}
		return left, diagnostic
	}
	panic(reflect.TypeOf(self))
}

// Compile the given expression in
// the provided environment.
// ---
// Traverses the AST of the given expression, performs
// all necessary type-checking, recursively simplifies
// the expression and infers its type.
//
// If an error occurs, it is returned. Otherwise, the
// simplified expression and its respective pattern is
// returned instead.
func Compile(self Expression, envm env.Environment[Macro], envp env.Environment[pat.Pattern], branch env.Branch[Macro]) (pat.Pattern, error) {
	if self, ok := self.(Literal); ok {
		return pat.Literal(self.Literal), nil
	}
	if self, ok := self.(Snippet); ok {
		buffer := pat.StdE // Multiplicative Identity
		for _, child := range self.Children {
			pattern, err := Compile(child, envm, envp, branch)
			if err == nil {
				buffer = pat.Product(buffer, pattern)
				continue
			}
			return nil, err
		}
		return buffer, nil
	}
	if self, ok := self.(Application); ok {
		if macro, ok := envm.Traverse(self.Identifier); ok {
			parameters := macro.Parameters()
			if len(self.Arguments) == len(parameters) {
				for n, argument := range self.Arguments {
					parameter := parameters[n]
					_, err := Expect(argument, envm, envp, branch, parameter.Pattern)
					if err == nil {
						continue
					}
					return nil, err
				}
				inferred, err := macro.Infer(envm, envp, branch)
				if err == nil {
					pure, err := macro.IsPure(envm, envp, branch)
					if err == nil {
						if pure || !self.Bare {
							return inferred, nil
						}
						return nil, source.Diagnose("Bare application of an impure function", self.Excerpt)
					}
					return nil, err
				}
				return nil, err
			}
			return nil, source.Diagnostic{
				Message: fmt.Sprintf("%s expected %d arguments, but %d were provided", self.Identifier, len(parameters), len(self.Arguments)),
				Excerpt: self.Origin(),
			}
		}
		return nil, source.Diagnose(fmt.Sprintf("Could not resolve identifier '%s'", self.Identifier), self.Excerpt)
	}
	if self, ok := self.(Switch); ok {
		_, err := Compile(self.Source, envm, envp, branch)
		if err == nil {
			enve, err := Under(self.Source, pat.Complement(func() pat.Pattern {
				buffer := pat.StdNever // Additive identity
				for _, option := range self.Branches {
					buffer = pat.Sum(buffer, option.Grammar.Pattern())
				}
				return buffer
			}()), envm, envp, branch)
			other, err := Compile(self.Otherwise, enve, env.Environment[pat.Pattern]{
				Root:    envp.Root,
				Parent:  &envp,
				Entries: env.Layer[pat.Pattern]{},
			}, branch)
			if err == nil {
				buffer := other
				for _, option := range self.Branches {
					err := option.Grammar.Compile(envp)
					if err == nil {
						envf, err := Under(self.Source, option.Grammar.Pattern(), envm, envp, branch)
						if err == nil {
							body, err := Compile(option.Body, env.Environment[Macro]{
								Root:   envf.Root,
								Parent: &envf,
								Entries: func() env.Layer[Macro] {
									buffer := env.Layer[Macro]{}
									for _, binding := range option.Grammar.Bindings() {
										buffer[binding.Name] = Placeholder{
											identifier: binding.Name,
											pattern:    binding.Pattern,
										}
									}
									return buffer
								}(),
							}, env.Environment[pat.Pattern]{
								Root:    envp.Root,
								Parent:  &envp,
								Entries: env.Layer[pat.Pattern]{},
							}, branch)
							if err == nil {
								buffer = pat.Sum(buffer, body)
								continue
							}
							return nil, err
						}
						return nil, err
					}
					return nil, err
				}
				return buffer, nil
			}
			return nil, err
		}
		return nil, err
	}
	if self, ok := self.(With); ok {
		pattern, err := Compile(self.Expression, envm, envp, branch)
		if err == nil {
			return Compile(self.Body, env.Environment[Macro]{
				Root:   envm.Root,
				Parent: &envm,
				Entries: env.Layer[Macro]{
					self.Identifier: Placeholder{
						identifier: self.Identifier,
						pattern:    pattern,
					},
				},
			}, env.Environment[pat.Pattern]{
				Root:    envp.Root,
				Parent:  &envp,
				Entries: env.Layer[pat.Pattern]{},
			}, branch)
		}
		return nil, err
	}
	if self, ok := self.(If); ok {
		_, err := Expect(self.Condition, envm, envp, branch, pat.StdBoolean)
		if err == nil {
			analysis, err := Analyze(self.Condition, envm, envp, branch)
			if err == nil {
				envf, err := Refine(analysis, envm, envp)
				if err == nil {
					body, err := Compile(self.Body, envf, env.Environment[pat.Pattern]{
						Root:    envp.Root,
						Parent:  &envp,
						Entries: env.Layer[pat.Pattern]{},
					}, branch)
					if err == nil {
						enve, err := Refine(analysis.Complement(), envm, envp)
						if err == nil {
							other, err := Compile(self.Otherwise, enve, env.Environment[pat.Pattern]{
								Root:    envp.Root,
								Parent:  &envp,
								Entries: env.Layer[pat.Pattern]{},
							}, branch)
							if err == nil {
								return pat.Sum(body, other), nil
							}
							return nil, err
						}
						return nil, err
					}
					return nil, err
				}
				return nil, err
			}
			return nil, err
		}
		return nil, err
	}
	if self, ok := self.(Raise); ok {
		_, err := Compile(self.Expression, envm, envp, branch)
		if err == nil {
			return pat.StdNever, nil
		}
		return nil, err
	}
	if self, ok := self.(Compare); ok {
		_, err := Compile(self.Left, envm, envp, branch)
		if err == nil {
			_, err := Compile(self.Right, envm, envp, branch)
			if err == nil {
				return pat.StdBoolean, nil
			}
			return nil, err
		}
		return nil, err
	}
	if self, ok := self.(Satisfies); ok {
		_, err := Compile(self.Expression, envm, envp, branch)
		if err == nil {
			return pat.StdBoolean, nil
		}
		return nil, err
	}
	if self, ok := self.(And); ok {
		_, err := Expect(self.Left, envm, envp, branch, pat.StdBoolean)
		if err == nil {
			analysis, err := Analyze(self.Left, envm, envp, branch)
			if err == nil {
				environment, err := Refine(analysis, envm, envp)
				if err == nil {
					_, err := Expect(self.Right, environment, env.Environment[pat.Pattern]{
						Root:    envp.Root,
						Parent:  &envp,
						Entries: env.Layer[pat.Pattern]{},
					}, branch, pat.StdBoolean)
					if err == nil {
						return pat.StdBoolean, nil
					}
					return nil, err
				}
				return nil, err
			}
			return nil, err
		}
		return nil, err
	}
	if self, ok := self.(Or); ok {
		_, err := Expect(self.Left, envm, envp, branch, pat.StdBoolean)
		if err == nil {
			analysis, err := Analyze(self.Left, envm, envp, branch)
			if err == nil {
				environment, err := Refine(analysis.Complement(), envm, envp)
				if err == nil {
					_, err := Expect(self.Right, environment, env.Environment[pat.Pattern]{
						Root:    envp.Root,
						Parent:  &envp,
						Entries: env.Layer[pat.Pattern]{},
					}, branch, pat.StdBoolean)
					if err == nil {
						return pat.StdBoolean, nil
					}
					return nil, err
				}
				return nil, err
			}
			return nil, err
		}
		return nil, err
	}
	if self, ok := self.(Not); ok {
		_, err := Expect(self.Expression, envm, envp, branch, pat.StdBoolean)
		if err == nil {
			return pat.StdBoolean, nil
		}
		return nil, err
	}
	if self, ok := self.(Less); ok {
		_, err := Expect(self.Left, envm, envp, branch, pat.StdNatural)
		if err == nil {
			_, err := Expect(self.Right, envm, envp, branch, pat.StdNatural)
			if err == nil {
				return pat.StdBoolean, nil
			}
			return nil, err
		}
		return nil, err
	}
	if self, ok := self.(Greater); ok {
		_, err := Expect(self.Left, envm, envp, branch, pat.StdNatural)
		if err == nil {
			_, err := Expect(self.Right, envm, envp, branch, pat.StdNatural)
			if err == nil {
				return pat.StdBoolean, nil
			}
			return nil, err
		}
		return nil, err
	}
	if self, ok := self.(Add); ok {
		_, err := Expect(self.Left, envm, envp, branch, pat.StdNatural)
		if err == nil {
			_, err := Expect(self.Right, envm, envp, branch, pat.StdNatural)
			if err == nil {
				return pat.StdNatural, nil
			}
			return nil, err
		}
		return nil, err
	}
	if self, ok := self.(Sub); ok {
		_, err := Expect(self.Left, envm, envp, branch, pat.StdNatural)
		if err == nil {
			_, err := Expect(self.Right, envm, envp, branch, pat.StdNatural)
			if err == nil {
				return pat.StdNatural, nil
			}
			return nil, err
		}
		return nil, err
	}
	if self, ok := self.(Mul); ok {
		_, err := Expect(self.Left, envm, envp, branch, pat.StdNatural)
		if err == nil {
			_, err := Expect(self.Right, envm, envp, branch, pat.StdNatural)
			if err == nil {
				return pat.StdNatural, nil
			}
			return nil, err
		}
		return nil, err
	}
	if self, ok := self.(Div); ok {
		_, err := Expect(self.Left, envm, envp, branch, pat.StdNatural)
		if err == nil {
			_, err := Expect(self.Right, envm, envp, branch, pat.StdNatural)
			if err == nil {
				return pat.StdNatural, nil
			}
			return nil, err
		}
		return nil, err
	}
	panic(reflect.TypeOf(self))
}

// Determine if the given expression is pure
// in the provided environment.
// ---
// An expression is pretty much always pure,
// unless it is an application of an impure
// macro. See [Macro.IsPure].
//
// If an error occurs, it is returned instead.
func IsPure(self Expression, envm env.Environment[Macro], envp env.Environment[pat.Pattern], branch env.Branch[Macro]) (bool, error) {
	if _, ok := self.(Literal); ok {
		return true, nil
	}
	if self, ok := self.(Snippet); ok {
		for _, segment := range self.Children {
			pure, err := IsPure(segment, envm, envp, branch)
			if err == nil {
				if pure {
					continue
				}
				return false, nil
			}
			return false, err
		}
		return true, nil
	}
	if self, ok := self.(Application); ok {
		for _, argument := range self.Arguments {
			pure, diagnostic := IsPure(argument, envm, envp, branch)
			if diagnostic == nil {
				if pure {
					continue
				}
				return false, nil
			}
			return false, diagnostic
		}
		macro, _ := envm.Traverse(self.Identifier)
		return macro.IsPure(envm, envp, branch)
	}
	if self, ok := self.(Switch); ok {
		pure, err := IsPure(self.Source, envm, envp, branch)
		if err == nil && pure {
			pure, err := IsPure(self.Otherwise, envm, envp, branch)
			if err == nil && pure {
				for _, option := range self.Branches {
					pure, err := IsPure(option.Body, env.Environment[Macro]{
						Root:   envm.Root,
						Parent: &envm,
						Entries: func() env.Layer[Macro] {
							buffer := env.Layer[Macro]{}
							for _, binding := range option.Grammar.Bindings() {
								buffer[binding.Name] = Placeholder{
									identifier: binding.Name,
									pattern:    binding.Pattern,
								}
							}
							return buffer
						}(),
					}, env.Environment[pat.Pattern]{
						Root:    envp.Root,
						Parent:  &envp,
						Entries: env.Layer[pat.Pattern]{},
					}, branch)
					if err == nil && pure {
						continue
					}
					return pure, err
				}
				return true, nil
			}
			return pure, err
		}
		return pure, err
	}
	if self, ok := self.(If); ok {
		pure, err := IsPure(self.Condition, envm, envp, branch)
		if err == nil && pure {
			pure, err := IsPure(self.Body, envm, envp, branch)
			if err == nil && pure {
				return IsPure(self.Otherwise, envm, envp, branch)
			}
			return pure, err
		}
		return pure, err
	}
	if self, ok := self.(With); ok {
		pure, err := IsPure(self.Expression, envm, envp, branch)
		if err == nil && pure {
			return IsPure(self.Body, envm, envp, branch)
		}
		return pure, err
	}
	if self, ok := self.(Compare); ok {
		pure, err := IsPure(self.Left, envm, envp, branch)
		if err == nil && pure {
			return IsPure(self.Right, envm, envp, branch)
		}
		return pure, err
	}
	if self, ok := self.(And); ok {
		pure, err := IsPure(self.Left, envm, envp, branch)
		if err == nil && pure {
			return IsPure(self.Right, envm, envp, branch)
		}
		return pure, err
	}
	if self, ok := self.(Or); ok {
		pure, err := IsPure(self.Left, envm, envp, branch)
		if err == nil && pure {
			return IsPure(self.Right, envm, envp, branch)
		}
		return pure, err
	}
	if self, ok := self.(Add); ok {
		pure, err := IsPure(self.Left, envm, envp, branch)
		if err == nil && pure {
			return IsPure(self.Right, envm, envp, branch)
		}
		return pure, err
	}
	if self, ok := self.(Sub); ok {
		pure, err := IsPure(self.Left, envm, envp, branch)
		if err == nil && pure {
			return IsPure(self.Right, envm, envp, branch)
		}
		return pure, err
	}
	if self, ok := self.(Mul); ok {
		pure, err := IsPure(self.Left, envm, envp, branch)
		if err == nil && pure {
			return IsPure(self.Right, envm, envp, branch)
		}
		return pure, err
	}
	if self, ok := self.(Div); ok {
		pure, err := IsPure(self.Left, envm, envp, branch)
		if err == nil && pure {
			return IsPure(self.Right, envm, envp, branch)
		}
		return pure, err
	}
	if self, ok := self.(Less); ok {
		pure, err := IsPure(self.Left, envm, envp, branch)
		if err == nil && pure {
			return IsPure(self.Right, envm, envp, branch)
		}
		return pure, err
	}
	if self, ok := self.(Greater); ok {
		pure, err := IsPure(self.Left, envm, envp, branch)
		if err == nil && pure {
			return IsPure(self.Right, envm, envp, branch)
		}
		return pure, err
	}
	if self, ok := self.(Satisfies); ok {
		return IsPure(self.Expression, envm, envp, branch)
	}
	if self, ok := self.(Not); ok {
		return IsPure(self.Expression, envm, envp, branch)
	}
	if self, ok := self.(Raise); ok {
		return IsPure(self.Expression, envm, envp, branch)
	}
	panic(reflect.TypeOf(self))
}

// Analyze the given expression in
// the provided environment.
// ---
// Determines the implications that the
// expression has in the header of a
// conditional expression.
//
// If an error occurs, it is returned instead.
func Analyze(self Expression, envm env.Environment[Macro], envp env.Environment[pat.Pattern], branch env.Branch[Macro]) (Implication, error) {
	if self, ok := self.(Compare); ok {
		acommutatively := func(left Expression, right Expression) (Implication, error) {
			if left, ok := left.(Application); ok {
				if right, ok := right.(Literal); ok {
					constant, err := left.IsConstant(envm, envp, branch)
					if err == nil && constant {
						return &ImpFact{
							identifier: left.Identifier,
							pattern:    pat.Literal(right.Literal),
						}, nil
					}
					return nil, err
				}
				return nil, nil
			}
			return nil, nil
		}
		left, err := acommutatively(self.Left, self.Right)
		if err == nil {
			if left == nil {
				right, err := acommutatively(self.Right, self.Left)
				if err == nil {
					if right == nil {
						return &ImpTop{}, nil
					}
					return right, nil
				}
				return nil, err
			}
			return left, nil
		}
		return nil, err
	}
	if self, ok := self.(Satisfies); ok {
		if application, ok := self.Expression.(Application); ok {
			constant, err := application.IsConstant(envm, envp, branch)
			if err == nil && constant {
				return &ImpFact{
					identifier: application.Identifier,
					pattern:    self.Pattern,
				}, nil
			}
			return &ImpTop{}, err
		}
		return &ImpTop{}, nil
	}
	if self, ok := self.(And); ok {
		left, err := Analyze(self.Left, envm, envp, branch)
		if err == nil {
			right, err := Analyze(self.Right, envm, envp, branch)
			if err == nil {
				return &ImpAnd{
					left:  left,
					right: right,
				}, nil
			}
			return nil, err
		}
		return nil, err
	}
	if self, ok := self.(Or); ok {
		left, err := Analyze(self.Left, envm, envp, branch)
		if err == nil {
			right, err := Analyze(self.Right, envm, envp, branch)
			if err == nil {
				return &ImpOr{
					left:  left,
					right: right,
				}, nil
			}
			return nil, err
		}
		return nil, err
	}
	if self, ok := self.(Not); ok {
		implication, err := Analyze(self.Expression, envm, envp, branch)
		if err == nil {
			return &ImpNot{
				imp: implication,
			}, nil
		}
		return nil, err
	}
	return &ImpTop{}, nil
}
