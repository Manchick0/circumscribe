package pattern

import (
	"circumscribe/iters"
	"circumscribe/source"
	"fmt"
	"iter"
	"maps"
	"slices"
	"strings"
)

type set[T comparable] = map[T]bool

type interval = [2]int32

type Pattern struct {
	Source        string
	compiled      nFA
	deterministic dFA
	determined    bool
}

type Branch[T uint16 | []uint16] struct {
	w interval
	q T
}

type nFA struct {
	initial    uint16
	accept     uint16
	epsilon    [][]uint16
	transition [][]Branch[[]uint16]
}

type dFA struct {
	initial    uint16
	accept     []uint16
	transition [][]Branch[uint16]
}

var (
	StdBool = Sum(Literal("true"), Literal("false"))
	StdAny  = Kleene(Any())
)

func (self Pattern) IsAssignable(other Pattern) bool {
	other.determinize()
	closure := self.compiled.closure(self.compiled.initial)
	dejaVu := set[[2]uint16]{}
	buffer := [][2]uint16{}
	for q := range closure {
		dejaVu[[2]uint16{q, other.deterministic.initial}] = true
		buffer = append(buffer, [2]uint16{q, other.deterministic.initial})
	}
	for len(buffer) > 0 {
		candidate := buffer[len(buffer)-1]
		buffer = buffer[:len(buffer)-1]
		if self.compiled.accept == candidate[0] && !slices.Contains(other.deterministic.accept, candidate[1]) {
			return false
		}
		left := self.compiled.transition[candidate[0]]
		right := other.deterministic.transition[candidate[1]]
		S := disjoin(slices.Collect(iters.Fmap(slices.Values(left), func(branch Branch[[]uint16]) interval {
			return branch.w
		})))
		for _, w := range S {
			Q := slices.Collect(iters.Fmap(iters.Filter(slices.Values(left), func(branch Branch[[]uint16]) bool {
				return branch.w[0] <= w[0] && w[1] <= branch.w[1]
			}), func(branch Branch[[]uint16]) []uint16 {
				return branch.q
			}))
			if slices.ContainsFunc(right, func(q Branch[uint16]) bool {
				return q.w[0] <= w[0] && w[1] <= q.w[1]
			}) {
				for _, branch := range right {
					if branch.w[0] <= w[0] && w[1] <= branch.w[1] {
						for _, q := range Q {
							for _, qe := range q {
								closure := self.compiled.closure(qe)
								for q := range closure {
									if dejaVu[[2]uint16{q, branch.q}] {
										continue
									}
									buffer = append(buffer, [2]uint16{q, branch.q})
									dejaVu[[2]uint16{q, branch.q}] = true
								}
							}
						}
					}
				}
				continue
			}
			return false
		}
	}
	return true
}

// Construct a Literal pattern.
// ---
// Computes a pattern that represents
// the single given `Literal`.
func Literal(literal string) Pattern {
	epsilon := make([][]uint16, len(literal)+1)
	transition := make([][]Branch[[]uint16], len(literal)+1)
	for n, c := range literal {
		transition[n] = []Branch[[]uint16]{
			{
				w: [2]int32{c, c + 1},
				q: []uint16{uint16(n + 1)},
			},
		}
	}
	return Pattern{
		Source: fmt.Sprintf("|%s|", literal),
		compiled: nFA{
			initial:    0,
			accept:     uint16(len(literal)),
			epsilon:    epsilon,
			transition: transition,
		},
		deterministic: dFA{},
		determined:    false,
	}
}

func Any() Pattern {
	return Pattern{
		Source: "any",
		compiled: nFA{
			initial: 0,
			accept:  1,
			epsilon: [][]uint16{{}, {}},
			transition: [][]Branch[[]uint16]{
				{
					{
						w: [2]int32{0x0, 0x110000},
						q: []uint16{1},
					},
				},
				{},
			},
		},
		deterministic: dFA{},
		determined:    false,
	}
}

func Sum(left Pattern, right Pattern) Pattern {
	l, r := left.compiled, right.compiled.shift(len(left.compiled.transition))
	buffer := nFA{
		initial:    uint16(len(l.transition) + len(r.transition)),
		accept:     uint16(len(l.transition) + len(r.transition) + 1),
		epsilon:    slices.Concat(l.epsilon, r.epsilon, [][]uint16{{l.initial, r.initial}, {}}),
		transition: slices.Concat(l.transition, r.transition, [][]Branch[[]uint16]{{}, {}}),
	}
	buffer.epsilon[l.accept] = append(buffer.epsilon[l.accept], uint16(len(l.transition)+len(r.transition)+1))
	buffer.epsilon[r.accept] = append(buffer.epsilon[r.accept], uint16(len(l.transition)+len(r.transition)+1))
	return Pattern{
		Source:        fmt.Sprintf("%s + %s", left.Source, right.Source),
		compiled:      buffer,
		deterministic: dFA{},
		determined:    false,
	}
}

