package pat

import (
	"circumscribe/env"
	"circumscribe/iters"
	"circumscribe/sgr"
	"circumscribe/source"
	"fmt"
	"iter"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"
)

var (
	StdW       = Reference("w")
	StdE       = Reference("e")
	StdBoolean = Reference("boolean")
	StdNatural = Reference("natural")
	StdNever   = Reference("never")
	StdAny     = Reference("any")
)

type Pattern interface {
	Compile(env.Environment[Pattern], env.Branch[Pattern]) (*nFA, error)
	Pretty(bool, int) string
	Origin() *source.Excerpt
}

type interval struct {
	interval [2]rune
	machine  *nFA
	excerpt  *source.Excerpt
}

func Interval(from rune, to rune) Pattern {
	return &interval{
		interval: [2]rune{from, to + 1},
		machine:  nil,
		excerpt:  nil,
	}
}

func (self *interval) Compile(environment env.Environment[Pattern], branch env.Branch[Pattern]) (*nFA, error) {
	if self.machine == nil {
		self.machine = &nFA{
			initial: 0,
			accept:  []uint16{1},
			epsilon: [][]uint16{{}, {}},
			transition: [][]thread{
				{{w: self.interval, q: []uint16{1}}},
				{},
			},
		}
		return self.machine, nil
	}
	return self.machine, nil
}

func (self *interval) Origin() *source.Excerpt {
	return self.excerpt
}

func (self *interval) Pretty(color bool, precedence int) string {
	if color {
		return fmt.Sprintf("%s-%s", sgr.Escape(fmt.Sprintf("|\\u%04x|", self.interval[0]), sgr.Literal), sgr.Escape(fmt.Sprintf("|\\u%04x|", self.interval[1]-1), sgr.Literal))
	}
	return fmt.Sprintf("|\\u%04x|-|\\u%04x|", self.interval[0], self.interval[1]-1)
}

func (self *interval) String() string {
	return self.Pretty(false, PrecAll)
}

type literal struct {
	literal string
	machine *nFA
	excerpt *source.Excerpt
}

func Literal(lit string) Pattern {
	return &literal{
		literal: lit,
		machine: nil,
		excerpt: nil,
	}
}

func (self *literal) Compile(environment env.Environment[Pattern], branch env.Branch[Pattern]) (*nFA, error) {
	if self.machine == nil {
		if len(self.literal) > 0 {
			epsilon := make([][]uint16, len(self.literal)+1)
			transition := make([][]thread, len(self.literal)+1)
			buffer := strings.NewReader(self.literal)
			for n := 0; buffer.Len() > 0; n++ {
				color, _, err := buffer.ReadRune()
				if err == nil {
					transition[n] = []thread{
						{
							w: [2]rune{color, color + 1},
							q: []uint16{uint16(n + 1)},
						},
					}
					epsilon[n] = []uint16{}
					continue
				}
				return nil, err
			}
			self.machine = &nFA{
				initial:    0,
				accept:     []uint16{uint16(utf8.RuneCountInString(self.literal))},
				epsilon:    epsilon,
				transition: transition,
			}
			return self.machine, nil
		}
		self.machine = &nFA{
			initial:    0,
			accept:     []uint16{1},
			epsilon:    [][]uint16{{1}, {}},
			transition: [][]thread{{}, {}},
		}
		return self.machine, nil
	}
	return self.machine, nil
}

func (self *literal) Origin() *source.Excerpt {
	return self.excerpt
}

func (self *literal) Pretty(color bool, precendence int) string {
	if color {
		return sgr.Escape(self.String(), sgr.Literal)
	}
	return self.String()
}

func (self *literal) String() string {
	return fmt.Sprintf("|%s|", strings.ReplaceAll(self.literal, "\n", "\\n"))
}

type sum struct {
	left    Pattern
	right   Pattern
	machine *nFA
	excerpt *source.Excerpt
}

