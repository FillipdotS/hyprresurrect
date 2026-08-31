package restore

import "strings"

// shellSafe are the characters a POSIX shell leaves alone unquoted.
const shellSafe = "abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
	"0123456789" + "@%+=:,./-_"

// hl.exec_cmd takes one string, not an argv, and runs it through a shell
// (verified: `$(id -u)` expands, `;` starts a second command). Joining on spaces
// alone would lose the boundaries between arguments, so an argv element
// containing a space or `;` becomes two arguments or two commands. Quoting is
// what keeps the list the same length on the other side.

// shellCommand joins argv into the single command string hl.exec_cmd expects,
// quoting each argument so the shell reproduces it exactly.
func shellCommand(argv []string) string {
	quoted := make([]string, len(argv))
	for i, arg := range argv {
		quoted[i] = shellQuote(arg)
	}

	return strings.Join(quoted, " ")
}

func shellQuote(arg string) string {
	if arg == "" {
		return "''"
	}

	if strings.IndexFunc(arg, func(r rune) bool {
		return !strings.ContainsRune(shellSafe, r)
	}) < 0 {
		return arg
	}

	// A single quote cannot appear inside single quotes at all: close the
	// quoting, emit an escaped quote, reopen.
	return "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
}
