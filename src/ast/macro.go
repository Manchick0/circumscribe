package ast

import (
	"circumscribe/env"
	"circumscribe/pat"
	"circumscribe/source"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

// A macro
// ---
// A macro is a single reusable piece of code
// that may produce a snippet, given some
// arguments.
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
	Evaluate(env.Environment[Macro], env.Environment[pat.Pattern], []string, int) (string, error)
	// Infer the pattern denoting the return type
	// of the macro.
	// ---
	// If the pattern is known, it is returned directly.
	// Otherwise, the body of the function is used in order
	// to infer the return pattern.
	Infer(env.Environment[Macro], env.Environment[pat.Pattern], env.Branch[Macro]) (pat.Pattern, error)
	IsPure(env.Environment[Macro], env.Environment[pat.Pattern], env.Branch[Macro]) (bool, error)
	Parameters() []Parameter
	Name() string
}

var (
	// trim()
	// ---
	// Trim the leading and trailing
	// whitespace from the given source.
	StdTrim = Native{
		Identifier: "trim",
		Params:     []Parameter{{Name: "source", Pattern: pat.StdAny}},
		Body: func(args []string) (string, error) {
			return strings.TrimSpace(args[0]), nil
		},
		Pattern: pat.StdAny,
		Pure:    true,
	}
	// length()
	// ---
	// Compute the length of the given snippet.
	StdLength = Native{
		Identifier: "length",
		Params:     []Parameter{{Name: "source", Pattern: pat.StdAny}},
		Body: func(args []string) (string, error) {
			return strconv.Itoa(len(args[0])), nil
		},
		Pattern: pat.StdNatural,
		Pure:    true,
	}
	// tail()
	// ---
	// Compute the tail of the given snippet.
	StdTail = Native{
		Identifier: "tail",
		Params:     []Parameter{{Name: "source", Pattern: pat.StdAny}},
		Body: func(args []string) (string, error) {
			if len(args) > 0 {
				_, len := utf8.DecodeRuneInString(args[0])
				return args[0][len:], nil
			}
			return args[0], nil
		},
		Pattern: pat.StdAny,
		Pure:    true,
	}
	// head()
	// ---
	// Compute the head of the given snippet.
	StdHead = Native{
		Identifier: "head",
		Params:     []Parameter{{Name: "source", Pattern: pat.StdAny}},
		Body: func(args []string) (string, error) {
			if len(args) > 0 {
				_, len := utf8.DecodeRuneInString(args[0])
				return args[0][:len], nil
			}
			return args[0], nil
		},
		Pattern: pat.StdW,
		Pure:    true,
	}
	// env()
	// ---
	// Return the provided environment variable.
	StdEnv = Native{
		Identifier: "env",
		Params:     []Parameter{{Name: "name", Pattern: pat.StdAny}},
		Body: func(args []string) (string, error) {
			return os.Getenv(args[0]), nil
		},
		Pattern: pat.StdAny,
		Pure:    false,
	}
	// read()
	// ---
	// Read the contents of the provided path,
	// resolved against the current working
	// directory, unless absolute.
	StdRead = Native{
		Identifier: "read",
		Params:     []Parameter{{Name: "path", Pattern: pat.StdAny}},
		Body: func(args []string) (string, error) {
			path, err := func() (string, error) {
				cwd, err := os.Getwd()
				if err == nil {
					if filepath.IsAbs(args[0]) {
						return filepath.Clean(args[0]), nil
					}
					return filepath.Join(cwd, args[0]), nil
				}
				return "", err
			}()
			if err == nil {
				content, err := os.ReadFile(path)
				return string(content), err
			}
			return "", err
		},
		Pattern: pat.StdAny,
		Pure:    false,
	}
	// ord()
	// ---
	// Compute the ordinal of the given character.
	StdOrd = Native{
		Identifier: "ord",
		Params:     []Parameter{{Name: "w", Pattern: pat.StdW}},
		Body: func(args []string) (string, error) {
			r, _ := utf8.DecodeRuneInString(args[0])
			return strconv.Itoa(int(r)), nil
		},
		Pattern: pat.StdNatural,
		Pure:    true,
	}
	// chr()
	// ---
	// Compute the character for the given ordinal.
	StdChr = Native{
		Identifier: "chr",
		Params:     []Parameter{{Name: "ord", Pattern: pat.StdNatural}},
		Body: func(args []string) (string, error) {
			ord, err := strconv.ParseInt(args[0], 10, 32)
			if err == nil && ord <= 0x10FFFF {
				return string(rune(ord)), nil
			}
			return "", fmt.Errorf("Ordinal '%s' is not within the Unicode range", args[0])
		},
		Pattern: pat.StdW,
		Pure:    true,
	}
	// reverse()
	// ---
	// Reverse the given snippet.
	StdReverse = Native{
		Identifier: "reverse",
		Params:     []Parameter{{Name: "src", Pattern: pat.StdAny}},
		Body: func(args []string) (string, error) {
			var buffer strings.Builder
			var source = args[0]
			for len(source) > 0 {
				c, l := utf8.DecodeLastRuneInString(source)
				source = source[:len(source)-l]
				buffer.WriteRune(c)
			}
			return buffer.String(), nil
		},
		Pattern: pat.StdAny,
		Pure:    true,
	}
	// contains()
	// ---
	// Determine whether the given
	// source contains the given
	// snippet.
	StdContains = Native{
		Identifier: "contains",
		Params:     []Parameter{{Name: "source", Pattern: pat.StdAny}, {Name: "snippet", Pattern: pat.StdAny}},
		Body: func(args []string) (string, error) {
			if strings.Contains(args[0], args[1]) {
				return "true", nil
			}
			return "false", nil
		},
		Pattern: pat.StdBoolean,
		Pure:    true,
	}
	// uppercase()
	// ---
	// Convert the given source to
	// the uppercase equivalent.
	StdUppercase = Native{
		Identifier: "uppercase",
		Params:     []Parameter{{Name: "source", Pattern: pat.StdAny}},
		Body: func(args []string) (string, error) {
			return strings.ToUpper(args[0]), nil
		},
		Pattern: pat.StdAny,
		Pure:    true,
	}
	// lowercase()
	// ---
	// Convert the given source to
	// the lowercase equivalent.
	StdLowercase = Native{
		Identifier: "lowercae",
		Params:     []Parameter{{Name: "source", Pattern: pat.StdAny}},
		Body: func(args []string) (string, error) {
			return strings.ToLower(args[0]), nil
		},
		Pattern: pat.StdAny,
		Pure:    true,
	}
)

