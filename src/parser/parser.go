package parser

import (
	"circumscribe/expr"
	"circumscribe/pat"
	"circumscribe/source"
	"strings"
)

type Definition interface {
	Name() string
	Origin() source.Excerpt
	def()
}

type Macro struct {
	Identifier string
	Macro      expr.Macro
	Excerpt    source.Excerpt
}

func (self Macro) Name() string {
	return self.Identifier
}

func (self Macro) Origin() source.Excerpt {
	return self.Excerpt
}

func (self Macro) def() {}

type Pattern struct {
	Identifier string
	Pattern    pat.Pattern
	Excerpt    source.Excerpt
}

func (self Pattern) Name() string {
	return self.Identifier
}

func (self Pattern) Origin() source.Excerpt {
	return self.Excerpt
}

func (self Pattern) def() {}

const (
	Postfix = iota
	Mul
	Add
	Comp
	And
	Or
	All
)

func ReadExpression(reader *source.Reader, precedence int) (expr.Expression, error) {
	if reader.SkipWhitespace() {
		position := reader.Position()
		if reader.ReadOnly("match") {
			grammar, diagnostic := pat.ReadGrammar(reader)
			if diagnostic == nil {
				if reader.ReadMeaningful("in") {
					src, diagnostic := ReadExpression(reader, All)
					if diagnostic == nil {
						if reader.ReadMeaningful(":") {
							body, diagnostic := ReadExpression(reader, All)
							if diagnostic == nil {
								if reader.ReadMeaningful("else") {
									other, diagnostic := ReadExpression(reader, All)
									if diagnostic == nil {
										return expr.Match{
											Grammar:   grammar,
											Source:    src,
											Body:      body,
											Otherwise: other,
											Excerpt:   other.Origin().Expand(position),
										}, nil
									}
									return nil, diagnostic
								}
								return nil, source.Diagnose("Expected 'else'", reader.PointExcerpt())
							}
							return nil, diagnostic
						}
						return nil, source.Diagnose("Expected ':'", reader.PointExcerpt())
					}
					return nil, diagnostic
				}
				return nil, source.Diagnose("Expected 'in'", reader.PointExcerpt())
			}
			return nil, diagnostic
		}
		if reader.ReadOnly("if") {
			condition, diagnostic := ReadExpression(reader, All)
			if diagnostic == nil {
				if reader.ReadMeaningful(":") {
					body, diagnostic := ReadExpression(reader, All)
					if diagnostic == nil {
						if reader.ReadMeaningful("else") {
							otherwise, diagnostic := ReadExpression(reader, All)
							if diagnostic == nil {
								return ReadTrailing(reader, expr.If{
									Condition: condition,
									Body:      body,
									Otherwise: otherwise,
									Excerpt:   otherwise.Origin().Expand(position),
								}, precedence)
							}
							return nil, diagnostic
						}
						return nil, source.Diagnose("Expected 'else'", reader.PointExcerpt())
					}
					return nil, diagnostic
				}
				return nil, source.Diagnose("Expected ':'", reader.PointExcerpt())
			}
			return nil, diagnostic
		}
		if reader.ReadOnly("not") {
			expression, err := ReadExpression(reader, Postfix)
			if err == nil {
				return ReadTrailing(reader, expr.Not{
					Expression: expression,
					Excerpt:    expression.Origin().Expand(position),
				}, precedence)
			}
			return nil, err
		}
		if reader.IsAt("|") {
			literal, diagnostic := ReadLiteral(reader)
			if diagnostic == nil {
				return ReadTrailing(reader, literal, precedence)
			}
			return literal, diagnostic
		}
		identifier, diagnostic := reader.ReadIdentifier()
		if diagnostic == nil {
			if reader.ReadMeaningful("(") {
				var buffer []expr.Expression
				if reader.SkipWhitespace() && reader.ReadOnly(")") {
					return ReadTrailing(reader, expr.Application{
						Identifier: identifier,
						Arguments:  []expr.Expression{},
						Excerpt:    reader.Excerpt(position),
						Bare:       false,
					}, precedence)
				}
				for reader.SkipWhitespace() {
					argument, diagnostic := ReadExpression(reader, precedence)
					if diagnostic == nil {
						buffer = append(buffer, argument)
						if reader.ReadMeaningful(",") {
							continue
						}
						if reader.ReadMeaningful(")") {
							return ReadTrailing(reader, expr.Application{
								Identifier: identifier,
								Arguments:  buffer,
								Excerpt:    reader.Excerpt(position),
								Bare:       false,
							}, precedence)
						}
						return nil, source.Diagnose("Expected ')'", reader.PointExcerpt())
					}
					return nil, diagnostic
				}
				return nil, source.Diagnose("Expected ')'", reader.PointExcerpt())
			}
			return ReadTrailing(reader, expr.Application{
				Identifier: identifier,
				Arguments:  []expr.Expression{},
				Excerpt:    reader.Excerpt(position),
				Bare:       true,
			}, precedence)
		}
		return nil, diagnostic
	}
	return nil, source.Diagnose("Expected an expression", reader.PointExcerpt())
}

