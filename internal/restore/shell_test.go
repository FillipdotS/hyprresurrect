package restore

import "testing"

func TestShellQuote(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		// The argument that makes this necessary: joined unquoted, the shell
		// would read the semicolon as "next command" and split one argv element
		// into two commands. See TestShellCommand.
		{"real example", `make; notify-send done`, `'make; notify-send done'`},
		{"bare word", `foot`, `foot`},
		{"path", `/usr/bin/ghostty`, `/usr/bin/ghostty`},
		{"flag", `--type=renderer`, `--type=renderer`},
		{"space", `my file`, `'my file'`},
		{"single quote", `it's`, `'it'\''s'`},
		{"only a quote", `'`, `''\'''`},
		{"double quote", `say "hi"`, `'say "hi"'`},
		{"dollar", `$HOME`, `'$HOME'`},
		{"backtick", "a`b", "'a`b'"},
		{"semicolon", `a;ls /`, `'a;ls /'`},
		{"glob", `*.go`, `'*.go'`},
		{"tilde", `~/notes`, `'~/notes'`},
		{"empty", ``, `''`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shellQuote(tt.in); got != tt.want {
				t.Errorf("shellQuote(%q) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}

func TestShellCommand(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{"single", []string{"aseprite"}, `aseprite`},
		{"args", []string{"foot", "-e", "cliamp"}, `foot -e cliamp`},
		{
			"quoting per argument",
			[]string{"/bin/sh", "-c", "echo hi there"},
			`/bin/sh -c 'echo hi there'`,
		},
		{
			// A terminal running a build, as /proc reports it. Five argv
			// elements in, and the quoting is what keeps them five: without it
			// hyprland's shell runs `foot -e sh -c make` and then a separate
			// `notify-send done`.
			"real example",
			[]string{"foot", "-e", "sh", "-c", "make; notify-send done"},
			`foot -e sh -c 'make; notify-send done'`,
		},
		{"empty argv", nil, ``},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shellCommand(tt.in); got != tt.want {
				t.Errorf("shellCommand(%q) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}