func Sum(left Pattern, right Pattern) Pattern {
	return &sum{
		left:    left,
		right:   right,
		machine: nil,
		excerpt: nil,
	}
}

func (self *sum) Compile(environment env.Environment[Pattern], branch env.Branch[Pattern]) (*nFA, error) {
	if self.machine == nil {
		left, err := self.left.Compile(environment, branch)
		if err == nil {
			right, err := self.right.Compile(environment, branch)
			if err == nil {
				right := right.shift(len(left.transition))
				self.machine = &nFA{
					initial:       uint16(len(left.transition) + len(right.transition)),
					accept:        []uint16{uint16(len(left.transition) + len(right.transition) + 1)},
					epsilon:       slices.Concat(left.epsilon, right.epsilon, [][]uint16{{left.initial, right.initial}, {}}),
					transition:    slices.Concat(left.transition, right.transition, [][]thread{{}, {}}),
					deterministic: nil,
				}
				for _, accept := range left.accept {
					self.machine.epsilon[accept] = append(self.machine.epsilon[accept], uint16(len(left.transition)+len(right.transition)+1))
				}
				for _, accept := range right.accept {
					self.machine.epsilon[accept] = append(self.machine.epsilon[accept], uint16(len(left.transition)+len(right.transition)+1))
				}
				return self.machine, nil
			}
			return nil, err
		}
		return nil, err
	}
	return self.machine, nil
}

func (self *sum) Origin() *source.Excerpt {
	return self.excerpt
}

func (self *sum) Pretty(color bool, precedence int) string {
	if precedence < PrecAdd {
		return fmt.Sprintf("(%s + %s)", self.left.Pretty(color, PrecAdd), self.right.Pretty(color, PrecAdd))
	}
	return fmt.Sprintf("%s + %s", self.left.Pretty(color, PrecAdd), self.right.Pretty(color, PrecAdd))
}

func (self *sum) String() string {
	return self.Pretty(false, PrecAdd)
}

type product struct {
	left    Pattern
	right   Pattern
	machine *nFA
	excerpt *source.Excerpt
}

func Product(left Pattern, right Pattern) Pattern {
	return &product{
		left:    left,
		right:   right,
		machine: nil,
		excerpt: nil,
	}
}

func (self *product) Compile(environment env.Environment[Pattern], branch env.Branch[Pattern]) (*nFA, error) {
	if self.machine == nil {
		left, err := self.left.Compile(environment, branch)
		if err == nil {
			right, err := self.right.Compile(environment, branch)
			if err == nil {
				right := right.shift(len(left.transition))
				self.machine = &nFA{
					initial:       left.initial,
					accept:        right.accept,
					epsilon:       slices.Concat(left.epsilon, right.epsilon),
					transition:    slices.Concat(left.transition, right.transition),
					deterministic: nil,
				}
				for _, accept := range left.accept {
					self.machine.transition[accept] = self.machine.transition[right.initial]
					self.machine.epsilon[accept] = self.machine.epsilon[right.initial]
				}
				return self.machine, nil
			}
			return nil, err
		}
		return nil, err
	}
	return self.machine, nil
}

func (self *product) Origin() *source.Excerpt {
	return self.excerpt
}

func (self *product) Pretty(color bool, precedence int) string {
	if precedence < PrecMul {
		return fmt.Sprintf("(%s * %s)", self.left.Pretty(color, PrecMul), self.right.Pretty(color, PrecMul))
	}
	return fmt.Sprintf("%s * %s", self.left.Pretty(color, PrecMul), self.right.Pretty(color, PrecMul))
}

func (self *product) String() string {
	return self.Pretty(false, PrecMul)
}

type intersection struct {
	left    Pattern
	right   Pattern
	machine *nFA
	excerpt *source.Excerpt
}

func Intersection(left Pattern, right Pattern) Pattern {
	return &intersection{
		left:    left,
		right:   right,
		machine: nil,
		excerpt: nil,
	}
}

