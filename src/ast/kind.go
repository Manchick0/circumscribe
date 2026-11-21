package ast

import (
	"circumscribe/env"
	"circumscribe/pat"
	"circumscribe/source"
)

type Literal struct {
	Literal string
	Excerpt source.Excerpt
}

func (self Literal) expr() {}
func (self Literal) Origin() *source.Excerpt {
	return &self.Excerpt
}

type Snippet struct {
	Children []Expression
	Excerpt  source.Excerpt
}

func (self Snippet) expr() {}
func (self Snippet) Origin() *source.Excerpt {
	return &self.Excerpt
}

type Application struct {
	Identifier string
	Arguments  []Expression
	Bare       bool
	Excerpt    source.Excerpt
}

func (self Application) expr() {}
func (self Application) Origin() *source.Excerpt {
	return &self.Excerpt
}

func (self Application) Infer(envm env.Environment[Macro], envp env.Environment[pat.Pattern], branch env.Branch[Macro]) (pat.Pattern, error) {
	macro, _ := envm.Traverse(self.Identifier)
	return macro.Infer(envm, envp, branch)
}

func (self Application) IsConstant(envm env.Environment[Macro], envp env.Environment[pat.Pattern], branch env.Branch[Macro]) (bool, error) {
	if len(self.Arguments) == 0 {
		macro, _ := envm.Traverse(self.Identifier)
		return macro.IsPure(envm, envp, branch)
	}
	return false, nil
}

type If struct {
	Condition Expression
	Body      Expression
	Otherwise Expression
	Excerpt   source.Excerpt
}

func (self If) expr() {}
func (self If) Origin() *source.Excerpt {
	return &self.Excerpt
}

type Case struct {
	Grammar pat.Grammar
	Body    Expression
}

type Switch struct {
	Branches  []Case
	Source    Expression
	Otherwise Expression
	Excerpt   source.Excerpt
}

func (self Switch) expr() {}
func (self Switch) Origin() *source.Excerpt {
	return &self.Excerpt
}

type With struct {
	Identifier string
	Expression Expression
	Body       Expression
	Excerpt    source.Excerpt
}

func (self With) expr() {}
func (self With) Origin() *source.Excerpt {
	return &self.Excerpt
}

type Raise struct {
	Expression Expression
	Excerpt    source.Excerpt
}

func (self Raise) expr() {}
func (self Raise) Origin() *source.Excerpt {
	return &self.Excerpt
}

type Compare struct {
	Left    Expression
	Right   Expression
	Excerpt source.Excerpt
}

func (self Compare) expr() {}
func (self Compare) Origin() *source.Excerpt {
	return &self.Excerpt
}

type Satisfies struct {
	Expression Expression
	Pattern    pat.Pattern
	Excerpt    source.Excerpt
}

func (self Satisfies) expr() {}
func (self Satisfies) Origin() *source.Excerpt {
	return &self.Excerpt
}

// Logic

type And struct {
	Left    Expression
	Right   Expression
	Excerpt source.Excerpt
}

func (self And) expr() {}
func (self And) Origin() *source.Excerpt {
	return &self.Excerpt
}

type Or struct {
	Left    Expression
	Right   Expression
	Excerpt source.Excerpt
}

func (self Or) expr() {}
func (self Or) Origin() *source.Excerpt {
	return &self.Excerpt
}

type Not struct {
	Expression Expression
	Excerpt    source.Excerpt
}

func (self Not) expr() {}
func (self Not) Origin() *source.Excerpt {
	return &self.Excerpt
}

// Naturals

type Add struct {
	Left    Expression
	Right   Expression
	Excerpt source.Excerpt
}

func (self Add) expr() {}
func (self Add) Origin() *source.Excerpt {
	return &self.Excerpt
}

type Sub struct {
	Left    Expression
	Right   Expression
	Excerpt source.Excerpt
}

func (self Sub) expr() {}
func (self Sub) Origin() *source.Excerpt {
	return &self.Excerpt
}

type Mul struct {
	Left    Expression
	Right   Expression
	Excerpt source.Excerpt
}

func (self Mul) expr() {}
func (self Mul) Origin() *source.Excerpt {
	return &self.Excerpt
}

type Div struct {
	Left    Expression
	Right   Expression
	Excerpt source.Excerpt
}

func (self Div) expr() {}
func (self Div) Origin() *source.Excerpt {
	return &self.Excerpt
}

type Less struct {
	Left    Expression
	Right   Expression
	Excerpt source.Excerpt
}

func (self Less) expr() {}
func (self Less) Origin() *source.Excerpt {
	return &self.Excerpt
}

type Greater struct {
	Left    Expression
	Right   Expression
	Excerpt source.Excerpt
}

func (self Greater) expr() {}
func (self Greater) Origin() *source.Excerpt {
	return &self.Excerpt
}
