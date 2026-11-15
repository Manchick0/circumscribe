package main

import (
	"circumscribe/env"
	"circumscribe/expr"
	"circumscribe/parser"
	"circumscribe/pat"
	"circumscribe/source"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const USAGE = `usage: circumscribe <command> <args> [...flags]

A general-purpose macro substitution tool.

substitute <path>      Substitute either the provided file or all circumscribed
                       files within the provided directory, recursively. A file
                       is substituted to the stdout, which may be altered with
                       the [-o <path>] flag.

                       [-o <path>]         Substitute to the provided directory instead,
                       [--output <path>]   mirroring the structure of the source one.

                       [-x <glob>]         Ignore all files that satisfy the given
                       [--exclude <glob>]  glob pattern.

                       [-c <path>]         Read definitions from the given path
                       [--context <path>]  instead of the usual '.circumscribe'.

                       [-m | --mirror]     Copy the rest of the directory, unchanged.

evaluate <expression>  Evaluate the given expression, with regard to the definitions 
                       in the appropriate context file.

                       [-std]              Only provide the standard functions, without 
                       [--standard]        loading any definitions from the context file,
                                           hence not requiring its existence.

                       [-c <path>]         Read definitions from the given path
                       [--context <path>]  instead of the usual '.circumscribe'.

init                   Initialize a '.circumscribe' file in the current working
                       directory, optionally with the chosen template:

                       default  The default '.circumscribe' containing a greeting.

                       natural  A '.circumscribe' containing an example of
					            a peano naturals, implemented in circumscribe.

                       empty    An empty '.circumscribe'.

                       [-c <path>]         Write to the provided <path> instead of
                       [--context <path>]  instead of the usual '.circumscribe'.

                       [-f | --force]      Overwrite the existing '.circumscribe', if
                                           one is present.

version                Display the version information of the circumscribe binary.

help                   Display this help message.
`
const DEFAULT = `def greeting: greet(|Marie|)

def greet(name):
	|Hello, <name>!|
`

// A RegExp representing a *circumscribed*
// basename in form of <name>(.extension).
var Circumscribed = regexp.MustCompile(`^<(.*)>(?:\.(.*))?$`)

func main() {
	options, flags, err := parseArgs(os.Args[1:])
	if err == nil {
		if len(options) > 0 {
			if options[0] == "substitute" {
				if len(options) > 1 {
					path, err := func() (string, error) {
						if filepath.IsAbs(options[1]) {
							return filepath.Clean(options[1]), nil
						}
						cwd, err := os.Getwd()
						if err == nil {
							return filepath.Join(cwd, options[1]), nil
						}
						return cwd, err
					}()
					if err == nil {
						envm, envp, err := initEnvironment(flags)
						if err == nil {
							stat, err := os.Stat(path)
							if err == nil {
								if stat.IsDir() {
									err := substituteDir(path, envm, envp, flags)
									if err == nil {
										os.Exit(0)
									}
									fmt.Fprintln(os.Stderr, err)
									os.Exit(1)
								}
								err := substituteFile(path, envm, envp, flags)
								if err == nil {
									os.Exit(0)
								}
								fmt.Fprintln(os.Stderr, err)
								os.Exit(1)
							}
							fmt.Fprintln(os.Stderr, err)
							os.Exit(1)
						}
						fmt.Fprintln(os.Stderr, err)
						os.Exit(1)
					}
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				}
			}
			if options[0] == "evaluate" {
				envm, envp, diagnostic := initEnvironment(flags)
				if diagnostic == nil {
					reader := source.New("evaluate", options[1])
					expression, diagnostic := parser.ReadExpression(&reader, parser.All)
					if diagnostic == nil {
						folded, _, diagnostic := expr.Fold(expression, envm, envp, env.Branch[expr.Macro]{})
						if diagnostic == nil {
							snippet, diagnostic := expr.Evaluate(folded, envm, envp)
							if diagnostic == nil {
								fmt.Println(snippet)
								os.Exit(0)
							}
							fmt.Fprintln(os.Stderr, diagnostic)
							os.Exit(1)
						}
						fmt.Fprintln(os.Stderr, diagnostic)
						os.Exit(1)
					}
					fmt.Fprintln(os.Stderr, diagnostic)
					os.Exit(1)
				}
				fmt.Fprintln(os.Stderr, diagnostic)
				os.Exit(1)
			}
			if options[0] == "infer" {
				envm, envp, diagnostic := initEnvironment(flags)
				if diagnostic == nil {
					reader := source.New("evaluate", options[1])
					expression, diagnostic := parser.ReadExpression(&reader, parser.All)
					if diagnostic == nil {
						_, pattern, diagnostic := expr.Fold(expression, envm, envp, env.Branch[expr.Macro]{})
						if diagnostic == nil {
							fmt.Println(pattern)
							os.Exit(0)
						}
						fmt.Fprintln(os.Stderr, diagnostic)
						os.Exit(1)
					}
					fmt.Fprintln(os.Stderr, diagnostic)
					os.Exit(1)
				}
				fmt.Fprintln(os.Stderr, diagnostic)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "Unknown option '%s'\n", options[0])
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, USAGE)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// Substitute a single file.
// ---
// This procedure is taken whenever 'circumscribe substitute'
// is called on a single file:
//
// circumscribe substitute foo.txt
//
// If neither [-o <path>] nor [--output <path>] were provided
// as CLI flags, the substitution is printed to the stdout.
// Otherwise, it is written to the provided path.
func substituteFile(path string, envm env.Environment[expr.Macro], envp env.Environment[pat.Pattern], flags Flags) error {
	cwd, err := os.Getwd()
	if err == nil {
		// The name used for the reader
		// is the relative path from cwd
		// to the file.
		name, err := filepath.Rel(cwd, path)
		if err == nil {
			content, err := os.ReadFile(path)
			if err == nil {
				s10n, err := substitute(name, string(content), envm, envp)
				if err == nil {
					// If neither [-o <path>] nor [--output <path>]
					// was provided, write to the stdout by default.
					if flags.output == nil {
						fmt.Print(s10n)
						return nil
					}
					// Otherwise, write to the provided path.
					return os.WriteFile(*flags.output, []byte(s10n), 0644)
				}
				return err
			}
			return err
		}
		return err
	}
	return err
}

func substituteDir(path string, envm env.Environment[expr.Macro], envp env.Environment[pat.Pattern], flags Flags) error {
	return filepath.WalkDir(path, func(child string, dirent fs.DirEntry, err error) error {
		mode := dirent.Type()
		if mode.IsRegular() {
			parent := filepath.Dir(child)
			entry := filepath.Base(child)
			rel, err := filepath.Rel(path, child)
			if err == nil {
				content, err := os.ReadFile(child)
				if err == nil {
					// We should output elsewhere
					if flags.output != nil {
						submatch := Circumscribed.FindStringSubmatch(entry)
						sibling := filepath.Join(*flags.output, filepath.Dir(rel))
						if submatch != nil {
							err := os.MkdirAll(sibling, 0755)
							if err == nil {
								s10n, err := substitute(rel, string(content), envm, envp)
								if err == nil {
									normalized := fmt.Sprintf("%s.%s", submatch[1], submatch[2])
									return os.WriteFile(filepath.Join(sibling, normalized), []byte(s10n), 0644)
								}
								return err
							}
							return err
						}
						// Otherwise, if 'mirror' is set,
						// copy the file unmodified.
						if flags.mirror {
							err := os.MkdirAll(sibling, 0755)
							if err == nil {
								return os.WriteFile(filepath.Join(sibling, entry), content, 0644)
							}
							return err
						}
						return nil
					}
					// Otherwise, substitute in-place.
					// ---
					// Since the 'mirror' flag has no effect
					// in this case, we only proceed if the
					// entry is circumscribed.
					submatch := Circumscribed.FindStringSubmatch(entry)
					if submatch != nil {
						s10n, err := substitute(rel, string(content), envm, envp)
						if err == nil {
							normalized := fmt.Sprintf("%s.%s", submatch[1], submatch[2])
							return os.WriteFile(filepath.Join(parent, normalized), []byte(s10n), 0644)
						}
						return err
					}
					return nil
				}
				return err
			}
			return err
		}
		return nil
	})
}

// Subsitute the given content in the given environment.
// ---
// The 'name' parameter is the name under which a
// 'source.Reader' is created, and should represent
// the name of the file being substituted.
//
// For better user experience, it is desired to always
// provide a path _relative to the cwd_ as the name
// argument.
func substitute(name string, content string, envm env.Environment[expr.Macro], envp env.Environment[pat.Pattern]) (string, error) {
	var reader source.Reader = source.New(name, content)
	var buffer strings.Builder
	for reader.CanRead() {
		if reader.ReadOnly("<") {
			body, diagnostic := parser.ReadExpression(&reader, parser.All)
			if diagnostic == nil {
				folded, _, diagnostic := expr.Fold(body, envm, envp, env.Branch[expr.Macro]{})
				if diagnostic == nil {
					snippet, diagnostic := expr.Evaluate(folded, envm, envp)
					if diagnostic == nil {
						buffer.WriteString(snippet)
						if reader.ReadMeaningful(">") {
							continue
						}
						return buffer.String(), source.Diagnose("Expected '>'", reader.PointExcerpt())
					}
					return buffer.String(), diagnostic
				}
				return buffer.String(), diagnostic
			}
			return buffer.String(), diagnostic
		}
		buffer.WriteRune(reader.Read())
	}
	return buffer.String(), nil
}

// Initialize the circumscribe environment
// from the appropriate context.
// ---
func initEnvironment(flags Flags) (env.Environment[expr.Macro], env.Environment[pat.Pattern], error) {
	cwd, err := os.Getwd()
	if err == nil {
		context, err := flags.ResolveContext()
		if err == nil {
			envm, envp, diagnostic := func() (env.Layer[expr.Macro], env.Layer[pat.Pattern], error) {
				envm := env.Layer[expr.Macro]{}
				envp := env.Layer[pat.Pattern]{}
				for _, ctx := range context {
					content, err := os.ReadFile(ctx)
					if err == nil {
						name, err := filepath.Rel(cwd, ctx)
						if err == nil {
							reader := source.New(name, string(content))
							for {
								entry, err := parser.ReadDefinition(&reader)
								if err == nil {
									if entry == nil {
										break
									}
									if entry, ok := entry.(parser.Macro); ok {
										if _, ok := envm[entry.Identifier]; ok {
											return nil, nil, source.Diagnose(fmt.Sprintf("Duplicate definition of '%s'", entry.Identifier), entry.Excerpt)
										}
										envm[entry.Identifier] = entry.Macro
										continue
									}
									if entry, ok := entry.(parser.Pattern); ok {
										if _, ok := envp[entry.Identifier]; ok {
											return nil, nil, source.Diagnose(fmt.Sprintf("Duplicate definition of '%s'", entry.Identifier), entry.Excerpt)
										}
										envp[entry.Identifier] = entry.Pattern
										continue
									}
									panic("entry, ok := entry.(parser.Macro); ok || entry, ok := entry.(parser.Pattern); ok")
								}
								return nil, nil, err
							}
							continue
						}
						return nil, nil, err
					}
					return nil, nil, err
				}
				return envm, envp, nil
			}()
			if diagnostic == nil {
				stdm, stdp := stdlib()
				envm, envp := env.Environment[expr.Macro]{
					Root:    nil,
					Parent:  &stdm,
					Entries: envm,
				}, env.Environment[pat.Pattern]{
					Root:    nil,
					Parent:  &stdp,
					Entries: envp,
				}
				envm.Root, envp.Root = &envm, &envp
				for _, entry := range envp.Entries {
					_, err := entry.Compile(envp, env.Branch[pat.Pattern]{})
					if err == nil {
						continue
					}
					return env.Environment[expr.Macro]{}, env.Environment[pat.Pattern]{}, err
				}
				for _, entry := range envm.Entries {
					if macro, ok := entry.(*expr.Source); ok {
						diagnostic := macro.Fold(envm, envp, env.Branch[expr.Macro]{})
						if diagnostic == nil {
							continue
						}
						return env.Environment[expr.Macro]{}, env.Environment[pat.Pattern]{}, diagnostic
					}
					panic("Attempted to fold a non-source macro")
				}
				return envm, envp, nil
			}
			return env.Environment[expr.Macro]{}, env.Environment[pat.Pattern]{}, diagnostic
		}
		return env.Environment[expr.Macro]{}, env.Environment[pat.Pattern]{}, err
	}
	return env.Environment[expr.Macro]{}, env.Environment[pat.Pattern]{}, err
}

func stdlib() (env.Environment[expr.Macro], env.Environment[pat.Pattern]) {
	return env.Environment[expr.Macro]{
			Parent: nil,
			Root:   nil,
			Entries: env.Layer[expr.Macro]{
				"meaningful": expr.StdMeaningful,
				"reverse":    expr.StdReverse,
				"length":     expr.StdLength,
				"head":       expr.StdHead,
				"tail":       expr.StdTail,
				"read":       expr.StdRead,
				"chr":        expr.StdChr,
				"ord":        expr.StdOrd,
			},
		}, env.Environment[pat.Pattern]{
			Parent: nil,
			Root:   nil,
			Entries: env.Layer[pat.Pattern]{
				"w":       pat.StdW,
				"e":       pat.StdE,
				"s":       pat.StdS,
				"any":     pat.StdAny,
				"boolean": pat.StdBoolean,
				"natural": pat.StdNatural,
			},
		}
}

// The CLI flags passed to the circumscribe
// executable. These are different from
// options: those are collected separately.
type Flags struct {
	output  *string  // The output path, or nil if none was given         (substitute)
	context []string // The content file(s), empty if none were given     (evaluate, substitute, init)
	exclude *string  // The exclude pattern, or nil if none was given     (substitute)
	mirror  bool     // Whether to mirror the rest of the directory       (substitute)
	force   bool     // Whether to create .circumscribe if one is present (init)
}

// Compute the context file, containing
// the macro definitions.
// ---
// If a context was specified through [--context <path>]
// or [-c <path>], it is used as long as the file exists
// and is a regular file.
//
// If no context was specified, '.circumscribe' is resolved
// against the current working directory and used as the
// context instead.
//
// If no context was provided, yet '.circumscribe' does
// not exist, a different error is used in favor of the
// usual 'fs.ErrNotExist', since the existence of the
// file was never promised in the first place.
func (self Flags) ResolveContext() ([]string, error) {
	if len(self.context) == 0 {
		cwd, err := os.Getwd()
		if err == nil {
			path := filepath.Join(cwd, ".circumscribe")
			stat, err := os.Stat(path)
			if err == nil {
				mode := stat.Mode()
				if mode.IsRegular() {
					return []string{path}, nil
				}
				return nil, errors.New("circumscribe: '.circumscribe' in the current working directory isn't a regular file")
			}
			if errors.Is(err, fs.ErrNotExist) {
				return nil, errors.New("circumscribe: Could not locate '.circumscribe' in the current working directory. Perhaps 'circumscribe init'?")
			}
			return nil, err
		}
		return nil, err
	}
	for _, entry := range self.context {
		stat, err := os.Stat(entry)
		if err == nil {
			mode := stat.Mode()
			if mode.IsRegular() {
				continue
			}
			return nil, fmt.Errorf("circumscribe: '%s', provided as context, must be a regular file", entry)
		}
		return nil, err
	}
	return self.context, nil
}

func parseArgs(args []string) ([]string, Flags, error) {
	options := []string{}
	flags := Flags{}
	for position := 0; position < len(args); position++ {
		argument := args[position]
		switch argument {
		case "--output", "-o":
			position++
			if position < len(args) {
				if filepath.IsAbs(args[position]) {
					output := filepath.Clean(args[position])
					flags.output = &output
					continue
				}
				cwd, err := os.Getwd()
				if err == nil {
					output := filepath.Join(cwd, args[position])
					flags.output = &output
					continue
				}
				return options, flags, err
			}
			return options, flags, fmt.Errorf("%s requires a positional argument", argument)
		case "--context", "-c":
			position++
			if position < len(args) {
				if filepath.IsAbs(args[position]) {
					context := filepath.Clean(args[position])
					flags.context = append(flags.context, context)
					continue
				}
				cwd, err := os.Getwd()
				if err == nil {
					context := filepath.Join(cwd, args[position])
					flags.context = append(flags.context, context)
					continue
				}
				return options, flags, err
			}
			return options, flags, fmt.Errorf("%s requires a positional argument", argument)
		case "--exclude", "-x":
			position++
			if position < len(args) {
				flags.exclude = &args[position]
				continue
			}
			return options, flags, fmt.Errorf("%s requires a positional argument", argument)
		case "--mirror", "-m":
			flags.mirror = true
			continue
		case "--force", "-f":
			flags.force = true
			continue
		default:
			options = append(options, argument)
			continue
		}
	}
	return options, flags, nil
}