func (self *intersection) Compile(environment env.Environment[Pattern], branch env.Branch[Pattern]) (*nFA, error) {
	if self.machine == nil {
		left, err := self.left.Compile(environment, branch)
		if err == nil {
			right, err := self.right.Compile(environment, branch)
			if err == nil {
				return left.Intersection(right), nil
			}
			return nil, err
		}
		return nil, err
	}
	return self.machine, nil
}

func (self *intersection) Origin() *source.Excerpt {
	return self.excerpt
}

func (self *intersection) Pretty(color bool, precedence int) string {
	if precedence < PrecAnd {
		return fmt.Sprintf("(%s & %s)", self.left.Pretty(color, PrecMul), self.right.Pretty(color, PrecMul))
	}
	return fmt.Sprintf("%s & %s", self.left.Pretty(color, PrecMul), self.right.Pretty(color, PrecMul))
}

func (self *intersection) String() string {
	return self.Pretty(false, PrecAnd)
}

type ellipsis struct {
	pattern Pattern
	machine *nFA
	excerpt *source.Excerpt
}

func Ellipsis(pattern Pattern) Pattern {
	return &ellipsis{
		pattern: pattern,
		machine: nil,
		excerpt: nil,
	}
}

func (self *ellipsis) Compile(environment env.Environment[Pattern], branch env.Branch[Pattern]) (*nFA, error) {
	if self.machine == nil {
		pattern, err := self.pattern.Compile(environment, branch)
		if err == nil {
			if !pattern.IsNullary() {
				pattern := pattern.shift(0)
				for _, accept := range pattern.accept {
					pattern.epsilon[accept] = append(pattern.epsilon[accept], pattern.initial)
					pattern.epsilon[accept] = append(pattern.epsilon[accept], uint16(len(pattern.transition)+1))
				}
				self.machine = &nFA{
					initial:       uint16(len(pattern.transition)),
					accept:        []uint16{uint16(len(pattern.transition) + 1)},
					epsilon:       slices.Concat(pattern.epsilon, [][]uint16{{pattern.initial, uint16(len(pattern.transition) + 1)}, {}}),
					transition:    slices.Concat(pattern.transition, [][]thread{{}, {}}),
					deterministic: nil,
				}
				return self.machine, nil
			}
			return nil, source.Diagnostic{
				Message: "A nullable pattern may not appear in an ellipsis",
				Excerpt: self.pattern.Origin(),
			}
		}
		return nil, err
	}
	return self.machine, nil
}

func (self *ellipsis) Origin() *source.Excerpt {
	return self.excerpt
}

func (self *ellipsis) Pretty(color bool, precedence int) string {
	return fmt.Sprintf("%s...", self.pattern.Pretty(color, PrecEll))
}

func (self *ellipsis) String() string {
	return self.Pretty(false, PrecEll)
}

type complement struct {
	pattern Pattern
	machine *nFA
	excerpt *source.Excerpt
}

func Complement(pattern Pattern) Pattern {
	return &complement{
		pattern: pattern,
		machine: nil,
		excerpt: nil,
	}
}

func (self *complement) Compile(environment env.Environment[Pattern], branch env.Branch[Pattern]) (*nFA, error) {
	if self.machine == nil {
		machine, err := self.pattern.Compile(environment, branch)
		if err == nil {
			self.machine = machine.Complement()
			return self.machine, nil
		}
		return nil, err
	}
	return self.machine, nil
}

func (self *complement) Origin() *source.Excerpt {
	return self.excerpt
}

func (self *complement) Pretty(color bool, precedence int) string {
	return fmt.Sprintf("!%s", self.pattern.Pretty(color, PrecEll))
}

func (self *complement) String() string {
	return self.Pretty(false, PrecEll)
}

type reference struct {
	identifier string
	machine    *nFA
	excerpt    *source.Excerpt
}

