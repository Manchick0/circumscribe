package parser

import (
	"circumscribe/expression"
	"circumscribe/pattern"
	"circumscribe/source"
	"strings"
)

const (
	POSTFIX = iota
	AND
	OR
)

func ReadExpression(reader *source.Reader) (expression.Expression, error) {
	if reader.SkipWhitespace() {
		position := reader.Position()
		if reader.ReadOnly("if") {
			condition, diagnostic := ReadExpression(reader)
			if diagnostic == nil {
				if reader.ReadMeaningful(":") {
					body, diagnostic := ReadExpression(reader)
					if diagnostic == nil {
						if reader.ReadMeaningful("else") {
							otherwise, diagnostic := ReadExpression(reader)
							if diagnostic == nil {
								return expression.Condition{
									Condition: condition,
									Body:      body,
									Otherwise: otherwise,
									Excerpt:   otherwise.Origin().Expand(position),
								}, nil
							}
							return nil, diagnostic
						}
						return nil, source.Diagnostic{
							Message: "Expected 'else'",
							Excerpt: reader.PointExcerpt(),
						}
					}
					return nil, diagnostic
				}
				return nil, source.Diagnostic{
					Message: "Expected ':'",
					Excerpt: reader.PointExcerpt(),
				}
			}
			return nil, diagnostic
		}
		if reader.IsAt("|") {
			return ReadLiteral(reader)
		}
		identifier, diagnostic := ReadIdentifier(reader)
		if diagnostic == nil {
			if reader.ReadMeaningful("(") {
				var buffer []expression.Expression
				if reader.SkipWhitespace() && reader.ReadOnly(")") {
					return expression.Application{
						Identifier: identifier,
						Arguments:  []expression.Expression{},
						Excerpt:    reader.Excerpt(position),
						Bare:       false,
					}, nil
				}
				for reader.SkipWhitespace() {
					argument, diagnostic := ReadExpression(reader)
					if diagnostic == nil {
						buffer = append(buffer, argument)
						if reader.ReadMeaningful(",") {
							continue
						}
						if reader.ReadMeaningful(")") {
							return expression.Application{
								Identifier: identifier,
								Arguments:  buffer,
								Excerpt:    reader.Excerpt(position),
								Bare:       false,
							}, nil
						}
						return nil, source.Diagnostic{
							Message: "Expected ')'",
							Excerpt: reader.PointExcerpt(),
						}
					}
					return nil, diagnostic
				}
				return nil, source.Diagnostic{
					Message: "Expected ')'",
					Excerpt: reader.PointExcerpt(),
				}
			}
			return expression.Application{
				Identifier: identifier,
				Arguments:  []expression.Expression{},
				Excerpt:    reader.Excerpt(position),
				Bare:       true,
			}, nil
		}
		return nil, diagnostic
	}
	return nil, source.Diagnostic{
		Message: "Expected an expression",
		Excerpt: reader.PointExcerpt(),
	}
}

func ReadDefinition(reader *source.Reader) (expression.Macro, error) {
	if reader.SkipWhitespace() {
		position := reader.Position()
		if reader.ReadMeaningful("def") {
			identifier, diagnostic := ReadIdentifier(reader)
			if diagnostic == nil {
				parameters, diagnostic := ReadParameters(reader)
				if diagnostic == nil {
					annotation, diagnostic := func() (*pattern.Pattern, error) {
						if reader.ReadMeaningful("->") {
							annotation, diagnostic := pattern.ReadPattern(reader, 2)
							if diagnostic == nil {
								return &annotation, nil
							}
							return nil, diagnostic
						}
						return nil, nil
					}()
					if diagnostic == nil {
						if reader.ReadMeaningful(":") {
							body, diagnostic := ReadExpression(reader)
							if diagnostic == nil {
								return &expression.Source{
									Identifier: identifier,
									Pattern:    annotation,
									Params:     parameters,
									Body: struct {
										Source expression.Expression
										Folded expression.Expression
									}{
										Source: body,
										Folded: nil,
									},
									Excerpt: body.Origin().Expand(position),
								}, nil
							}
							return nil, diagnostic
						}
						return nil, source.Diagnostic{
							Message: "Expected ':'",
							Excerpt: reader.PointExcerpt(),
						}
					}
					return nil, diagnostic
				}
				return nil, diagnostic
			}
			return nil, diagnostic
		}
		return nil, source.Diagnostic{
			Message: "Expected 'def'",
			Excerpt: reader.PointExcerpt(),
		}
	}
	return nil, nil
}