type Parameter struct {
	Name    string
	Pattern pat.Pattern
}

type Source struct {
	Identifier string
	Body       Expression
	Params     []Parameter
	Pattern    pat.Pattern
	Pure       *bool
	Excerpt    source.Excerpt
}

func (self *Source) Evaluate(envm env.Environment[Macro], envp env.Environment[pat.Pattern], arguments []string, recursion int) (string, error) {
	if recursion < rlimit {
		buffer := env.Layer[Macro]{}
		for n, parameter := range self.Params {
			buffer[parameter.Name] = Constant{
				identifier: parameter.Name,
				constant:   arguments[n],
			}
		}
		return Evaluate(self.Body, env.Environment[Macro]{
			Root:    envm.Root,
			Parent:  envm.Root,
			Entries: buffer,
		}, env.Environment[pat.Pattern]{
			Root:    envp.Root,
			Parent:  envp.Root,
			Entries: env.Layer[pat.Pattern]{},
		}, recursion+1)
	}
	return "", fmt.Errorf("Encountered an extensively deep recursion chain (>=512)")
}

func (self *Source) Parameters() []Parameter {
	return self.Params
}

func (self *Source) Infer(envm env.Environment[Macro], envp env.Environment[pat.Pattern], branch env.Branch[Macro]) (pat.Pattern, error) {
	if self.Pattern == nil {
		if !branch.Contains(self) {
			diagnostic := self.Compile(envm, envp, branch)
			if diagnostic == nil {
				return self.Infer(envm, envp, branch)
			}
			return nil, diagnostic
		}
		return nil, source.Diagnose("A recursive function must have its pattern annotated", self.Excerpt)
	}
	return self.Pattern, nil
}

func (self *Source) Compile(envm env.Environment[Macro], envp env.Environment[pat.Pattern], branch env.Branch[Macro]) error {
	pattern, diagnostic := Compile(self.Body, env.Environment[Macro]{
		Root:   envm.Root,
		Parent: envm.Root,
		Entries: func() env.Layer[Macro] {
			buffer := env.Layer[Macro]{}
			for _, parameter := range self.Params {
				buffer[parameter.Name] = Placeholder{
					identifier: parameter.Name,
					pattern:    parameter.Pattern,
				}
			}
			return buffer
		}(),
	}, env.Environment[pat.Pattern]{
		Root:    envp.Root,
		Parent:  envp.Root,
		Entries: env.Layer[pat.Pattern]{},
	}, env.Branch[Macro]{
		Parent: &branch,
		Entry:  self,
	})
	if diagnostic == nil {
		if self.Pattern != nil {
			left, err := pattern.Compile(envp, env.Branch[pat.Pattern]{})
			if err == nil {
				right, err := self.Pattern.Compile(envp, env.Branch[pat.Pattern]{})
				if err == nil {
					complement := right.Complement()
					intersection := left.Intersection(complement)
					if intersection.IsMeaningful() {
						return source.Diagnostic{
							Message: fmt.Sprintf("Pattern '%s' is not assignable to '%s'", left, right),
							Excerpt: self.Body.Origin(),
						}
					}
					return nil
				}
				return err
			}
			return err
		}
		self.Pattern = pattern
		return nil
	}
	return diagnostic
}