func Reference(identifier string) Pattern {
	return &reference{
		identifier: identifier,
		machine:    nil,
		excerpt:    nil,
	}
}

func (self *reference) Compile(environment env.Environment[Pattern], branch env.Branch[Pattern]) (*nFA, error) {
	if self.machine == nil {
		if !branch.Contains(self) {
			if pattern, ok := environment.Traverse(self.identifier); ok {
				return pattern.Compile(environment, env.Branch[Pattern]{
					Parent: &branch,
					Entry:  self,
				})
			}
			return nil, source.Diagnostic{
				Message: fmt.Sprintf("Could not resolve identifier '%s'", self.identifier),
				Excerpt: self.excerpt,
			}
		}
		return nil, source.Diagnostic{
			Message: "Recursion within a pattern breaks its regularity",
			Excerpt: self.excerpt,
		}
	}
	return self.machine, nil
}

func (self *reference) Origin() *source.Excerpt {
	return self.excerpt
}

func (self *reference) Pretty(color bool, precdence int) string {
	if color {
		return sgr.Escape(self.identifier, sgr.Reference)
	}
	return self.identifier
}

func (self *reference) String() string {
	return self.Pretty(false, PrecAll)
}

type nFA struct {
	initial       uint16
	accept        []uint16
	epsilon       [][]uint16
	transition    [][]thread
	deterministic *nFA
}

type thread struct {
	w [2]rune
	q []uint16
}

func (self *nFA) PartiallySatisfies(language string) []string {
	if self.deterministic == nil || self.deterministic == self {
		buffer := map[string]bool{}
		Q := []uint16{}
		for qq := range self.closure(self.initial) {
			if slices.Contains(self.accept, qq) {
				buffer[language] = true
			}
			Q = append(Q, qq)
			continue
		}
		for len(language) > 0 && len(Q) > 0 {
			QQ := []uint16{}
			color, len := utf8.DecodeRuneInString(language)
			language = language[len:]
			for _, q := range Q {
				for _, branch := range self.transition[q] {
					if branch.w[0] <= color && color < branch.w[1] {
						for _, qq := range branch.q {
							for qe := range self.closure(qq) {
								if slices.Contains(self.accept, qe) {
									buffer[language] = true
								}
								QQ = append(QQ, qe)
								continue
							}
							continue
						}
						continue
					}
					continue
				}
				continue
			}
			Q = QQ
			continue
		}
		return slices.Collect(maps.Keys(buffer))
	}
	return self.deterministic.PartiallySatisfies(language)
}

func (self *nFA) Satisfies(language string) bool {
	if self.deterministic == nil || self.deterministic == self {
		Q := []uint16{}
		for qq := range self.closure(self.initial) {
			Q = append(Q, qq)
			continue
		}
		for len(language) > 0 && len(Q) > 0 {
			QQ := []uint16{}
			color, len := utf8.DecodeRuneInString(language)
			language = language[len:]
			for _, q := range Q {
				for _, branch := range self.transition[q] {
					if branch.w[0] <= color && color < branch.w[1] {
						for _, qq := range branch.q {
							for qe := range self.closure(qq) {
								QQ = append(QQ, qe)
								continue
							}
							continue
						}
						continue
					}
					continue
				}
				continue
			}
			Q = QQ
			continue
		}
		return slices.ContainsFunc(self.accept, func(accept uint16) bool {
			return slices.Contains(Q, accept)
		})
	}
	return self.deterministic.Satisfies(language)
}

// Determine whether the automaton is nullary.
// ---
// An automaton is considered nullary if an accept
// state may be reached from the initial one by
// only following epsilon transitions.
//
// @since 1.0.0
func (self *nFA) IsNullary() bool {
	closure := self.closure(self.initial)
	for q := range closure {
		if slices.Contains(self.accept, q) {
			return true
		}
		continue
	}
	return false
}