func ReadParameters(reader *source.Reader) ([]expression.Parameter, error) {
	if reader.ReadMeaningful("(") {
		buffer := []expression.Parameter{}
		if reader.ReadMeaningful(")") {
			return []expression.Parameter{}, nil
		}
		for reader.SkipWhitespace() {
			identifier, diagnostic := ReadIdentifier(reader)
			if diagnostic == nil {
				annotation, diagnostic := func() (pattern.Pattern, error) {
					if reader.ReadMeaningful(":") {
						return pattern.ReadPattern(reader, 2)
					}
					return pattern.StdAny, nil
				}()
				if diagnostic == nil {
					buffer = append(buffer, expression.Parameter{
						Name:    identifier,
						Pattern: annotation,
					})
					if reader.ReadMeaningful(",") {
						continue
					}
					if reader.ReadMeaningful(")") {
						return buffer, nil
					}
					return nil, source.Diagnostic{
						Message: "Expected ')'",
						Excerpt: reader.PointExcerpt(),
					}
				}
				return nil, diagnostic
			}
			return nil, diagnostic
		}
		return nil, source.Diagnostic{
			Message: "Expected ')'",
			Excerpt: reader.PointExcerpt(),
		}
	}
	return []expression.Parameter{}, nil
}

// Read a single identifier from the given Reader.
func ReadIdentifier(reader *source.Reader) (string, error) {
	if reader.SkipWhitespace() {
		var buffer strings.Builder
		for reader.CanRead() {
			c := reader.Peek()
			if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '\'' || c == '_' {
				buffer.WriteRune(reader.Read())
				continue
			}
			break
		}
		if buffer.Len() > 0 {
			return buffer.String(), nil
		}
		return "", source.Diagnostic{
			Message: "Expected an identifier",
			Excerpt: reader.PointExcerpt(),
		}
	}
	return "", source.Diagnostic{
		Message: "Expected an identifier",
		Excerpt: reader.PointExcerpt(),
	}
}

// Read a single literal from the given Reader.
func ReadLiteral(reader *source.Reader) (expression.Expression, error) {
	if reader.SkipWhitespace() {
		position := reader.Position()
		if reader.ReadOnly("|") {
			var buffer strings.Builder
			for reader.CanRead() {
				if reader.ReadOnly("|") {
					return expression.Literal{
						Literal: buffer.String(),
						Excerpt: reader.Excerpt(position),
					}, nil
				}
				if reader.ReadOnly("\\") {
					if reader.ReadOnly("|") {
						buffer.WriteRune('|')
						continue
					}
					if reader.ReadOnly("<") {
						buffer.WriteRune('<')
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
					return nil, source.Diagnostic{
						Message: "Unrecognized escape sequence",
						Excerpt: reader.PointExcerpt(),
					}
				}
				buffer.WriteRune(reader.Read())
			}
			return nil, source.Diagnostic{
				Message: "Expected '|'",
				Excerpt: reader.PointExcerpt(),
			}
		}
		return nil, source.Diagnostic{
			Message: "Expected '|' here",
			Excerpt: reader.PointExcerpt(),
		}
	}
	return nil, source.Diagnostic{
		Message: "Expected '|'",
		Excerpt: reader.PointExcerpt(),
	}
}
