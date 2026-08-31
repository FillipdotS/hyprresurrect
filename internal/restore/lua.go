package restore

import (
	"fmt"
	"strings"
)

// luaString renders s as a Lua double-quoted string literal. Window commands
// carry quotes and backslashes, and one bad literal is a syntax error that
// fails the whole eval rather than the one statement.
func luaString(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')

	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case 0:
			b.WriteString(`\0`)
		default:
			// Escaping by byte keeps multi-byte UTF-8 intact: every
			// continuation byte is >= 0x80 and passes through untouched.
			if c < 0x20 || c == 0x7f {
				fmt.Fprintf(&b, `\%d`, c)
				continue
			}
			b.WriteByte(c)
		}
	}

	b.WriteByte('"')

	return b.String()
}
