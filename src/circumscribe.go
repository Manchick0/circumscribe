package main

import (
	"circumscribe/expression"
	"circumscribe/parser"
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
						environment, err := initEnvironment(flags)
						if err == nil {
							stat, err := os.Stat(path)
							if err == nil {
								if stat.IsDir() {
									err := substituteDir(path, environment, flags)
									if err == nil {
										os.Exit(0)
									}
									fmt.Fprintln(os.Stderr, err)
									os.Exit(1)
								}
								err := substituteFile(path, environment, flags)
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
				environment, diagnostic := initEnvironment(flags)
				if diagnostic == nil {
					reader := source.New("evaluate", options[1])
					expr, diagnostic := parser.ReadExpression(&reader)
					if diagnostic == nil {
						folded, _, diagnostic := expr.Fold(environment, expression.Branch{})
						if diagnostic == nil {
							snippet, diagnostic := folded.Evaluate(environment)
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
				environment, diagnostic := initEnvironment(flags)
				if diagnostic == nil {
					reader := source.New("evaluate", options[1])
					expr, diagnostic := parser.ReadExpression(&reader)
					if diagnostic == nil {
						_, pattern, diagnostic := expr.Fold(environment, expression.Branch{})
						if diagnostic == nil {
							fmt.Println(pattern.Source)
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
func substituteFile(path string, environment expression.Environment, flags Flags) error {
	cwd, err := os.Getwd()
	if err == nil {
		// The name used for the reader
		// is the relative path from cwd
		// to the file.
		name, err := filepath.Rel(cwd, path)
		if err == nil {
			content, err := os.ReadFile(path)
			if err == nil {
				s10n, err := substitute(name, string(content), environment)
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

func substituteDir(path string, environment expression.Environment, flags Flags) error {
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
								s10n, err := substitute(rel, string(content), environment)
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
						s10n, err := substitute(rel, string(content), environment)
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
func substitute(name string, content string, environment expression.Environment) (string, error) {
	var reader source.Reader = source.New(name, content)
	var buffer strings.Builder
	for reader.CanRead() {
		if reader.ReadOnly("<") {
			body, diagnostic := parser.ReadExpression(&reader)
			if diagnostic == nil {
				folded, _, diagnostic := body.Fold(environment, expression.Branch{})
				if diagnostic == nil {
					snippet, diagnostic := folded.Evaluate(environment)
					if diagnostic == nil {
						buffer.WriteString(snippet)
						if reader.ReadMeaningful(">") {
							continue
						}
						return buffer.String(), source.Diagnostic{
							Message: "Expected '>'",
							Excerpt: reader.PointExcerpt(),
						}
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

func initEnvironment(flags Flags) (expression.Environment, error) {
	cwd, err := os.Getwd()
	if err == nil {
		context, err := flags.ResolveContext()
		if err == nil {
			content, err := os.ReadFile(context)
			if err == nil {
				name, err := filepath.Rel(cwd, context)
				if err == nil {
					reader := source.New(name, string(content))
					entries, diagnostic := func() (map[string]expression.Macro, error) {
						buffer := map[string]expression.Macro{}
						for {
							entry, diagnostic := parser.ReadDefinition(&reader)
							if diagnostic == nil {
								if entry != nil {
									buffer[entry.Name()] = entry
									continue
								}
								return buffer, nil
							}
							return nil, diagnostic
						}
					}()
					if diagnostic == nil {
						environment := expression.Environment{
							Root:    nil,
							Parent:  &expression.Standard,
							Entries: entries,
						}
						environment.Root = &environment
						for _, entry := range environment.Entries {
							if macro, ok := entry.(*expression.Source); ok {
								diagnostic := macro.Fold(environment, expression.Branch{})
								if diagnostic == nil {
									continue
								}
								return expression.Environment{}, diagnostic
							}
							panic("Attempted to fold a non-source macro")
						}
						return environment, nil
					}
					return expression.Environment{}, diagnostic
				}
				return expression.Environment{}, err
			}
			return expression.Environment{}, err
		}
		return expression.Environment{}, err
	}
	return expression.Environment{}, err
}

// The CLI flags passed to the circumscribe
// executable. These are different from
// options: those are collected separately.
type Flags struct {
	output  *string // The output path, or nil if none was given         (substitute)
	context *string // The content file, or nil if none was given        (evaluate, substitute, init)
	exclude *string // The exclude pattern, or nil if none was given     (substitute)
	mirror  bool    // Whether to mirror the rest of the directory       (substitute)
	force   bool    // Whether to create .circumscribe if one is present (init)
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
func (self Flags) ResolveContext() (string, error) {
	if self.context == nil {
		cwd, err := os.Getwd()
		if err == nil {
			path := filepath.Join(cwd, ".circumscribe")
			stat, err := os.Stat(path)
			if err == nil {
				mode := stat.Mode()
				if mode.IsRegular() {
					return path, nil
				}
				return path, errors.New("circumscribe: The context file must be a regular file")
			}
			if errors.Is(err, fs.ErrNotExist) {
				return path, errors.New("circumscribe: Could not locate '.circumscribe' in the current working directory. Perhaps 'circumscribe init'?")
			}
			return path, err
		}
		return cwd, err
	}
	stat, err := os.Stat(*self.context)
	if err == nil {
		mode := stat.Mode()
		if mode.IsRegular() {
			return *self.context, nil
		}
		return *self.context, fmt.Errorf("circumscribe: The context file must be a regular file")
	}
	return *self.context, err
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
					flags.context = &context
					continue
				}
				cwd, err := os.Getwd()
				if err == nil {
					context := filepath.Join(cwd, args[position])
					flags.context = &context
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