func ReadTrailing(reader *source.Reader, expression expr.Expression, precedence int) (expr.Expression, error) {
	if precedence >= Mul && reader.ReadMeaningful("*") {
		right, err := ReadExpression(reader, Mul)
		if err == nil {
			return ReadTrailing(reader, expr.Mul{
				Left:    expression,
				Right:   right,
				Excerpt: expression.Origin().Union(*right.Origin()),
			}, precedence)
		}
		return right, err
	}
	if precedence >= Mul && reader.ReadMeaningful("/") {
		right, err := ReadExpression(reader, Mul)
		if err == nil {
			return ReadTrailing(reader, expr.Div{
				Left:    expression,
				Right:   right,
				Excerpt: expression.Origin().Union(*right.Origin()),
			}, precedence)
		}
		return right, err
	}
	if precedence >= Add && reader.ReadMeaningful("+") {
		right, err := ReadExpression(reader, Add)
		if err == nil {
			return ReadTrailing(reader, expr.Add{
				Left:    expression,
				Right:   right,
				Excerpt: expression.Origin().Union(*right.Origin()),
			}, precedence)
		}
		return right, err
	}
	if precedence >= Add && reader.ReadMeaningful("-") {
		right, err := ReadExpression(reader, Add)
		if err == nil {
			return ReadTrailing(reader, expr.Sub{
				Left:    expression,
				Right:   right,
				Excerpt: expression.Origin().Union(*right.Origin()),
			}, precedence)
		}
		return right, err
	}
	if precedence >= Comp && reader.ReadMeaningful("=") {
		other, err := ReadExpression(reader, Comp)
		if err == nil {
			return ReadTrailing(reader, expr.Compare{
				Left:    expression,
				Right:   other,
				Excerpt: expression.Origin().Union(*other.Origin()),
			}, precedence)
		}
		return other, err
	}
	if precedence >= Comp && reader.ReadMeaningful("satisfies") {
		pattern, err := pat.ReadPattern(reader, pat.PrecAll)
		if err == nil {
			return ReadTrailing(reader, expr.Satisfies{
				Expression: expression,
				Pattern:    pattern,
				Excerpt:    expression.Origin().Union(*pattern.Origin()),
			}, precedence)
		}
		return nil, err
	}
	if precedence >= And && reader.ReadMeaningful("and") {
		right, err := ReadExpression(reader, Comp)
		if err == nil {
			return ReadTrailing(reader, expr.And{
				Left:    expression,
				Right:   right,
				Excerpt: expression.Origin().Union(*right.Origin()),
			}, precedence)
		}
		return right, err
	}
	if precedence >= Or && reader.ReadMeaningful("or") {
		right, err := ReadExpression(reader, Comp)
		if err == nil {
			return ReadTrailing(reader, expr.Or{
				Left:    expression,
				Right:   right,
				Excerpt: expression.Origin().Union(*right.Origin()),
			}, precedence)
		}
		return right, err
	}
	return expression, nil
}

func ReadDefinition(reader *source.Reader) (Definition, error) {
	if reader.SkipWhitespace() {
		position := reader.Position()
		if reader.ReadMeaningful("def") {
			identifier, diagnostic := reader.ReadIdentifier()
			if diagnostic == nil {
				parameters, diagnostic := ReadParameters(reader)
				if diagnostic == nil {
					annotation, diagnostic := func() (pat.Pattern, error) {
						if reader.ReadMeaningful("->") {
							annotation, diagnostic := pat.ReadPattern(reader, pat.PrecAll)
							if diagnostic == nil {
								return annotation, nil
							}
							return nil, diagnostic
						}
						return nil, nil
					}()
					if diagnostic == nil {
						if reader.ReadMeaningful(":") {
							body, diagnostic := ReadExpression(reader, All)
							if diagnostic == nil {
								excerpt := body.Origin().Expand(position)
								return Macro{
									Identifier: identifier,
									Macro: &expr.Source{
										Identifier: identifier,
										Pattern:    annotation,
										Params:     parameters,
										Body: struct {
											Source expr.Expression
											Folded expr.Expression
										}{
											Source: body,
											Folded: nil,
										},
										Excerpt: excerpt,
									},
									Excerpt: excerpt,
								}, nil
							}
							return nil, diagnostic
						}
						return nil, source.Diagnose("Expected ':'", reader.PointExcerpt())
					}
					return nil, diagnostic
				}
				return nil, diagnostic
			}
			return nil, diagnostic
		}
		if reader.ReadMeaningful("pat") {
			identifier, diagnostic := reader.ReadIdentifier()
			if diagnostic == nil {
				if reader.ReadMeaningful(":") {
					pattern, diagnostic := pat.ReadPattern(reader, pat.PrecAll)
					if diagnostic == nil {
						return Pattern{
							Identifier: identifier,
							Pattern:    pattern,
							Excerpt:    pattern.Origin().Expand(position),
						}, nil
					}
					return nil, diagnostic
				}
				return nil, source.Diagnose("Expected ':'", reader.PointExcerpt())
			}
			return nil, diagnostic
		}
		return nil, source.Diagnose("Expected 'def'", reader.PointExcerpt())
	}
	return nil, nil
}