func Product(left Pattern, right Pattern) Pattern {
	l, r := left.compiled, right.compiled.shift(len(left.compiled.transition))
	buffer := nFA{
		initial:    l.initial,
		accept:     r.accept,
		epsilon:    slices.Concat(l.epsilon, r.epsilon),
		transition: slices.Concat(l.transition, r.transition),
	}
	buffer.transition[l.accept] = buffer.transition[r.initial]
	buffer.epsilon[l.accept] = buffer.epsilon[r.initial]
	return Pattern{
		Source:        fmt.Sprintf("%s * %s", left.Source, right.Source),
		compiled:      buffer,
		deterministic: dFA{},
		determined:    false,
	}
}

func Kleene(pattern Pattern) Pattern {
	compiled := &pattern.compiled
	compiled.epsilon[compiled.accept] = append(compiled.epsilon[compiled.accept], compiled.initial)
	compiled.epsilon[compiled.accept] = append(compiled.epsilon[compiled.accept], uint16(len(compiled.transition)+1))
	return Pattern{
		Source: fmt.Sprintf("%s...", pattern.Source),
		compiled: nFA{
			initial:    uint16(len(compiled.transition)),
			accept:     uint16(len(compiled.transition) + 1),
			epsilon:    slices.Concat(compiled.epsilon, [][]uint16{{compiled.initial, uint16(len(compiled.transition) + 1)}, {}}),
			transition: slices.Concat(compiled.transition, [][]Branch[[]uint16]{{}, {}}),
		},
		deterministic: dFA{},
		determined:    false,
	}
}

func (self *Pattern) determinize() bool {
	if self.determined {
		return false
	}
	isAccepting := func(Q set[uint16]) bool {
		for q := range Q {
			if q == self.compiled.accept {
				return true
			}
		}
		return false
	}
	closure := self.compiled.closure(self.compiled.initial)
	dictionary := map[string]uint16{key(closure): 0}
	transition := [][]Branch[uint16]{{}}
	accept := make([]uint16, 0)
	buffer := []set[uint16]{closure}
	if isAccepting(closure) {
		accept = append(accept, 0)
	}
	for len(buffer) > 0 {
		Q := buffer[len(buffer)-1]
		buffer = buffer[:len(buffer)-1]
		position := dictionary[key(Q)]
		S := self.compiled.alphabet(Q)
		for _, w := range S {
			dq := func() set[uint16] {
				buffer := make(set[uint16])
				for q := range Q {
					for _, branch := range self.compiled.transition[q] {
						if branch.w[0] <= w[0] && w[1] <= branch.w[1] {
							for _, qqe := range branch.q {
								closure := self.compiled.closure(qqe)
								maps.Copy(buffer, closure)
							}
						}
					}
				}
				return buffer
			}()
			if len(dq) > 0 {
				transition[position] = append(transition[position], Branch[uint16]{
					w: w,
					q: func() uint16 {
						name := key(dq)
						position, ok := dictionary[name]
						if !ok {
							var position uint16 = uint16(len(transition))
							if isAccepting(dq) {
								accept = append(accept, position)
							}
							transition = append(transition, []Branch[uint16]{})
							dictionary[name] = position
							buffer = append(buffer, dq)
							return position
						}
						return position
					}(),
				})
			}
		}
	}
	self.deterministic = dFA{
		initial:    0,
		accept:     accept,
		transition: transition,
	}
	self.determined = true
	return true
}

func (self nFA) alphabet(Q set[uint16]) []interval {
	var buffer []interval
	for q := range Q {
		t := self.transition[q]
		disjoint := disjoin(slices.Collect(iters.Fmap(slices.Values(t), func(branch Branch[[]uint16]) interval {
			return branch.w
		})))
		for _, q := range disjoint {
			buffer = append(buffer, q)
		}
	}
	return buffer
}

func (self nFA) String() string {
	return fmt.Sprintf("%v", self.transition)
}

func (self dFA) String() string {
	return fmt.Sprintf("initial = %d accept = %v transition = %v", self.initial, self.accept, self.transition)
}

