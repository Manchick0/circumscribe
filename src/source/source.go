package source

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

type Reader struct {
	source   Source
	position uint64
	column   uint32
	line     uint32
}

type Excerpt struct {
	source   Source
	interval [2]Position
}

type Position struct {
	absolute uint64
	column   uint32
	line     uint32
}

type Source struct {
	name    string
	content string
}

type Diagnostic struct {
	Message string
	Excerpt Excerpt
}

func New(name string, content string) Reader {
	return Reader{
		source: Source{
			name:    name,
			content: content,
		},
		position: 0,
		column:   1,
		line:     1,
	}
}

func (self *Reader) ReadMany(n int) string {
	var buffer strings.Builder
	for ; n > 0; n-- {
		buffer.WriteRune(self.Read())
	}
	return buffer.String()
}

func (self *Reader) Read() rune {
	c, len := utf8.DecodeRuneInString(self.source.content[self.position:])
	if c != utf8.RuneError {
		self.position += uint64(len)
		if c == '\n' {
			self.column = 1
			self.line++
			return c
		}
		self.column++
		return c
	}
	return utf8.RuneError
}

func (self Reader) Peek() rune {
	c, _ := utf8.DecodeRuneInString(self.source.content[self.position:])
	return c
}

func (self Reader) PointExcerpt() Excerpt {
	position := self.Position()
	if self.CanRead() {
		c, len := utf8.DecodeRuneInString(self.source.content[self.position:])
		if c == '\n' || c == '\r' {
			return Excerpt{
				source: self.source,
				interval: [2]Position{
					position,
					{
						absolute: self.position + uint64(len),
						line:     self.line + 1,
						column:   1,
					},
				},
			}
		}
		return Excerpt{
			source: self.source,
			interval: [2]Position{
				position,
				{
					absolute: self.position + uint64(len),
					column:   self.column + 1,
					line:     self.line,
				},
			},
		}
	}
	return Excerpt{
		source: self.source,
		interval: [2]Position{
			position,
			{
				absolute: self.position + 1,
				column:   self.column + 1,
				line:     self.line,
			},
		},
	}
}

func (self *Reader) ReadOnly(sequence string) bool {
	if self.IsAt(sequence) {
		self.ReadMany(utf8.RuneCountInString(sequence))
		return true
	}
	return false
}

func (self *Reader) ReadMeaningful(sequence string) bool {
	if self.SkipWhitespace() {
		return self.ReadOnly(sequence)
	}
	return false
}

func (self *Reader) IsAt(sequence string) bool {
	if self.CanRead() {
		for n := 0; n < len(sequence); n++ {
			if self.source.content[self.position+uint64(n)] == sequence[n] {
				continue
			}
			return false
		}
		return true
	}
	return false
}

func (self *Reader) SkipWhitespace() bool {
	for self.CanRead() {
		c := self.Peek()
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			self.Read()
			continue
		}
		if c == '#' {
			for self.CanRead() {
				c := self.Peek()
				if c == '\n' || c == '\r' {
					return self.SkipWhitespace()
				}
				self.Read()
			}
			return false
		}
		return true
	}
	return false
}

func (self Reader) CanRead() bool {
	return self.position < uint64(len(self.source.content))
}

func (self Reader) Excerpt(position Position) Excerpt {
	return Excerpt{
		source: self.source,
		interval: [2]Position{
			position,
			self.Position(),
		},
	}
}

func (self Reader) String() string {
	return self.source.content[self.position:]
}

func (self Reader) Position() Position {
	return Position{
		absolute: self.position,
		column:   self.column,
		line:     self.line,
	}
}

func (self Position) max(other Position) Position {
	if self.absolute > other.absolute {
		return self
	}
	return other
}

func (self Position) min(other Position) Position {
	if self.absolute < other.absolute {
		return self
	}
	return other
}

func (self Excerpt) Render() string {
	var buffer strings.Builder
	var underline strings.Builder

	gutter := func(line int) string {
		length := max(
			int(math.Log10(float64(self.interval[0].line)))+1,
			int(math.Log10(float64(self.interval[1].line)))+1,
		)
		if line == 0 {
			return fmt.Sprintf(" %s | ", strings.Repeat(" ", length))
		}
		return fmt.Sprintf(" %*d | ", length, line)
	}
	begin := max(strings.LastIndex(self.source.content[:self.interval[0].absolute], "\n")+1, 0)
	end := func() int {
		if int(self.interval[1].absolute) < len(self.source.content) {
			position := strings.IndexRune(self.source.content[self.interval[1].absolute:], '\n')
			if position == -1 {
				return len(self.source.content)
			}
			return int(self.interval[1].absolute) + position
		}
		return len(self.source.content)
	}()

	line := int(self.interval[0].line)
	buffer.WriteString(gutter(line))
	underline.WriteString(gutter(0))
	line++
	for n := begin; n < end; n++ {
		c, l := utf8.DecodeRuneInString(self.source.content[n:])
		if c == '\n' || c == '\r' {
			buffer.WriteRune('\n')
			buffer.WriteString(underline.String())
			buffer.WriteRune('\n')
			buffer.WriteString(gutter(line))
			underline.Reset()
			underline.WriteString(gutter(0))
			line++
			continue
		}
		buffer.WriteRune(c)
		underline.WriteRune(func() rune {
			if n >= int(self.interval[0].absolute) && n < int(self.interval[1].absolute) {
				return '^'
			}
			return ' '
		}())
		n += l - 1
	}
	buffer.WriteRune('\n')
	buffer.WriteString(underline.String())
	return buffer.String()
}

func (self Excerpt) String() string {
	return fmt.Sprintf("(%s | %d:%d-%d:%d)", self.source.name, self.interval[0].line, self.interval[0].column, self.interval[1].line, self.interval[1].column)
}

func (self Diagnostic) Error() string {
	return fmt.Sprintf("%s\n%s %s", self.Excerpt.Render(), self.Excerpt, self.Message)
}

func (self Excerpt) Union(interval [2]Position) Excerpt {
	return Excerpt{
		source: self.source,
		interval: [2]Position{
			self.interval[0].min(interval[0]),
			self.interval[1].max(interval[1]),
		},
	}
}

func (self Excerpt) Expand(position Position) Excerpt {
	return Excerpt{
		source: self.source,
		interval: [2]Position{
			self.interval[0].min(position),
			self.interval[1].max(position),
		},
	}
}
