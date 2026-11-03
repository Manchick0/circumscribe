package expression

import (
	"circumscribe/pattern"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

type Environment struct {
	Root    *Environment
	Parent  *Environment
	Entries map[string]Macro
}

var (
	StdHead = Native{
		Identifier: "head",
		Params: []Parameter{
			{
				Name:    "source",
				Pattern: pattern.StdAny,
			},
		},
		Body: func(environment Environment, arguments []string) (string, error) {
			head, _ := utf8.DecodeRuneInString(arguments[0])
			return fmt.Sprintf("%c", head), nil
		},
		Pattern: pattern.Any(),
		Pure:    true,
	}
	StdTail = Native{
		Identifier: "tail",
		Params: []Parameter{
			{
				Name:    "source",
				Pattern: pattern.StdAny,
			},
		},
		Body: func(environment Environment, arguments []string) (string, error) {
			_, len := utf8.DecodeRuneInString(arguments[0])
			return arguments[0][len:], nil
		},
		Pattern: pattern.StdAny,
		Pure:    true,
	}
	StdLowercase = Native{
		Identifier: "lowercase",
		Params: []Parameter{
			{
				Name:    "source",
				Pattern: pattern.StdAny,
			},
		},
		Body: func(environment Environment, arguments []string) (string, error) {
			return strings.ToLower(arguments[0]), nil
		},
		Pattern: pattern.StdAny,
		Pure:    true,
	}
	StdUppercase = Native{
		Identifier: "uppercase",
		Params: []Parameter{
			{
				Name:    "source",
				Pattern: pattern.StdAny,
			},
		},
		Body: func(environment Environment, arguments []string) (string, error) {
			return strings.ToUpper(arguments[0]), nil
		},
		Pattern: pattern.StdAny,
		Pure:    true,
	}
	StdEnv = Native{
		Identifier: "env",
		Params: []Parameter{
			{
				Name:    "name",
				Pattern: pattern.StdAny,
			},
		},
		Body: func(environment Environment, arguments []string) (string, error) {
			return os.Getenv(arguments[0]), nil
		},
		Pattern: pattern.StdAny,
		Pure:    false,
	}
	StdRead = Native{
		Identifier: "read",
		Params: []Parameter{
			{
				Name:    "path",
				Pattern: pattern.StdAny,
			},
		},
		Body: func(environment Environment, arguments []string) (string, error) {
			content, err := os.ReadFile(arguments[0])
			if err == nil {
				return string(content), nil
			}
			return "", err
		},
		Pattern: pattern.StdAny,
		Pure:    false,
	}
	StdTrim = Native{
		Identifier: "trim",
		Params: []Parameter{
			{
				Name:    "source",
				Pattern: pattern.StdAny,
			},
		},
		Body: func(environment Environment, arguments []string) (string, error) {
			return strings.TrimSpace(arguments[0]), nil
		},
		Pattern: pattern.StdAny,
		Pure:    true,
	}
	StdLength = Native{
		Identifier: "length",
		Params: []Parameter{
			{
				Name:    "source",
				Pattern: pattern.StdAny,
			},
		},
		Body: func(environment Environment, arguments []string) (string, error) {
			return fmt.Sprintf("%d", len(arguments[0])), nil
		},
		Pattern: pattern.StdAny,
		Pure:    true,
	}
	StdContains = Native{
		Identifier: "contains",
		Params: []Parameter{
			{
				Name:    "source",
				Pattern: pattern.StdAny,
			},
			{
				Name:    "snippet",
				Pattern: pattern.StdAny,
			},
		},
		Body: func(environment Environment, arguments []string) (string, error) {
			if strings.Contains(arguments[0], arguments[1]) {
				return "true", nil
			}
			return "false", nil
		},
		Pattern: pattern.StdBool,
		Pure:    true,
	}
	StdMeaningful = Native{
		Identifier: "meaningful",
		Params: []Parameter{
			{
				Name:    "source",
				Pattern: pattern.StdAny,
			},
		},
		Body: func(environment Environment, arguments []string) (string, error) {
			if len(arguments[0]) > 0 {
				return "true", nil
			}
			return "false", nil
		},
		Pattern: pattern.StdBool,
		Pure:    true,
	}
)

var Standard = Environment{
	Parent: nil,
	Root:   nil,
	Entries: map[string]Macro{
		"length":     StdLength,
		"contains":   StdContains,
		"head":       StdHead,
		"tail":       StdTail,
		"trim":       StdTrim,
		"meaningful": StdMeaningful,
		"read":       StdRead,
		"env":        StdEnv,
		"uppercase":  StdUppercase,
		"lowercase":  StdLowercase,
	},
}

func (self *Environment) traverse(identifier string) Macro {
	entry, ok := self.Entries[identifier]
	if ok {
		return entry
	}
	if self.Parent != nil {
		return self.Parent.traverse(identifier)
	}
	return nil
}

type Branch struct {
	Parent *Branch
	Entry  Macro
}

func (self Branch) contains(candidate Macro) bool {
	if self.Entry != nil && &self.Entry == &candidate {
		return true
	}
	return self.Parent != nil && self.Parent.contains(candidate)
}