func (self *nFA) IsMeaningful() bool {
	visited := map[uint16]bool{self.initial: true}
	buffer := []uint16{self.initial}
	for len(buffer) > 0 {
		q := buffer[len(buffer)-1]
		buffer = buffer[:len(buffer)-1]
		if !slices.Contains(self.accept, q) {
			for _, q := range self.transition[q] {
				for _, qq := range q.q {
					if _, ok := visited[qq]; ok {
						continue
					}
					buffer = append(buffer, qq)
					visited[qq] = true
				}
			}
			for _, q := range self.epsilon[q] {
				if _, ok := visited[q]; ok {
					continue
				}
				buffer = append(buffer, q)
				visited[q] = true
			}
			continue
		}
		return true
	}
	return false
}

// Compute the intersection of two automata.
// ---
// Computes the product automaton that represents
// the intersection L(self) ∩ L(other), and simulates
// running both self and the given other on the same
// input string.
//
// @since 1.0.0
func (self *nFA) Intersection(other *nFA) *nFA {
	buffer := [][2]uint16{{self.initial, other.initial}}
	dictionary := map[[2]uint16]uint16{
		{self.initial, other.initial}: 0,
	}
	machine := nFA{
		initial:       0,
		accept:        []uint16{},
		epsilon:       [][]uint16{{}},
		transition:    [][]thread{{}},
		deterministic: nil,
	}
	for len(buffer) > 0 {
		Q := buffer[len(buffer)-1]
		W := func() [][2]rune {
			buffer := map[[2]rune]bool{}
			for q := range self.closure(Q[0]) {
				for _, qq := range self.transition[q] {
					buffer[qq.w] = true
				}
			}
			for q := range other.closure(Q[1]) {
				for _, qq := range other.transition[q] {
					buffer[qq.w] = true
				}
			}
			return disjoin(slices.Collect(maps.Keys(buffer)))
		}()
		position := dictionary[Q]
		buffer = buffer[:len(buffer)-1]
		// Compute all possible interval transitions from state Q
		for _, w := range W {
			left := self.step(Q[0], w)
			right := other.step(Q[1], w)
			if len(left) == 0 || len(right) == 0 {
				continue
			}
			machine.transition[position] = append(machine.transition[position], func() thread {
				branch := thread{
					w: w,
					q: []uint16{},
				}
				for _, p := range left {
					for _, q := range right {
						pq := [2]uint16{p, q}
						if _, ok := dictionary[pq]; !ok {
							position := uint16(len(machine.transition))
							machine.transition = append(machine.transition, []thread{})
							machine.epsilon = append(machine.epsilon, []uint16{})
							branch.q = append(branch.q, position)
							buffer = append(buffer, pq)
							dictionary[pq] = position
							continue
						}
						branch.q = append(branch.q, dictionary[pq])
					}
				}
				return branch
			}())
		}
		// Determine whether Q is accepting
		cl, cr := self.closure(Q[0]), other.closure(Q[1])
		if slices.ContainsFunc(self.accept, func(q uint16) bool {
			return cl[q]
		}) && slices.ContainsFunc(other.accept, func(q uint16) bool {
			return cr[q]
		}) {
			machine.accept = append(machine.accept, position)
		}
	}
	return &machine
}

func (self *nFA) Complement() *nFA {
	deterministic := self.Determinize()
	machine := nFA{
		initial: deterministic.initial,
		accept: func() []uint16 {
			buffer := []uint16{}
			for n := range deterministic.transition {
				if slices.Contains(deterministic.accept, uint16(n)) {
					continue
				}
				buffer = append(buffer, uint16(n))
			}
			return buffer
		}(),
		// We may point to the tables of the original automaton
		// here, in order to reduce the potentially high memory
		// usage of a deterministic automaton.
		epsilon:       deterministic.epsilon,
		transition:    deterministic.transition,
		deterministic: nil,
	}
	machine.deterministic = &machine
	return &machine
}

