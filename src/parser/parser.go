package parser

import (
	"circumscribe/ast"
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
	Macro      ast.Macro
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
	PrecMul
	PrecAdd
	PrecRel
	PrecComp
	PrecAnd
	PrecOr
	All
)

func ReadExpression(reader *source.Reader, precedence int, escape bool) (ast.Expression, error) {
	if reader.SkipWhitespace() {
		position := reader.Position()
		if reader.ReadOnly("(") {
			expression, err := ReadExpression(reader, All, false)
			if err == nil {
				if reader.ReadOnly(")") {
					return ReadTrailing(reader, expression, precedence, escape)
				}
				return nil, source.Diagnose("Expected ')'", reader.PointExcerpt())
			}
			return nil, err
		}
		if reader.ReadWord("match") {
			grammar, diagnostic := pat.ReadGrammar(reader)
			if diagnostic == nil {
				if reader.ReadWord("in") {
					src, diagnostic := ReadExpression(reader, All, false)
					if diagnostic == nil {
						if reader.ReadSyntax(":") {
							body, diagnostic := ReadExpression(reader, All, false)
							if diagnostic == nil {
								if reader.ReadWord("else") {
									other, diagnostic := ReadExpression(reader, precedence, escape)
									if diagnostic == nil {
										return ast.Switch{
											Source: src,
											Branches: []ast.Case{
												{
													Grammar: grammar,
													Body:    body,
												},
											},
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
		if reader.ReadWord("switch") {
			src, err := ReadExpression(reader, All, false)
			if err == nil {
				if reader.ReadSyntax(":") {
					buffer := []ast.Case{}
					for reader.SkipWhitespace() {
						if reader.ReadWord("case") {
							grammar, err := pat.ReadGrammar(reader)
							if err == nil {
								if reader.ReadSyntax(":") {
									body, err := ReadExpression(reader, All, false)
									if err == nil {
										buffer = append(buffer, ast.Case{
											Grammar: grammar,
											Body:    body,
										})
										if reader.ReadSyntax("default") {
											otherwise, err := ReadExpression(reader, precedence, escape)
											if err == nil {
												return ast.Switch{
													Source:    src,
													Branches:  buffer,
													Otherwise: otherwise,
													Excerpt:   otherwise.Origin().Expand(position),
												}, nil
											}
											return nil, err
										}
										continue
									}
									return nil, err
								}
								return nil, source.Diagnose("Expected ':'", reader.PointExcerpt())
							}
							return nil, err
						}
						return nil, source.Diagnose("Expected 'case'", reader.PointExcerpt())
					}
					return nil, source.Diagnose("Expected 'default'", reader.PointExcerpt())
				}
				return nil, source.Diagnose("Expected ':'", reader.PointExcerpt())
			}
			return nil, err
		}
		if reader.ReadWord("if") {
			condition, diagnostic := ReadExpression(reader, All, false)
			if diagnostic == nil {
				if reader.ReadSyntax(":") {
					body, diagnostic := ReadExpression(reader, All, false)
					if diagnostic == nil {
						if reader.ReadWord("else") {
							otherwise, diagnostic := ReadExpression(reader, precedence, escape)
							if diagnostic == nil {
								return ast.If{
									Condition: condition,
									Body:      body,
									Otherwise: otherwise,
									Excerpt:   otherwise.Origin().Expand(position),
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
		// TODO: Perhaps (somehow) change the order?
		if reader.ReadWord("with") {
			expression, err := ReadExpression(reader, precedence, escape)
			if err == nil {
				if reader.ReadWord("as") {
					identifier, err := reader.ReadIdentifier()
					if err == nil {
						if reader.ReadSyntax(":") {
							body, err := ReadExpression(reader, precedence, escape)
							if err == nil {
								return ast.With{
									Identifier: identifier,
									Expression: expression,
									Body:       body,
									Excerpt:    body.Origin().Expand(position),
								}, nil
							}
							return nil, err
						}
						return nil, source.Diagnose("Expected ':'", reader.PointExcerpt())
					}
					return nil, err
				}
				return nil, source.Diagnose("Expected 'as'", reader.PointExcerpt())
			}
			return nil, err
		}
		if reader.ReadWord("raise") {
			expression, err := ReadExpression(reader, precedence, escape)
			if err == nil {
				return ast.Raise{
					Expression: expression,
					Excerpt:    expression.Origin().Expand(position),
				}, nil
			}
			return nil, err
		}
		if reader.ReadWord("not") {
			expression, err := ReadExpression(reader, precedence, escape)
			if err == nil {
				return ReadTrailing(reader, ast.Not{
					Expression: expression,
					Excerpt:    expression.Origin().Expand(position),
				}, precedence, escape)
			}
			return nil, err
		}
		if reader.IsAt("|") {
			literal, diagnostic := ReadLiteral(reader)
			if diagnostic == nil {
				return ReadTrailing(reader, literal, precedence, escape)
			}
			return literal, diagnostic
		}
		identifier, diagnostic := reader.ReadIdentifier()
		if diagnostic == nil {
			if reader.ReadSyntax("(") {
				var buffer []ast.Expression
				if reader.SkipWhitespace() && reader.ReadOnly(")") {
					return ReadTrailing(reader, ast.Call{
						Identifier: identifier,
						Arguments:  []ast.Expression{},
						Excerpt:    reader.Excerpt(position),
						Bare:       false,
					}, precedence, escape)
				}
				for reader.SkipWhitespace() {
					argument, diagnostic := ReadExpression(reader, precedence, false)
					if diagnostic == nil {
						buffer = append(buffer, argument)
						if reader.ReadSyntax(",") {
							continue
						}
						if reader.ReadSyntax(")") {
							return ReadTrailing(reader, ast.Call{
								Identifier: identifier,
								Arguments:  buffer,
								Excerpt:    reader.Excerpt(position),
								Bare:       false,
							}, precedence, escape)
						}
						return nil, source.Diagnose("Expected ')'", reader.PointExcerpt())
					}
					return nil, diagnostic
				}
				return nil, source.Diagnose("Expected ')'", reader.PointExcerpt())
			}
			return ReadTrailing(reader, ast.Call{
				Identifier: identifier,
				Arguments:  []ast.Expression{},
				Excerpt:    reader.Excerpt(position),
				Bare:       true,
			}, precedence, escape)
		}
		return nil, diagnostic
	}
	return nil, source.Diagnose("Expected an expression", reader.PointExcerpt())
}

func ReadTrailing(reader *source.Reader, expression ast.Expression, precedence int, escape bool) (ast.Expression, error) {
	if precedence >= PrecMul && reader.ReadSyntax("*") {
		right, err := ReadExpression(reader, PrecMul, false)
		if err == nil {
			return ReadTrailing(reader, ast.Mul{
				Left:    expression,
				Right:   right,
				Excerpt: expression.Origin().Union(*right.Origin()),
			}, precedence, escape)
		}
		return right, err
	}
	if precedence >= PrecMul && reader.ReadSyntax("/") {
		right, err := ReadExpression(reader, PrecMul, false)
		if err == nil {
			return ReadTrailing(reader, ast.Div{
				Left:    expression,
				Right:   right,
				Excerpt: expression.Origin().Union(*right.Origin()),
			}, precedence, escape)
		}
		return right, err
	}
	if precedence >= PrecAdd && reader.ReadSyntax("+") {
		right, err := ReadExpression(reader, PrecAdd, false)
		if err == nil {
			return ReadTrailing(reader, ast.Add{
				Left:    expression,
				Right:   right,
				Excerpt: expression.Origin().Union(*right.Origin()),
			}, precedence, escape)
		}
		return right, err
	}
	if precedence >= PrecAdd && reader.ReadSyntax("-") {
		right, err := ReadExpression(reader, PrecAdd, false)
		if err == nil {
			return ReadTrailing(reader, ast.Sub{
				Left:    expression,
				Right:   right,
				Excerpt: expression.Origin().Union(*right.Origin()),
			}, precedence, escape)
		}
		return right, err
	}
	if precedence >= PrecRel && !escape && reader.ReadSyntax("<") {
		right, err := ReadExpression(reader, PrecRel, false)
		if err == nil {
			return ReadTrailing(reader, ast.Less{
				Left:    expression,
				Right:   right,
				Excerpt: expression.Origin().Union(*right.Origin()),
			}, precedence, false)
		}
		return right, err
	}
	if precedence >= PrecRel && !escape && reader.ReadSyntax(">") {
		right, err := ReadExpression(reader, PrecRel, false)
		if err == nil {
			return ReadTrailing(reader, ast.Greater{
				Left:    expression,
				Right:   right,
				Excerpt: expression.Origin().Union(*right.Origin()),
			}, precedence, false)
		}
		return right, err
	}
	if precedence >= PrecComp && reader.ReadSyntax("=") {
		other, err := ReadExpression(reader, PrecComp, escape)
		if err == nil {
			return ReadTrailing(reader, ast.Compare{
				Left:    expression,
				Right:   other,
				Excerpt: expression.Origin().Union(*other.Origin()),
			}, precedence, escape)
		}
		return other, err
	}
	if precedence >= PrecComp && reader.ReadWord("satisfies") {
		pattern, err := pat.ReadPattern(reader, pat.PrecAll)
		if err == nil {
			return ReadTrailing(reader, ast.Satisfies{
				Expression: expression,
				Pattern:    pattern,
				Excerpt:    expression.Origin().Union(*pattern.Origin()),
			}, precedence, escape)
		}
		return nil, err
	}
	if precedence >= PrecAnd && reader.ReadWord("and") {
		right, err := ReadExpression(reader, PrecAnd, escape)
		if err == nil {
			return ReadTrailing(reader, ast.And{
				Left:    expression,
				Right:   right,
				Excerpt: expression.Origin().Union(*right.Origin()),
			}, precedence, escape)
		}
		return right, err
	}
	if precedence >= PrecOr && reader.ReadWord("or") {
		right, err := ReadExpression(reader, PrecOr, escape)
		if err == nil {
			return ReadTrailing(reader, ast.Or{
				Left:    expression,
				Right:   right,
				Excerpt: expression.Origin().Union(*right.Origin()),
			}, precedence, escape)
		}
		return right, err
	}
	return expression, nil
}

func ReadEntry(reader *source.Reader) (Definition, error) {
	if reader.SkipWhitespace() {
		position := reader.Position()
		if reader.ReadWord("def") {
			identifier, diagnostic := reader.ReadIdentifier()
			if diagnostic == nil {
				parameters, diagnostic := ReadParameters(reader)
				if diagnostic == nil {
					annotation, diagnostic := func() (pat.Pattern, error) {
						if reader.ReadSyntax("->") {
							annotation, diagnostic := pat.ReadPattern(reader, pat.PrecAll)
							if diagnostic == nil {
								return annotation, nil
							}
							return nil, diagnostic
						}
						return nil, nil
					}()
					if diagnostic == nil {
						if reader.ReadSyntax(":") {
							body, diagnostic := ReadExpression(reader, All, false)
							if diagnostic == nil {
								excerpt := body.Origin().Expand(position)
								return Macro{
									Identifier: identifier,
									Macro: &ast.Source{
										Identifier: identifier,
										Pattern:    annotation,
										Params:     parameters,
										Body:       body,
										Excerpt:    excerpt,
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
		if reader.ReadWord("pat") {
			identifier, diagnostic := reader.ReadIdentifier()
			if diagnostic == nil {
				if reader.ReadSyntax(":") {
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

func ReadParameters(reader *source.Reader) ([]ast.Parameter, error) {
	if reader.ReadSyntax("(") {
		buffer := []ast.Parameter{}
		if reader.ReadSyntax(")") {
			return []ast.Parameter{}, nil
		}
		for reader.SkipWhitespace() {
			identifier, diagnostic := reader.ReadIdentifier()
			if diagnostic == nil {
				annotation, diagnostic := func() (pat.Pattern, error) {
					if reader.ReadSyntax(":") {
						return pat.ReadPattern(reader, pat.PrecAll)
					}
					return pat.StdAny, nil
				}()
				if diagnostic == nil {
					buffer = append(buffer, ast.Parameter{
						Name:    identifier,
						Pattern: annotation,
					})
					if reader.ReadSyntax(",") {
						continue
					}
					if reader.ReadSyntax(")") {
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
	return []ast.Parameter{}, nil
}

// Read a single literal from the given Reader.
func ReadLiteral(reader *source.Reader) (ast.Expression, error) {
	if reader.SkipWhitespace() {
		position := reader.Position()
		if reader.ReadOnly("|") {
			var children []ast.Expression
			var buffer strings.Builder
			for reader.CanRead() {
				if reader.ReadOnly("|") {
					if len(children) == 0 || buffer.Len() > 0 {
						children = append(children, ast.Literal{
							Literal: buffer.String(),
							Excerpt: reader.Excerpt(position),
						})
					}
					return ast.Snippet{
						Children: children,
						Excerpt:  children[0].Origin().Expand(reader.Position()),
					}, nil
				}
				if reader.ReadOnly("<") {
					if buffer.Len() > 0 {
						children = append(children, ast.Literal{
							Literal: buffer.String(),
							Excerpt: reader.Excerpt(position),
						})
					}
					expression, err := ReadExpression(reader, All, true)
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