func (self *Source) IsPure(envm env.Environment[Macro], envp env.Environment[pat.Pattern], branch env.Branch[Macro]) (bool, error) {
	if self.Pure == nil {
		if branch.Contains(self) {
			// "pure until proven otherwise"
			return true, nil
		}
		pure, diagnostic := IsPure(self.Body, env.Environment[Macro]{
			Root:   envm.Root,
			Parent: envm.Root,
			Entries: func() env.Layer[Macro] {
				buffer := env.Layer[Macro]{}
				for _, parameter := range self.Params {
					buffer[parameter.Name] = Placeholder{
						identifier: parameter.Name,
						pattern:    parameter.Pattern,
					}
				}
				return buffer
			}(),
		}, env.Environment[pat.Pattern]{
			Root:    envp.Root,
			Parent:  envp.Parent,
			Entries: env.Layer[pat.Pattern]{},
		}, env.Branch[Macro]{
			Parent: &branch,
			Entry:  self,
		})
		if diagnostic == nil {
			self.Pure = &pure
			return pure, nil
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
	Pattern    pat.Pattern
	Body       func([]string) (string, error)
	Pure       bool
}

func (self Native) Evaluate(_ env.Environment[Macro], _ env.Environment[pat.Pattern], arguments []string, _ int) (string, error) {
	return self.Body(arguments)
}

func (self Native) Infer(env.Environment[Macro], env.Environment[pat.Pattern], env.Branch[Macro]) (pat.Pattern, error) {
	if self.Pattern != nil {
		return self.Pattern, nil
	}
	panic("A native function must have its pattern annotated")
}

func (self Native) IsPure(env.Environment[Macro], env.Environment[pat.Pattern], env.Branch[Macro]) (bool, error) {
	return self.Pure, nil
}

func (self Native) Name() string {
	return self.Identifier
}

func (self Native) Parameters() []Parameter {
	return self.Params
}

type Constant struct {
	identifier string
	constant   string
}

func (self Constant) Evaluate(env.Environment[Macro], env.Environment[pat.Pattern], []string, int) (string, error) {
	return self.constant, nil
}

func (self Constant) Infer(env.Environment[Macro], env.Environment[pat.Pattern], env.Branch[Macro]) (pat.Pattern, error) {
	return pat.Literal(self.constant), nil
}

func (self Constant) IsPure(env.Environment[Macro], env.Environment[pat.Pattern], env.Branch[Macro]) (bool, error) {
	return true, nil
}

func (self Constant) Name() string {
	return self.identifier
}

func (self Constant) Parameters() []Parameter {
	return []Parameter{}
}

type Placeholder struct {
	identifier string
	pattern    pat.Pattern
}

func (self Placeholder) Evaluate(env.Environment[Macro], env.Environment[pat.Pattern], []string, int) (string, error) {
	panic("Placeholder.Evaluate(env.Environment[Macro], []string)")
}

func (self Placeholder) IsPure(env.Environment[Macro], env.Environment[pat.Pattern], env.Branch[Macro]) (bool, error) {
	return true, nil
}

func (self Placeholder) Infer(env.Environment[Macro], env.Environment[pat.Pattern], env.Branch[Macro]) (pat.Pattern, error) {
	return self.pattern, nil
}

func (self Placeholder) Parameters() []Parameter {
	return []Parameter{}
}

func (self Placeholder) Name() string {
	return self.identifier
}

type Refinement struct {
	pattern  pat.Pattern
	delegate Macro
}

func (self Refinement) Evaluate(env.Environment[Macro], env.Environment[pat.Pattern], []string, int) (string, error) {
	panic("Refinement.Evaluate(env.Environment[Macro], []string)")
}

func (self Refinement) IsPure(envm env.Environment[Macro], envp env.Environment[pat.Pattern], branch env.Branch[Macro]) (bool, error) {
	return self.delegate.IsPure(envm, envp, branch)
}

func (self Refinement) Infer(env.Environment[Macro], env.Environment[pat.Pattern], env.Branch[Macro]) (pat.Pattern, error) {
	return self.pattern, nil
}

func (self Refinement) Parameters() []Parameter {
	return self.delegate.Parameters()
}

func (self Refinement) Name() string {
	return self.delegate.Name()
}