// Compute the deterministic counterpart of the
// automaton, following the subset construction.
// ---
// The deterministic automaton will contain a
// sink state, to which all unhandled transitions
// from every state will point.
func (self *nFA) Determinize() *nFA {
	if self.deterministic == nil {
		closure := self.closure(self.initial)
		dictionary := map[string]uint16{serialize(closure): 1}
		buffer := []map[uint16]bool{closure}
		machine := nFA{
			initial:       1,
			accept:        []uint16{},
			epsilon:       [][]uint16{{}, {}},
			transition:    [][]thread{{{w: [2]rune{0x0000, 0x110000}, q: []uint16{0}}}, {}},
			deterministic: nil,
		}
		for len(buffer) > 0 {
			Q := buffer[len(buffer)-1]
			// Compute the alphabet of disjoint
			// intervals from all substates
			W := func() [][2]rune {
				buffer := map[[2]rune]bool{{0x0000, 0x110000}: true}
				for q := range Q {
					for _, branch := range self.transition[q] {
						buffer[branch.w] = true
					}
				}
				return disjoin(slices.Collect(maps.Keys(buffer)))
			}()
			buffer = buffer[:len(buffer)-1]
			position := dictionary[serialize(Q)]
			// Compute all possible interval tranistions from state Q
			for _, w := range W {
				dq := func() map[uint16]bool {
					buffer := map[uint16]bool{}
					for q := range Q {
						for _, qq := range self.step(q, w) {
							maps.Copy(buffer, self.closure(qq))
						}
					}
					return buffer
				}()
				if len(dq) > 0 {
					machine.transition[position] = append(machine.transition[position], thread{
						w: w,
						q: func() []uint16 {
							name := serialize(dq)
							if _, ok := dictionary[name]; !ok {
								position := uint16(len(machine.transition))
								machine.transition = append(machine.transition, []thread{})
								machine.epsilon = append(machine.epsilon, []uint16{})
								dictionary[name] = position
								buffer = append(buffer, dq)
								return []uint16{position}
							}
							return []uint16{dictionary[name]}
						}(),
					})
					continue
				}
				machine.transition[position] = append(machine.transition[position], thread{
					w: w,
					q: []uint16{0},
				})
			}
			// Determine whether Q is accepting
			for q := range Q {
				if slices.Contains(self.accept, q) {
					machine.accept = append(machine.accept, position)
					break
				}
			}
		}
		machine.deterministic = &machine
		self.deterministic = &machine
		return self.deterministic
	}
	return self.deterministic
}

func (self *nFA) String() string {
	return fmt.Sprintf("initial = %d -> accept = %v transition = %v", self.initial, self.accept, self.transition)
}

// Compute the set of reachable states from
// the given non-deterministic state 'from',
// following the given transition 'w', or
// any epsilon transitions.
// ---
func (self *nFA) step(from uint16, w [2]rune) []uint16 {
	buffer := map[uint16]bool{}
	for q := range self.closure(from) {
		for _, qq := range self.transition[q] {
			if qq.w[0] <= w[0] && w[1] <= qq.w[1] {
				for _, q := range qq.q {
					buffer[q] = true
				}
				continue
			}
			continue
		}
	}
	return slices.Collect(maps.Keys(buffer))
}

func (self *nFA) alphabet(from uint16) [][2]rune {
	var buffer [][2]rune
	for _, thread := range self.transition[from] {
		buffer = append(buffer, thread.w)
	}
	return disjoin(buffer)
}

// Compute a string representation of
// the given non-deterministic state Q.
// ---
func serialize(Q map[uint16]bool) string {
	var buffer strings.Builder
	values := slices.Collect(maps.Keys(Q))
	slices.Sort(values)
	for n, q := range values {
		if n > 0 {
			buffer.WriteRune(':')
		}
		fmt.Fprint(&buffer, q)
	}
	return buffer.String()
}

