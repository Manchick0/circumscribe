package main

import (
	"circumscribe/ast"
	"circumscribe/env"
	"circumscribe/parser"
	"circumscribe/pat"
	"circumscribe/sgr"
	"circumscribe/source"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
)

const USAGE = `usage: circumscribe <command> <args> [...flags]

A general-purpose macro substitution tool.

substitute <path>      Substitute either the provided file or all circumscribed
                       files within the provided directory, recursively. A file
                       is substituted to the stdout, which may be altered with
                       the [-o <path>] flag.

                       [-o <path>]          Substitute to the provided directory instead,
                       [--output <path>]    mirroring the structure of the source one.

                       [-c <path>]          Read definitions from the given path
                       [--context <path>]   instead of the usual '.circumscribe'.

                       [-m | --mirror]      When substituting a directory, copy
                                            the rest of the directory unchanged.

                       [-n | --no-newline]  When substituting to the stdout, do not
                                            append a newline at the end.

evaluate <expression>  Evaluate the given expression, with regard to the definitions 
                       in the appropriate context file.

                       [-c <path>]         Read definitions from the given path
                       [--context <path>]  instead of the usual '.circumscribe'.

init                   Initialize a '.circumscribe' file in the current working
                       directory.

                       [-e | --empty]  An empty '.circumscribe'.

                       [-f | --force]  Overwrite the existing '.circumscribe', if
                                       one is present.

version                Display the version information of the circumscribe binary.

help                   Display this help message.
`
const DEFAULT = `def greeting: greet(|Marie|)

def greet(name):
	|Hello, <name>!|
`

const ROOT = "CIRCUMSCRIBE_ROOT"
const STANDARD = "standard.circumscribe"

// A RegExp representing a *circumscribed*
// basename in form of <name>(.extension).
var Circumscribed = regexp.MustCompile(`^<(.*)>(?:\.(.*))?$`)