func ReadParameters(reader *source.Reader) ([]expr.Parameter, error) {
	if reader.ReadMeaningful("(") {
		buffer := []expr.Parameter{}
		if reader.ReadMeaningful(")") {
			return []expr.Parameter{}, nil
		}
		for reader.SkipWhitespace() {
			identifier, diagnostic := reader.ReadIdentifier()
			if diagnostic == nil {
				annotation, diagnostic := func() (pat.Pattern, error) {
					if reader.ReadMeaningful(":") {
						return pat.ReadPattern(reader, pat.PrecAll)
					}
					return pat.StdAny, nil
				}()
				if diagnostic == nil {
					buffer = append(buffer, expr.Parameter{
						Name:    identifier,
						Pattern: annotation,
					})
					if reader.ReadMeaningful(",") {
						continue
					}
					if reader.ReadMeaningful(")") {
						return buffer, nil
					}
					return nil, source.Diagnose("Expected ')'", reader.PointExcerpt())
				}
				return nil, diagnostic
			}
			return nil, diagnostic
		}
		return nil, source.Diagnose("Expected ')'", reader.PointExcerpt())
	}
	return []expr.Parameter{}, nil
}

// Read a single literal from the given Reader.
func ReadLiteral(reader *source.Reader) (expr.Expression, error) {
	if reader.SkipWhitespace() {
		position := reader.Position()
		if reader.ReadOnly("|") {
			var children []expr.Expression
			var buffer strings.Builder
			for reader.CanRead() {
				if reader.ReadOnly("|") {
					children = append(children, expr.Literal{
						Literal: buffer.String(),
						Excerpt: reader.Excerpt(position),
					})
					return expr.Snippet{
						Children: children,
						Excerpt:  children[0].Origin().Expand(reader.Position()),
					}, nil
				}
				if reader.ReadOnly("<") {
					children = append(children, expr.Literal{
						Literal: buffer.String(),
						Excerpt: reader.Excerpt(position),
					})
					expression, err := ReadExpression(reader, All)
					if err == nil {
						children = append(children, expression)
						if reader.SkipWhitespace() {
							position = reader.Position()
							if reader.ReadOnly(">") {
								buffer.Reset()
								continue
							}
							return nil, source.Diagnose("Expected '>'", reader.PointExcerpt())
						}
						return nil, source.Diagnose("Expected '>'", reader.PointExcerpt())
					}
					return nil, err
				}
				if reader.ReadOnly("\\") {
					if reader.ReadOnly("|") {
						buffer.WriteRune('|')
						continue
					}
					if reader.ReadOnly("u") {
						point, err := reader.ReadUnicode()
						if err == nil {
							buffer.WriteRune(point)
							continue
						}
						return nil, err
					}
					if reader.ReadOnly("<") {
						buffer.WriteRune('<')
						continue
					}
					if reader.ReadOnly("\\") {
						buffer.WriteRune('\\')
						continue
					}
					if reader.ReadOnly("n") {
						buffer.WriteRune('\n')
						continue
					}
					if reader.ReadOnly("r") {
						buffer.WriteRune('\r')
						continue
					}
					if reader.ReadOnly("t") {
						buffer.WriteRune('\t')
						continue
					}
					if reader.ReadOnly("b") {
						buffer.WriteRune('\b')
						continue
					}
					return nil, source.Diagnose("Unrecognized escape sequence", reader.PointExcerpt())
				}
				buffer.WriteRune(reader.Read())
			}
			return nil, source.Diagnose("Expected '|'", reader.PointExcerpt())
		}
		return nil, source.Diagnose("Expected '|'", reader.PointExcerpt())
	}
	return nil, source.Diagnose("Expected '|'", reader.PointExcerpt())
}