// Compute the key for the given `Q`.
// ---
func key(Q set[uint16]) string {
	var buffer strings.Builder
	values := slices.Collect(maps.Keys(Q))
	slices.Sort(values)
	for n, q := range values {
		if n > 0 {
			buffer.WriteRune(':')
		}
		fmt.Fprintf(&buffer, "%d", q)
	}
	return buffer.String()
}

func (self nFA) shift(n int) nFA {
	return nFA{
		initial: self.initial + uint16(n),
		accept:  self.accept + uint16(n),
		epsilon: slices.Collect(iters.Fmap(slices.Values(self.epsilon), func(xs []uint16) []uint16 {
			return slices.Collect(iters.Fmap(slices.Values(xs), func(q uint16) uint16 {
				return q + uint16(n)
			}))
		})),
		transition: slices.Collect(iters.Fmap(slices.Values(self.transition), func(xs []Branch[[]uint16]) []Branch[[]uint16] {
			return slices.Collect(iters.Fmap(slices.Values(xs), func(t Branch[[]uint16]) Branch[[]uint16] {
				return Branch[[]uint16]{
					w: t.w,
					q: slices.Collect(iters.Fmap(slices.Values(t.q), func(q uint16) uint16 {
						return q + uint16(n)
					})),
				}
			}))
		})),
	}
}

func (self nFA) closure(q uint16) map[uint16]bool {
	buffer := make(map[uint16]bool)
	for _, q := range self.epsilon[q] {
		state := self.closure(q)
		for q := range state {
			buffer[q] = true
		}
	}
	buffer[q] = true
	return buffer
}

func disjoin(candidates []interval) []interval {
	entries := slices.Collect(iters.Bind(slices.Values(candidates), func(candidate interval) iter.Seq[[2]int32] {
		return slices.Values([][2]int32{
			{candidate[0], 1},
			{candidate[1], -1},
		})
	}))
	slices.SortFunc(entries, func(a [2]int32, b [2]int32) int {
		if a[0] == b[0] {
			return int(a[1] - b[1])
		}
		return int(a[0] - b[0])
	})
	buffer := make([]interval, 0, len(candidates))
	var open int32 = 0
	for i := 0; i < len(entries)-1; i++ {
		left := entries[i]
		right := entries[i+1]
		open += left[1]
		if open > 0 && left[0] != right[0] {
			buffer = append(buffer, [2]int32{left[0], right[0]})
		}
	}
	return buffer
}

const (
	Postfix = iota
	And
	Or
)

func ReadPattern(reader *source.Reader, precedence int) (Pattern, error) {
	if reader.ReadMeaningful("(") {
		expression, diagnostic := ReadPattern(reader, Postfix)
		if diagnostic == nil {
			if reader.ReadMeaningful(")") {
				return readTrailing(reader, expression, precedence)
			}
			return Pattern{}, source.Diagnostic{
				Message: "Expected ')'",
				Excerpt: reader.PointExcerpt(),
			}
		}
		return Pattern{}, diagnostic
	}
	if reader.ReadMeaningful("|") {
		var buffer strings.Builder
		for reader.CanRead() {
			if reader.ReadOnly("|") {
				return readTrailing(reader, Literal(buffer.String()), precedence)
			}
			buffer.WriteRune(reader.Read())
		}
		return Pattern{}, source.Diagnostic{
			Message: "Expected '|'",
			Excerpt: reader.PointExcerpt(),
		}
	}
	if reader.ReadMeaningful("any") {
		return readTrailing(reader, Any(), precedence)
	}
	return Pattern{}, source.Diagnostic{
		Message: "Expected a pattern",
		Excerpt: reader.PointExcerpt(),
	}
}

func readTrailing(reader *source.Reader, pattern Pattern, precedence int) (Pattern, error) {
	if precedence >= 0 && reader.ReadMeaningful("...") {
		return readTrailing(reader, Kleene(pattern), precedence)
	}
	if precedence >= And && reader.ReadMeaningful("*") {
		factor, diagnostic := ReadPattern(reader, And)
		if diagnostic == nil {
			return readTrailing(reader, Product(pattern, factor), precedence)
		}
		return Pattern{}, nil
	}
	if precedence >= Or && reader.ReadMeaningful("+") {
		summand, diagnostic := ReadPattern(reader, Or)
		if diagnostic == nil {
			return readTrailing(reader, Sum(pattern, summand), precedence)
		}
		return Pattern{}, nil
	}
	return pattern, nil
}