func (self *nFA) shift(n int) nFA {
	return nFA{
		initial: self.initial + uint16(n),
		accept: slices.Collect(iters.Fmap(slices.Values(self.accept), func(q uint16) uint16 {
			return q + uint16(n)
		})),
		epsilon: slices.Collect(iters.Fmap(slices.Values(self.epsilon), func(xs []uint16) []uint16 {
			return slices.Collect(iters.Fmap(slices.Values(xs), func(q uint16) uint16 {
				return q + uint16(n)
			}))
		})),
		transition: slices.Collect(iters.Fmap(slices.Values(self.transition), func(xs []thread) []thread {
			return slices.Collect(iters.Fmap(slices.Values(xs), func(t thread) thread {
				return thread{
					w: t.w,
					q: slices.Collect(iters.Fmap(slices.Values(t.q), func(q uint16) uint16 {
						return q + uint16(n)
					})),
				}
			}))
		})),
	}
}

func (self *nFA) closure(q uint16) map[uint16]bool {
	buffer := make(map[uint16]bool)
	for _, qq := range self.epsilon[q] {
		maps.Copy(buffer, self.closure(qq))
	}
	buffer[q] = true
	return buffer
}

func disjoin(candidates [][2]rune) [][2]rune {
	entries := slices.Collect(iters.Bind(slices.Values(candidates), func(candidate [2]rune) iter.Seq[[2]rune] {
		return slices.Values([][2]rune{
			{candidate[0], 1},
			{candidate[1], -1},
		})
	}))
	slices.SortFunc(entries, func(a [2]rune, b [2]rune) int {
		if a[0] == b[0] {
			return int(a[1] - b[1])
		}
		return int(a[0] - b[0])
	})
	buffer := make([][2]rune, 0, len(candidates))
	var open rune = 0
	for i := 0; i < len(entries)-1; i++ {
		left := entries[i]
		right := entries[i+1]
		open += left[1]
		if open > 0 && left[0] != right[0] {
			buffer = append(buffer, [2]rune{left[0], right[0]})
		}
	}
	return buffer
}

const (
	PrecEll = iota
	PrecAnd
	PrecMul
	PrecAdd
	PrecAll
)

func ReadPattern(reader *source.Reader, precedence int) (Pattern, error) {
	if reader.SkipWhitespace() {
		position := reader.Position()
		if reader.ReadOnly("(") {
			expression, diagnostic := ReadPattern(reader, PrecAll)
			if diagnostic == nil {
				if reader.ReadSyntax(")") {
					return readTrailing(reader, expression, precedence)
				}
				return nil, source.Diagnose("Expected ')'", reader.PointExcerpt())
			}
			return nil, diagnostic
		}
		if reader.ReadOnly("!") {
			pattern, err := ReadPattern(reader, PrecEll)
			if err == nil {
				excerpt := pattern.Origin().Expand(position)
				return readTrailing(reader, &complement{
					pattern: pattern,
					excerpt: &excerpt,
					machine: nil,
				}, precedence)
			}
		}
		if reader.IsAt("|") {
			literal, err := readLiteral(reader)
			if err == nil {
				interval, err := readInterval(reader, literal)
				if err == nil {
					return readTrailing(reader, interval, precedence)
				}
				return nil, err
			}
			return nil, err
		}
		var buffer strings.Builder
		for reader.CanRead() {
			color := reader.Peek()
			if color >= 'a' && color <= 'z' || color >= 'A' && color <= 'Z' || color == '\'' || color == '_' {
				buffer.WriteRune(reader.Read())
				continue
			}
			break
		}
		if buffer.Len() > 0 {
			excerpt := reader.Excerpt(position)
			return readTrailing(reader, &reference{
				identifier: buffer.String(),
				excerpt:    &excerpt,
				machine:    nil,
			}, precedence)
		}
		return nil, source.Diagnose("Expected an identifier", reader.PointExcerpt())
	}
	return nil, source.Diagnose("Expected a pattern", reader.PointExcerpt())
}

