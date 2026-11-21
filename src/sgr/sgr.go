package sgr

import "fmt"

const (
	Dragonfruit = 0xee80ff
	Lemon       = 0xffe18d
	Cranberry   = 0xff729d
	Pebble      = 0x454545
)

const Terminate = "\033[39m"

// Surround the given 'source' in the
// appropritate escape sequence.
// ---
func Escape(source string, color int) string {
	return fmt.Sprintf("%s%s%s", Sequence(color), source, Terminate)
}

// Compute the begin escape sequence
// for the given color.
// ---
func Sequence(color int) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm", color>>16&0xFF, color>>8&0xFF, color&0xFF)
}