// The version of the circumscribe executable,
// passed with the linker flags.
var version string

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
				fmt.Fprint(os.Stderr, USAGE)
				os.Exit(1)
			}
			if options[0] == "evaluate" {
				if len(options) > 1 {
					envm, envp, err := initEnvironment(flags)
					if err == nil {
						reader := source.NewReader("evaluate", options[1])
						expression, err := parser.ReadExpression(&reader, parser.All, false)
						if err == nil {
							_, diagnostic := ast.Compile(expression, envm, envp, env.Branch[ast.Macro]{})
							if diagnostic == nil {
								snippet, err := ast.Evaluate(expression, envm, envp, 0)
								if err == nil {
									fmt.Println(snippet)
									os.Exit(0)
								}
								fmt.Fprintln(os.Stderr, err)
								os.Exit(1)
							}
							fmt.Fprintln(os.Stderr, diagnostic)
							os.Exit(1)
						}
						fmt.Fprintln(os.Stderr, err)
						os.Exit(1)
					}
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				}
				fmt.Fprint(os.Stderr, USAGE)
				os.Exit(1)
			}
			if options[0] == "infer" {
				if len(options) > 1 {
					envm, envp, err := initEnvironment(flags)
					if err == nil {
						reader := source.NewReader("infer", options[1])
						position := reader.Position()
						identifier, err := reader.ReadIdentifier()
						if err == nil {
							macro, ok := envm.Traverse(identifier)
							if ok {
								pattern, err := macro.Infer(envm, envp, env.Branch[ast.Macro]{})
								if err == nil {
									pure, err := macro.IsPure(envm, envp, env.Branch[ast.Macro]{})
									if err == nil {
										var buffer strings.Builder
										fmt.Fprint(&buffer, func() string {
											if pure {
												return sgr.Escape(identifier, sgr.Pure)
											}
											return sgr.Escape(identifier, sgr.Impure)
										}())
										fmt.Fprint(&buffer, " :: ")
										fmt.Fprint(&buffer, "(")
										for n, parameter := range macro.Parameters() {
											if n > 0 {
												fmt.Fprint(&buffer, ", ")
											}
											fmt.Fprintf(&buffer, "%s", parameter.Pattern.Pretty(true, pat.PrecAll))
										}
										fmt.Fprint(&buffer, ")")
										fmt.Fprint(&buffer, " -> ")
										fmt.Fprintf(&buffer, "%s", pattern.Pretty(true, pat.PrecAll))
										fmt.Println(buffer.String())
										os.Exit(0)
									}
									fmt.Fprintln(os.Stderr, err)
									os.Exit(1)
								}
								fmt.Fprintln(os.Stderr, err)
								os.Exit(1)
							}
							fmt.Fprintln(os.Stderr, source.Diagnose(fmt.Sprintf("circumscribe: Could not resolve identifier '%s'", identifier), reader.Excerpt(position)))
							os.Exit(1)
						}
						fmt.Fprintln(os.Stderr, err)
						os.Exit(1)
					}
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				}
				fmt.Fprint(os.Stderr, USAGE)
				os.Exit(1)
			}
			if options[0] == "init" {
				cwd, err := os.Getwd()
				if err == nil {
					path := filepath.Join(cwd, ".circumscribe")
					if _, err := os.Stat(path); err != nil || flags.force {
						if flags.empty {
							os.WriteFile(path, []byte{}, 0644)
							os.Exit(0)
						}
						os.WriteFile(path, []byte(DEFAULT), 0644)
						os.Exit(0)
					}
					fmt.Fprintln(os.Stderr, "circumscribe: '.circumscribe' already exists in the current working directory. Use '-f' to ignore.")
					os.Exit(1)
				}
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			if options[0] == "version" {
				fmt.Printf("circumscribe %s\n", version)
				os.Exit(0)
			}
			if options[0] == "help" {
				fmt.Print(USAGE)
				os.Exit(0)
			}
			fmt.Fprint(os.Stderr, USAGE)
			os.Exit(1)
		}
		fmt.Fprint(os.Stderr, USAGE)
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
func substituteFile(path string, envm env.Environment[ast.Macro], envp env.Environment[pat.Pattern], flags Flags) error {
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
						if flags.nonewline {
							fmt.Print(s10n)
							return nil
						}
						fmt.Println(s10n)
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

func substituteDir(path string, envm env.Environment[ast.Macro], envp env.Environment[pat.Pattern], flags Flags) error {
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
func substitute(name string, content string, envm env.Environment[ast.Macro], envp env.Environment[pat.Pattern]) (string, error) {
	var reader source.Reader = source.NewReader(name, content)
	var buffer strings.Builder
	for reader.CanRead() {
		if reader.ReadOnly("<") {
			body, diagnostic := parser.ReadExpression(&reader, parser.All, true)
			if diagnostic == nil {
				_, diagnostic := ast.Compile(body, envm, envp, env.Branch[ast.Macro]{})
				if diagnostic == nil {
					snippet, diagnostic := ast.Evaluate(body, envm, envp, 0)
					if diagnostic == nil {
						buffer.WriteString(snippet)
						if reader.ReadSyntax(">") {
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
func initEnvironment(flags Flags) (env.Environment[ast.Macro], env.Environment[pat.Pattern], error) {
	cwd, err := os.Getwd()
	if err == nil {
		context, err := flags.ResolveContext()
		if err == nil {
			stdm, stdp, err := stdlib()
			if err == nil {
				envm, envp := env.Environment[ast.Macro]{
					Root:    nil,
					Parent:  &stdm,
					Entries: env.Layer[ast.Macro]{},
				}, env.Environment[pat.Pattern]{
					Root:    nil,
					Parent:  &stdp,
					Entries: env.Layer[pat.Pattern]{},
				}
				// Self-hosting the userspace
				envm.Root, envp.Root = &envm, &envp
				for _, ctx := range context {
					content, err := os.ReadFile(ctx)
					if err == nil {
						name, err := filepath.Rel(cwd, ctx)
						if err == nil {
							reader := source.NewReader(name, string(content))
							for {
								entry, err := parser.ReadEntry(&reader)
								if err == nil {
									if entry != nil {
										if entry, ok := entry.(parser.Macro); ok {
											if _, ok := envm.Entries[entry.Identifier]; ok {
												return env.Environment[ast.Macro]{},
													env.Environment[pat.Pattern]{},
													source.Diagnose(fmt.Sprintf("Duplicate declaration of '%s'", entry.Identifier), entry.Excerpt)
											}
											envm.Entries[entry.Identifier] = entry.Macro
											continue
										}
										if entry, ok := entry.(parser.Pattern); ok {
											if _, ok := envp.Entries[entry.Identifier]; ok {
												return env.Environment[ast.Macro]{},
													env.Environment[pat.Pattern]{},
													source.Diagnose(fmt.Sprintf("Duplicate declaration of '%s'", entry.Identifier), entry.Excerpt)
											}
											envp.Entries[entry.Identifier] = entry.Pattern
											continue
										}
										panic(reflect.TypeOf(entry))
									}
									break
								}
								return env.Environment[ast.Macro]{}, env.Environment[pat.Pattern]{}, err
							}
							continue
						}
						return env.Environment[ast.Macro]{}, env.Environment[pat.Pattern]{}, err
					}
					return env.Environment[ast.Macro]{}, env.Environment[pat.Pattern]{}, err
				}
				for _, entry := range envm.Entries {
					if entry, ok := entry.(*ast.Source); ok {
						diagnostic := entry.Compile(envm, envp, env.Branch[ast.Macro]{})
						if diagnostic != nil {
							return env.Environment[ast.Macro]{}, env.Environment[pat.Pattern]{}, diagnostic
						}
						continue
					}
					panic(reflect.TypeOf(entry))
				}
				for _, entry := range envp.Entries {
					_, err := entry.Compile(envp, env.Branch[pat.Pattern]{})
					if err != nil {
						return env.Environment[ast.Macro]{}, env.Environment[pat.Pattern]{}, err
					}
				}
				return envm, envp, nil
			}
			return env.Environment[ast.Macro]{}, env.Environment[pat.Pattern]{}, err
		}
		return env.Environment[ast.Macro]{}, env.Environment[pat.Pattern]{}, err
	}
	return env.Environment[ast.Macro]{}, env.Environment[pat.Pattern]{}, err
}

func stdlib() (env.Environment[ast.Macro], env.Environment[pat.Pattern], error) {
	root, err := root()
	if err == nil {
		content, err := os.ReadFile(filepath.Join(root, STANDARD))
		if err == nil {
			reader := source.NewReader(STANDARD, string(content))
			envm, envp := env.Environment[ast.Macro]{
				Root: nil,
				Parent: &env.Environment[ast.Macro]{
					Root:   nil,
					Parent: nil,
					// Intrinsic
					Entries: env.Layer[ast.Macro]{
						"contains'":  ast.StdContains,
						"uppercase'": ast.StdUppercase,
						"lowercase'": ast.StdLowercase,
						"reverse'":   ast.StdReverse,
						"length'":    ast.StdLength,
						"head'":      ast.StdHead,
						"tail'":      ast.StdTail,
						"trim'":      ast.StdTrim,
						"read'":      ast.StdRead,
						"env'":       ast.StdEnv,
						"chr'":       ast.StdChr,
						"ord'":       ast.StdOrd,
					},
				},
				Entries: env.Layer[ast.Macro]{},
			}, env.Environment[pat.Pattern]{
				Root:    nil,
				Parent:  nil,
				Entries: env.Layer[pat.Pattern]{},
			}
			// Self-host the standard library
			envm.Root, envp.Root = &envm, &envp
			for {
				entry, err := parser.ReadEntry(&reader)
				if err == nil {
					if entry != nil {
						if entry, ok := entry.(parser.Macro); ok {
							if _, ok := envm.Entries[entry.Identifier]; ok {
								return env.Environment[ast.Macro]{},
									env.Environment[pat.Pattern]{},
									source.Diagnose(fmt.Sprintf("Duplicate declaration of '%s'", entry.Identifier), entry.Excerpt)
							}
							envm.Entries[entry.Identifier] = entry.Macro
							continue
						}
						if entry, ok := entry.(parser.Pattern); ok {
							if _, ok := envp.Entries[entry.Identifier]; ok {
								return env.Environment[ast.Macro]{},
									env.Environment[pat.Pattern]{},
									source.Diagnose(fmt.Sprintf("Duplicate declaration of '%s'", entry.Identifier), entry.Excerpt)
							}
							envp.Entries[entry.Identifier] = entry.Pattern
							continue
						}
						panic(reflect.TypeOf(entry))
					}
					break
				}
				return env.Environment[ast.Macro]{}, env.Environment[pat.Pattern]{}, err
			}
			for _, entry := range envm.Entries {
				if entry, ok := entry.(*ast.Source); ok {
					diagnostic := entry.Compile(envm, envp, env.Branch[ast.Macro]{})
					if diagnostic != nil {
						return env.Environment[ast.Macro]{}, env.Environment[pat.Pattern]{}, diagnostic
					}
					continue
				}
				panic(reflect.TypeOf(entry))
			}
			for _, entry := range envp.Entries {
				_, err := entry.Compile(envp, env.Branch[pat.Pattern]{})
				if err != nil {
					return env.Environment[ast.Macro]{}, env.Environment[pat.Pattern]{}, err
				}
			}
			return envm, envp, nil
		}
		return env.Environment[ast.Macro]{}, env.Environment[pat.Pattern]{}, err
	}
	return env.Environment[ast.Macro]{}, env.Environment[pat.Pattern]{}, err
}

func root() (string, error) {
	if path, ok := os.LookupEnv(ROOT); ok {
		stat, err := os.Stat(path)
		if err == nil {
			if stat.IsDir() {
				return path, nil
			}
			return "", fmt.Errorf("circumscribe: The set CIRCUMSCRIBE_ROOT '%s' must be a directory.", path)
		}
		return "", err
	}
	path, err := os.Executable()
	if err == nil {
		path, err := filepath.EvalSymlinks(path)
		if err == nil {
			return filepath.Dir(path), nil
		}
		return "", err
	}
	return "", err
}

// The CLI flags passed to the circumscribe
// executable. These are different from
// options: those are collected separately.
type Flags struct {
	output    *string  // The output path, or nil if none was given         (substitute)
	context   []string // The content file(s), empty if none were given     (evaluate, substitute, init)
	exclude   *string  // The exclude pattern, or nil if none was given     (substitute)
	mirror    bool     // Whether to mirror the rest of the directory       (substitute)
	nonewline bool     // Whether to not append a newline  					(substitute)
	force     bool     // Whether to create .circumscribe if one is present (init)
	empty     bool     // Whether to create an empty .circumscribe          (init)
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
			return options, flags, fmt.Errorf("circumscribe: '%s' requires a positional argument", argument)
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
			return options, flags, fmt.Errorf("circumscribe: '%s' requires a positional argument", argument)
		case "--exclude", "-x":
			position++
			if position < len(args) {
				flags.exclude = &args[position]
				continue
			}
			return options, flags, fmt.Errorf("circumscribe: '%s' requires a positional argument", argument)
		case "--mirror", "-m":
			flags.mirror = true
			continue
		case "--force", "-f":
			flags.force = true
			continue
		case "--no-newline", "-n":
			flags.nonewline = true
			continue
		case "--empty", "-e":
			flags.empty = true
			continue
		default:
			options = append(options, argument)
			continue
		}
	}
	return options, flags, nil
}