func readInterval(reader *source.Reader, left *literal) (Pattern, error) {
	if reader.ReadSyntax("-") {
		if utf8.RuneCountInString(left.literal) == 1 {
			right, err := readLiteral(reader)
			if err == nil {
				if utf8.RuneCountInString(right.literal) == 1 {
					from, _ := utf8.DecodeRuneInString(left.literal)
					to, _ := utf8.DecodeLastRuneInString(right.literal)
					excerpt := left.Origin().Union(*right.Origin())
					return &interval{
						interval: [2]rune{from, to + 1},
						excerpt:  &excerpt,
						machine:  nil,
					}, nil
				}
				return nil, source.Diagnostic{
					Message: "The literal must be assignable to 'w' in order to appear in an interval",
					Excerpt: right.excerpt,
				}
			}
			return nil, err
		}
		return nil, source.Diagnostic{
			Message: "The literal must be assignable to 'w' in order to appear in an interval",
			Excerpt: left.excerpt,
		}
	}
	return left, nil
}

func readLiteral(reader *source.Reader) (*literal, error) {
	if reader.SkipWhitespace() {
		position := reader.Position()
		if reader.ReadOnly("|") {
			var buffer strings.Builder
			for reader.CanRead() {
				if reader.ReadOnly("|") {
					excerpt := reader.Excerpt(position)
					return &literal{
						literal: buffer.String(),
						excerpt: &excerpt,
						machine: nil,
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

func readTrailing(reader *source.Reader, pattern Pattern, precedence int) (Pattern, error) {
	if precedence >= PrecEll && reader.ReadSyntax("...") {
		position := reader.Position()
		excerpt := pattern.Origin().Expand(position)
		return readTrailing(reader, &ellipsis{
			pattern: pattern,
			excerpt: &excerpt,
			machine: nil,
		}, precedence)
	}
	if precedence >= PrecAnd && reader.ReadSyntax("&") {
		other, diagnostic := ReadPattern(reader, PrecAnd)
		if diagnostic == nil {
			excerpt := pattern.Origin().Union(*other.Origin())
			return readTrailing(reader, &intersection{
				left:    pattern,
				right:   other,
				excerpt: &excerpt,
				machine: nil,
			}, precedence)
		}
		return nil, diagnostic
	}
	if precedence >= PrecMul && reader.ReadSyntax("*") {
		factor, diagnostic := ReadPattern(reader, PrecMul)
		if diagnostic == nil {
			excerpt := pattern.Origin().Union(*factor.Origin())
			return readTrailing(reader, &product{
				left:    pattern,
				right:   factor,
				excerpt: &excerpt,
				machine: nil,
			}, precedence)
		}
		return nil, diagnostic
	}
	if precedence >= PrecAdd && reader.ReadSyntax("+") {
		summand, diagnostic := ReadPattern(reader, PrecAdd)
		if diagnostic == nil {
			excerpt := pattern.Origin().Union(*summand.Origin())
			return readTrailing(reader, &sum{
				left:    pattern,
				right:   summand,
				excerpt: &excerpt,
				machine: nil,
			}, precedence)
		}
		return nil, diagnostic
	}
	if precedence >= PrecAdd && reader.ReadSyntax("-") {
		subtrahend, diagnostic := ReadPattern(reader, PrecAdd)
		if diagnostic == nil {
			excerpt := pattern.Origin().Union(*subtrahend.Origin())
			return readTrailing(reader, &intersection{
				left: pattern,
				right: &complement{
					pattern: subtrahend,
					excerpt: nil,
					machine: nil,
				},
				excerpt: &excerpt,
				machine: nil,
			}, precedence)
		}
		return nil, diagnostic
	}
	return pattern, nil
}
