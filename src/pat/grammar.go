package pat

import (
	"circumscribe/env"
	"circumscribe/source"
	"slices"
	"strings"
)

type Grammar struct {
	Sequence []Segment
}

func (self *Grammar) Compile(environment env.Environment[Pattern]) error {
	for n := range self.Sequence {
		err := self.Sequence[n].Compile(environment)
		if err == nil {
			continue
		}
		return err
	}
	return nil
}

// Match the given language against the grammar.
// ---
func (self *Grammar) Match(language string) map[string]string {
	return matchSequence(language, self.Sequence, env.Environment[string]{
		Root:    nil,
		Parent:  nil,
		Entries: env.Layer[string]{},
	})
}

// Convert the grammar to a corresponding
// pattern.
// ---
// Since a grammar expression essentially represents
// a sequence of patterns, one may compute a product
// pattern of all segments:
//
// |Hello, <name: any>!| -> |Hello,| * any * |!|
//
// When matching against a specific value, it will
// correspond to the respective pattern if the
// match succeeded.
func (self *Grammar) Pattern() Pattern {
	if len(self.Sequence) > 0 {
		buffer := self.Sequence[0].pattern
		for _, segment := range self.Sequence[1:] {
			buffer = Product(buffer, segment.pattern)
		}
		return buffer
	}
	return StdE
}

func (self *Grammar) Bindings() []struct {
	Name    string
	Pattern Pattern
} {
	var buffer []struct {
		Name    string
		Pattern Pattern
	}
	for _, segment := range self.Sequence {
		if segment.name == nil {
			continue
		}
		buffer = append(buffer, struct {
			Name    string
			Pattern Pattern
		}{
			Name:    *segment.name,
			Pattern: segment.pattern,
		})
	}
	return buffer
}

func matchSequence(
	language string,
	sequence []Segment,
	environment env.Environment[string],
) map[string]string {
	if len(sequence) > 0 {
		segment := sequence[0]
		candidates := segment.machine.PartiallySatisfies(language)
		slices.SortFunc(candidates, func(a, b string) int {
			return len(a) - len(b)
		})
		for _, candidate := range candidates {
			match := language[:len(language)-len(candidate)]
			if environment := matchSequence(candidate, sequence[1:], env.Environment[string]{
				Root:   environment.Root,
				Parent: &environment,
				Entries: func() env.Layer[string] {
					if segment.name != nil {
						return env.Layer[string]{
							*segment.name: match,
						}
					}
					return env.Layer[string]{}
				}(),
			}); environment != nil {
				return environment
			}
			continue
		}
		return nil
	}
	if len(language) == 0 {
		return environment.Flatten()
	}
	return nil
}

type Segment struct {
	name    *string
	machine *nFA
	pattern Pattern
}

func (self *Segment) Compile(environment env.Environment[Pattern]) error {
	if self.machine == nil {
		machine, err := self.pattern.Compile(environment, env.Branch[Pattern]{})
		if err == nil {
			self.machine = machine
			return nil
		}
		return err
	}
	return nil
}

func ReadGrammar(reader *source.Reader) (Grammar, error) {
	if reader.ReadSyntax("|") {
		var sequence []Segment
		var buffer strings.Builder
		for reader.CanRead() {
			if reader.ReadOnly("<") {
				if buffer.Len() > 0 {
					sequence = append(sequence, Segment{
						name:    nil,
						machine: nil,
						pattern: Literal(buffer.String()),
					})
					buffer.Reset()
				}
				name, err := func() (*string, error) {
					stamp := reader.Position()
					identifier, err := reader.ReadIdentifier()
					if err == nil {
						if reader.ReadSyntax(":") {
							return &identifier, nil
						}
						reader.Backtrack(stamp)
						return nil, err
					}
					reader.Backtrack(stamp)
					return nil, nil
				}()
				if err == nil {
					pattern, err := ReadPattern(reader, PrecAll)
					if err == nil {
						sequence = append(sequence, Segment{
							name:    name,
							machine: nil,
							pattern: pattern,
						})
						if reader.ReadSyntax(">") {
							continue
						}
						return Grammar{}, source.Diagnose("Expected '>'", reader.PointExcerpt())
					}
					return Grammar{}, err
				}
				return Grammar{}, err
			}
			if reader.ReadOnly("|") {
				if buffer.Len() > 0 {
					sequence = append(sequence, Segment{
						name:    nil,
						machine: nil,
						pattern: Literal(buffer.String()),
					})
					return Grammar{
						Sequence: sequence,
					}, nil
				}
				return Grammar{
					Sequence: sequence,
				}, nil
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
					return Grammar{}, err
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
				return Grammar{}, source.Diagnose("Unrecognized escape sequence", reader.PointExcerpt())
			}
			buffer.WriteRune(reader.Read())
		}
		return Grammar{}, source.Diagnose("Expected '|'", reader.PointExcerpt())
	}
	pattern, err := ReadPattern(reader, PrecAll)
	if err == nil {
		return Grammar{
			Sequence: []Segment{{
				name:    nil,
				machine: nil,
				pattern: pattern,
			}},
		}, nil
	}
	return Grammar{}, err
}
