package expr

import (
	"circumscribe/env"
	"circumscribe/pat"
	"circumscribe/source"
	"fmt"
	"math/big"
	"reflect"
	"strings"
)

type Expression interface {
	Origin() *source.Excerpt
	expr()
}

func Evaluate(self Expression, envm env.Environment[Macro], envp env.Environment[pat.Pattern]) (string, error) {
	if self, ok := self.(Literal); ok {
		return self.Literal, nil
	}
	if self, ok := self.(Snippet); ok {
		var buffer strings.Builder
		for _, child := range self.Children {
			snippet, diagnostic := Evaluate(child, envm, envp)
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
			snippet, diagnostic := Evaluate(argument, envm, envp)
			if diagnostic == nil {
				buffer = append(buffer, snippet)
				continue
			}
			return "", diagnostic
		}
		entry, _ := envm.Traverse(self.Identifier)
		if _, ok := entry.(Native); ok {
			snippet, err := entry.Evaluate(envm, envp, buffer)
			if err == nil {
				return snippet, nil
			}
			return "", source.Diagnose(err.Error(), self.Excerpt)
		}
		return entry.Evaluate(envm, envp, buffer)
	}
	if self, ok := self.(Match); ok {
		source, diagnostic := Evaluate(self.Source, envm, envp)
		if diagnostic == nil {
			layer := self.Grammar.Match(source)
			if layer != nil {
				buffer := map[string]Macro{}
				for name, snippet := range layer {
					buffer[name] = Constant{
						identifier: name,
						constant:   snippet,
					}
					continue
				}
				return Evaluate(self.Body, env.Environment[Macro]{
					Root:    envm.Root,
					Parent:  &envm,
					Entries: buffer,
				}, env.Environment[pat.Pattern]{
					Root:    envp.Root,
					Parent:  &envp,
					Entries: env.Layer[pat.Pattern]{},
				})
			}
			return Evaluate(self.Otherwise, envm, envp)
		}
		return source, diagnostic
	}
	if self, ok := self.(If); ok {
		condition, diagnostic := Evaluate(self.Condition, envm, envp)
		if diagnostic == nil {
			if condition == "true" {
				return Evaluate(self.Body, envm, envp)
			}
			return Evaluate(self.Otherwise, envm, envp)
		}
		return condition, diagnostic
	}
	if self, ok := self.(Compare); ok {
		left, dignostic := Evaluate(self.Left, envm, envp)
		if dignostic == nil {
			right, diagnostic := Evaluate(self.Right, envm, envp)
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
		left, diagnostic := Evaluate(self.Left, envm, envp)
		if diagnostic == nil {
			if left == "true" {
				return Evaluate(self.Right, envm, envp)
			}
			return "false", nil
		}
		return left, diagnostic
	}
	if self, ok := self.(Or); ok {
		left, diagnostic := Evaluate(self.Left, envm, envp)
		if diagnostic == nil {
			if left == "false" {
				return Evaluate(self.Right, envm, envp)
			}
			return "true", nil
		}
		return left, diagnostic
	}
	if self, ok := self.(Not); ok {
		snippet, err := Evaluate(self.Expression, envm, envp)
		if err == nil {
			if snippet == "true" {
				return "false", nil
			}
			return "true", nil
		}
		return snippet, err
	}
	if self, ok := self.(Satisfies); ok {
		snippet, err := Evaluate(self.Expression, envm, envp)
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
		left, diagnostic := Evaluate(self.Left, envm, envp)
		if diagnostic == nil {
			right, diagnostic := Evaluate(self.Right, envm, envp)
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
		left, diagnostic := Evaluate(self.Left, envm, envp)
		if diagnostic == nil {
			right, diagnostic := Evaluate(self.Right, envm, envp)
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
		left, diagnostic := Evaluate(self.Left, envm, envp)
		if diagnostic == nil {
			right, diagnostic := Evaluate(self.Right, envm, envp)
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
		left, diagnostic := Evaluate(self.Left, envm, envp)
		if diagnostic == nil {
			right, diagnostic := Evaluate(self.Right, envm, envp)
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
	panic("Evaluate(Expression, Environment)")
}

func Fold(self Expression, envm env.Environment[Macro], envp env.Environment[pat.Pattern], branch env.Branch[Macro]) (Expression, pat.Pattern, error) {
	if self, ok := self.(Literal); ok {
		return self, pat.Literal(self.Literal), nil
	}
	if self, ok := self.(Snippet); ok {
		children, patterns, diagnostic := func() ([]Expression, []pat.Pattern, error) {
			var children []Expression
			var patterns []pat.Pattern
			for _, child := range self.Children {
				expression, pattern, diagnostic := Fold(child, envm, envp, branch)
				if diagnostic == nil {
					if expression, ok := expression.(Literal); ok && len(expression.Literal) == 0 {
						continue
					}
					children = append(children, expression)
					patterns = append(patterns, pattern)
					continue
				}
				return children, patterns, diagnostic
			}
			return children, patterns, nil
		}()
		if diagnostic == nil {
			if len(children) > 0 {
				if len(children) == 1 {
					return children[0], patterns[0], nil
				}
				buffer := patterns[0]
				for _, child := range patterns[1:] {
					buffer = pat.Product(buffer, child)
				}
				return Snippet{
					Children: children,
					Excerpt:  self.Excerpt,
				}, buffer, nil
			}
			return Literal{
				Literal: "",
				Excerpt: self.Excerpt,
			}, pat.Literal(""), nil
		}
		return nil, nil, diagnostic
	}
	if self, ok := self.(Application); ok {
		if macro, ok := envm.Traverse(self.Identifier); ok {
			parameters := macro.Parameters()
			arguments, err := func() ([]Expression, error) {
				if len(self.Arguments) == len(parameters) {
					var buffer []Expression
					for n, argument := range self.Arguments {
						expression, pattern, err := Fold(argument, envm, envp, branch)
						if err == nil {
							parameter := parameters[n]
							err := pat.Assign(envp, pattern, parameter.Pattern)
							if err == nil {
								buffer = append(buffer, expression)
								continue
							}
							return nil, source.Diagnose(err.Error(), self.Excerpt)
						}
						return nil, err
					}
					return buffer, nil
				}
				return nil, source.Diagnostic{
					Message: fmt.Sprintf("%s expected %d arguments, but %d were provided", self.Identifier, len(parameters), len(self.Arguments)),
					Excerpt: self.Origin(),
				}
			}()
			if err == nil {
				inferred, err := macro.Infer(envm, envp, branch)
				if err == nil {
					pure, err := macro.IsPure(envm, envp, branch)
					if err == nil {
						if !self.Bare || pure {
							if macro, ok := macro.(Constant); ok {
								return Literal{
									Literal: macro.constant,
									Excerpt: self.Excerpt,
								}, inferred, nil
							}
							return Application{
								Identifier: self.Identifier,
								Arguments:  arguments,
								Bare:       self.Bare,
								Excerpt:    self.Excerpt,
							}, inferred, nil
						}
						return nil, nil, source.Diagnose("Bare application of an impure function", self.Excerpt)
					}
					return nil, nil, err
				}
				return nil, nil, err
			}
			return nil, nil, err
		}
		return nil, nil, source.Diagnose(fmt.Sprintf("Could not resolve identifier '%s'", self.Identifier), self.Excerpt)
	}
	if self, ok := self.(Match); ok {
		err := self.Grammar.Compile(envp)
		if err == nil {
			source, _, err := Fold(self.Source, envm, envp, branch)
			if err == nil {
				other, other_p, err := Fold(self.Otherwise, envm, envp, branch)
				if err == nil {
					if source, ok := source.(Literal); ok {
						layer := self.Grammar.Match(source.Literal)
						if layer != nil {
							buffer := env.Layer[Macro]{}
							for name, snippet := range layer {
								buffer[name] = Constant{
									identifier: name,
									constant:   snippet,
								}
								continue
							}
							return Fold(self.Body, env.Environment[Macro]{
								Root:    envm.Root,
								Parent:  &envm,
								Entries: buffer,
							}, env.Environment[pat.Pattern]{
								Root:    envp.Root,
								Parent:  &envp,
								Entries: env.Layer[pat.Pattern]{},
							}, branch)
						}
						return other, other_p, nil
					}
					body, body_p, err := Fold(self.Body, env.Environment[Macro]{
						Root:   envm.Root,
						Parent: &envm,
						Entries: func() env.Layer[Macro] {
							buffer := env.Layer[Macro]{}
							for _, binding := range self.Grammar.Bindings() {
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
						return Match{
							Grammar:   self.Grammar,
							Source:    source,
							Body:      body,
							Otherwise: other,
							Excerpt:   self.Excerpt,
						}, pat.Sum(body_p, other_p), nil
					}
					return nil, nil, err
				}
				return nil, nil, err
			}
			return nil, nil, err
		}
		return nil, nil, err
	}
	if self, ok := self.(If); ok {
		cond, cond_p, err := Fold(self.Condition, envm, envp, branch)
		if err == nil {
			analysis, err := Analyze(cond, envm, envp, branch)
			if err == nil {
				envf, err := Refine(analysis, envm, envp)
				if err == nil {
					body, body_p, err := Fold(self.Body, envf, env.Environment[pat.Pattern]{
						Root:    envp.Root,
						Parent:  &envp,
						Entries: env.Layer[pat.Pattern]{},
					}, branch)
					if err == nil {
						enve, err := Refine(analysis.Complement(), envm, envp)
						if err == nil {
							other, other_p, err := Fold(self.Otherwise, enve, env.Environment[pat.Pattern]{
								Root:    envp.Root,
								Parent:  &envp,
								Entries: env.Layer[pat.Pattern]{},
							}, branch)
							if err == nil {
								err := pat.Assign(envp, cond_p, pat.StdBoolean)
								if err == nil {
									if literal, ok := cond.(Literal); ok {
										if literal.Literal == "true" {
											return body, body_p, nil
										}
										return other, other_p, nil
									}
									return If{
										Condition: cond,
										Body:      body,
										Otherwise: other,
										Excerpt:   self.Excerpt,
									}, pat.Sum(body_p, other_p), nil
								}
								return nil, nil, source.Diagnostic{
									Message: err.Error(),
									Excerpt: cond.Origin(),
								}
							}
							return nil, nil, err
						}
						return nil, nil, err
					}
					return nil, nil, err
				}
				return nil, nil, err
			}
			return nil, nil, err
		}
		return nil, nil, err
	}
	if self, ok := self.(Compare); ok {
		left, left_p, diagnostic := Fold(self.Left, envm, envp, branch)
		if diagnostic == nil {
			right, right_p, diagnostic := Fold(self.Right, envm, envp, branch)
			if diagnostic == nil {
				if left, ok := left.(Literal); ok {
					if right, ok := right.(Literal); ok {
						if left.Literal == right.Literal {
							return Literal{
								Literal: "true",
								Excerpt: self.Excerpt,
							}, pat.Literal("true"), nil
						}
						return Literal{
							Literal: "false",
							Excerpt: self.Excerpt,
						}, pat.Literal("false"), nil
					}
					return Compare{
						Left:    left,
						Right:   right,
						Excerpt: self.Excerpt,
					}, pat.StdBoolean, nil
				}
				return Compare{
					Left:    left,
					Right:   right,
					Excerpt: self.Excerpt,
				}, pat.StdBoolean, nil
			}
			return right, right_p, diagnostic
		}
		return left, left_p, diagnostic
	}
	if self, ok := self.(Satisfies); ok {
		expression, pattern, err := Fold(self.Expression, envm, envp, branch)
		if err == nil {
			if err := pat.Assign(envp, pattern, self.Pattern); err == nil {
				return Literal{
					Literal: "true",
					Excerpt: self.Excerpt,
				}, pat.StdBoolean, nil
			}
			return Satisfies{
				Expression: expression,
				Pattern:    self.Pattern,
				Excerpt:    self.Excerpt,
			}, pat.StdBoolean, nil
		}
		return nil, nil, err
	}
	if self, ok := self.(And); ok {
		left, leftp, err := Fold(self.Left, envm, envp, branch)
		if err == nil {
			err := pat.Assign(envp, leftp, pat.StdBoolean)
			if err == nil {
				analysis, err := Analyze(left, envm, envp, branch)
				if err == nil {
					environment, err := Refine(analysis, envm, envp)
					if err == nil {
						right, rightp, err := Fold(self.Right, environment, env.Environment[pat.Pattern]{
							Root:    envp.Root,
							Parent:  &envp,
							Entries: env.Layer[pat.Pattern]{},
						}, branch)
						if err == nil {
							err := pat.Assign(envp, rightp, pat.StdBoolean)
							if err == nil {
								if left, ok := left.(Literal); ok {
									if right, ok := right.(Literal); ok {
										literal, err := Evaluate(And{
											Left:    left,
											Right:   right,
											Excerpt: self.Excerpt,
										}, envm, envp)
										if err == nil {
											return Literal{
												Literal: literal,
												Excerpt: self.Excerpt,
											}, pat.Literal(literal), nil
										}
										return nil, nil, err
									}
									return And{
										Left:    left,
										Right:   right,
										Excerpt: self.Excerpt,
									}, pat.StdBoolean, nil
								}
								return And{
									Left:    left,
									Right:   right,
									Excerpt: self.Excerpt,
								}, pat.StdBoolean, nil
							}
							return nil, nil, err
						}
						return nil, nil, err
					}
					return nil, nil, err
				}
				return nil, nil, err
			}
			return nil, nil, err
		}
		return nil, nil, err
	}
	if self, ok := self.(Or); ok {
		left, leftp, err := Fold(self.Left, envm, envp, branch)
		if err == nil {
			err := pat.Assign(envp, leftp, pat.StdBoolean)
			if err == nil {
				analysis, err := Analyze(left, envm, envp, branch)
				if err == nil {
					environment, err := Refine(analysis.Complement(), envm, envp)
					if err == nil {
						right, rightp, err := Fold(self.Right, environment, env.Environment[pat.Pattern]{
							Root:    envp.Root,
							Parent:  &envp,
							Entries: env.Layer[pat.Pattern]{},
						}, branch)
						if err == nil {
							err := pat.Assign(envp, rightp, pat.StdBoolean)
							if err == nil {
								if left, ok := left.(Literal); ok {
									if right, ok := right.(Literal); ok {
										literal, err := Evaluate(Or{
											Left:    left,
											Right:   right,
											Excerpt: self.Excerpt,
										}, envm, envp)
										if err == nil {
											return Literal{
												Literal: literal,
												Excerpt: self.Excerpt,
											}, pat.Literal(literal), nil
										}
										return nil, nil, err
									}
									return Or{
										Left:    left,
										Right:   right,
										Excerpt: self.Excerpt,
									}, pat.StdBoolean, nil
								}
								return Or{
									Left:    left,
									Right:   right,
									Excerpt: self.Excerpt,
								}, pat.StdBoolean, nil
							}
							return nil, nil, err
						}
						return nil, nil, err
					}
					return nil, nil, err
				}
				return nil, nil, err
			}
			return nil, nil, err
		}
		return nil, nil, err
	}
	if self, ok := self.(Add); ok {
		left, leftp, err := Fold(self.Left, envm, envp, branch)
		if err == nil {
			err := pat.Assign(envp, leftp, pat.StdNatural)
			if err == nil {
				right, rightp, err := Fold(self.Right, envm, envp, branch)
				if err == nil {
					err := pat.Assign(envp, rightp, pat.StdNatural)
					if err == nil {
						if left, ok := left.(Literal); ok {
							if right, ok := right.(Literal); ok {
								literal, err := Evaluate(Add{
									Left:    left,
									Right:   right,
									Excerpt: self.Excerpt,
								}, envm, envp)
								if err == nil {
									return Literal{
										Literal: literal,
										Excerpt: self.Excerpt,
									}, pat.Literal(literal), nil
								}
								return nil, nil, err
							}
							return Add{
								Left:    left,
								Right:   right,
								Excerpt: self.Excerpt,
							}, pat.StdNatural, nil
						}
						return Add{
							Left:    left,
							Right:   right,
							Excerpt: self.Excerpt,
						}, pat.StdNatural, nil
					}
					return nil, nil, source.Diagnostic{
						Message: err.Error(),
						Excerpt: right.Origin(),
					}
				}
				return nil, nil, err
			}
			return nil, nil, source.Diagnostic{
				Message: err.Error(),
				Excerpt: left.Origin(),
			}
		}
		return nil, nil, err
	}
	if self, ok := self.(Not); ok {
		expression, pattern, err := Fold(self.Expression, envm, envp, branch)
		if err == nil {
			err := pat.Assign(envp, pattern, pat.StdBoolean)
			if err == nil {
				if expression, ok := expression.(Literal); ok {
					if expression.Literal == "true" {
						return Literal{
							Literal: "false",
							Excerpt: self.Excerpt,
						}, pat.Literal("false"), nil
					}
					return Literal{
						Literal: "true",
						Excerpt: self.Excerpt,
					}, pat.Literal("true"), nil
				}
				return Not{
					Expression: expression,
					Excerpt:    self.Excerpt,
				}, pat.StdBoolean, nil
			}
			return nil, nil, err
		}
		return nil, nil, err
	}
	if self, ok := self.(Sub); ok {
		left, leftp, err := Fold(self.Left, envm, envp, branch)
		if err == nil {
			err := pat.Assign(envp, leftp, pat.StdNatural)
			if err == nil {
				right, rightp, err := Fold(self.Right, envm, envp, branch)
				if err == nil {
					err := pat.Assign(envp, rightp, pat.StdNatural)
					if err == nil {
						if left, ok := left.(Literal); ok {
							if right, ok := right.(Literal); ok {
								literal, err := Evaluate(Sub{
									Left:    left,
									Right:   right,
									Excerpt: self.Excerpt,
								}, envm, envp)
								if err == nil {
									return Literal{
										Literal: literal,
										Excerpt: self.Excerpt,
									}, pat.Literal(literal), nil
								}
								return nil, nil, err
							}
							return Sub{
								Left:    left,
								Right:   right,
								Excerpt: self.Excerpt,
							}, pat.StdNatural, nil
						}
						return Sub{
							Left:    left,
							Right:   right,
							Excerpt: self.Excerpt,
						}, pat.StdNatural, nil
					}
					return nil, nil, source.Diagnostic{
						Message: err.Error(),
						Excerpt: right.Origin(),
					}
				}
				return nil, nil, err
			}
			return nil, nil, source.Diagnostic{
				Message: err.Error(),
				Excerpt: left.Origin(),
			}
		}
		return nil, nil, err
	}
	if self, ok := self.(Mul); ok {
		left, leftp, err := Fold(self.Left, envm, envp, branch)
		if err == nil {
			err := pat.Assign(envp, leftp, pat.StdNatural)
			if err == nil {
				right, rightp, err := Fold(self.Right, envm, envp, branch)
				if err == nil {
					err := pat.Assign(envp, rightp, pat.StdNatural)
					if err == nil {
						if left, ok := left.(Literal); ok {
							if right, ok := right.(Literal); ok {
								literal, err := Evaluate(Mul{
									Left:    left,
									Right:   right,
									Excerpt: self.Excerpt,
								}, envm, envp)
								if err == nil {
									return Literal{
										Literal: literal,
										Excerpt: self.Excerpt,
									}, pat.Literal(literal), nil
								}
								return nil, nil, err
							}
							return Mul{
								Left:    left,
								Right:   right,
								Excerpt: self.Excerpt,
							}, pat.StdNatural, nil
						}
						return Mul{
							Left:    left,
							Right:   right,
							Excerpt: self.Excerpt,
						}, pat.StdNatural, nil
					}
					return nil, nil, source.Diagnostic{
						Message: err.Error(),
						Excerpt: right.Origin(),
					}
				}
				return nil, nil, err
			}
			return nil, nil, source.Diagnostic{
				Message: err.Error(),
				Excerpt: left.Origin(),
			}
		}
		return nil, nil, err
	}
	if self, ok := self.(Div); ok {
		left, leftp, err := Fold(self.Left, envm, envp, branch)
		if err == nil {
			err := pat.Assign(envp, leftp, pat.StdNatural)
			if err == nil {
				right, rightp, err := Fold(self.Right, envm, envp, branch)
				if err == nil {
					err := pat.Assign(envp, rightp, pat.StdNatural)
					if err == nil {
						if left, ok := left.(Literal); ok {
							if right, ok := right.(Literal); ok {
								literal, err := Evaluate(Div{
									Left:    left,
									Right:   right,
									Excerpt: self.Excerpt,
								}, envm, envp)
								if err == nil {
									return Literal{
										Literal: literal,
										Excerpt: self.Excerpt,
									}, pat.Literal(literal), nil
								}
								return nil, nil, err
							}
							return Div{
								Left:    left,
								Right:   right,
								Excerpt: self.Excerpt,
							}, pat.StdNatural, nil
						}
						return Div{
							Left:    left,
							Right:   right,
							Excerpt: self.Excerpt,
						}, pat.StdNatural, nil
					}
					return nil, nil, source.Diagnostic{
						Message: err.Error(),
						Excerpt: right.Origin(),
					}
				}
				return nil, nil, err
			}
			return nil, nil, source.Diagnostic{
				Message: err.Error(),
				Excerpt: left.Origin(),
			}
		}
		return nil, nil, err
	}
	panic(fmt.Sprintf("Fold(%s)", reflect.TypeOf(self)))
}

func Purity(self Expression, envm env.Environment[Macro], envp env.Environment[pat.Pattern], branch env.Branch[Macro]) (bool, error) {
	if _, ok := self.(Literal); ok {
		return true, nil
	}
	if self, ok := self.(Snippet); ok {
		for _, segment := range self.Children {
			pure, err := Purity(segment, envm, envp, branch)
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
			pure, diagnostic := Purity(argument, envm, envp, branch)
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
	if self, ok := self.(Match); ok {
		pure, err := Purity(self.Source, envm, envp, branch)
		if err == nil && pure {
			pure, err := Purity(self.Otherwise, envm, envp, branch)
			if err == nil && pure {
				return Purity(self.Body, env.Environment[Macro]{
					Root:   envm.Root,
					Parent: &envm,
					Entries: func() env.Layer[Macro] {
						buffer := env.Layer[Macro]{}
						for _, binding := range self.Grammar.Bindings() {
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
			}
			return pure, err
		}
		return pure, err
	}
	if self, ok := self.(If); ok {
		pure, err := Purity(self.Condition, envm, envp, branch)
		if err == nil && pure {
			pure, err := Purity(self.Body, envm, envp, branch)
			if err == nil && pure {
				return Purity(self.Otherwise, envm, envp, branch)
			}
			return pure, err
		}
		return pure, err
	}
	if self, ok := self.(Compare); ok {
		pure, err := Purity(self.Left, envm, envp, branch)
		if err == nil && pure {
			return Purity(self.Right, envm, envp, branch)
		}
		return pure, err
	}
	if self, ok := self.(And); ok {
		pure, err := Purity(self.Left, envm, envp, branch)
		if err == nil && pure {
			return Purity(self.Right, envm, envp, branch)
		}
		return pure, err
	}
	if self, ok := self.(Or); ok {
		pure, err := Purity(self.Left, envm, envp, branch)
		if err == nil && pure {
			return Purity(self.Right, envm, envp, branch)
		}
		return pure, err
	}
	if self, ok := self.(Add); ok {
		pure, err := Purity(self.Left, envm, envp, branch)
		if err == nil && pure {
			return Purity(self.Right, envm, envp, branch)
		}
		return pure, err
	}
	if self, ok := self.(Sub); ok {
		pure, err := Purity(self.Left, envm, envp, branch)
		if err == nil && pure {
			return Purity(self.Right, envm, envp, branch)
		}
		return pure, err
	}
	if self, ok := self.(Mul); ok {
		pure, err := Purity(self.Left, envm, envp, branch)
		if err == nil && pure {
			return Purity(self.Right, envm, envp, branch)
		}
		return pure, err
	}
	if self, ok := self.(Div); ok {
		pure, err := Purity(self.Left, envm, envp, branch)
		if err == nil && pure {
			return Purity(self.Right, envm, envp, branch)
		}
		return pure, err
	}
	if self, ok := self.(Satisfies); ok {
		return Purity(self.Expression, envm, envp, branch)
	}
	if self, ok := self.(Not); ok {
		return Purity(self.Expression, envm, envp, branch)
	}
	panic(fmt.Sprintf("Purity(%s)", reflect.TypeOf(self)))
}

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
