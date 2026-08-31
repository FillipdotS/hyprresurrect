package restore

import "testing"

func TestLuaString(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", `foot`, `"foot"`},
		{"spaces", `foot -e cliamp`, `"foot -e cliamp"`},
		{"double quote", `say "hi"`, `"say \"hi\""`},
		{"backslash", `C:\path`, `"C:\\path"`},
		{"both", `a\"b`, `"a\\\"b"`},
		{"newline", "a\nb", `"a\nb"`},
		{"carriage return", "a\rb", `"a\rb"`},
		{"tab is a control byte", "a\tb", `"a\009b"`},
		{"nul", "a\x00b", `"a\000b"`},
		{"del", "a\x7fb", `"a\127b"`},
		// Lua reads \ddd as up to three digits, so a short escape swallows a
		// digit that follows it: "\0" + "5" is the single byte 5, not two bytes.
		{"nul before a digit", "a\x005b", `"a\0005b"`},
		{"control byte before a digit", "a\x012b", `"a\0012b"`},
		{"tab before a digit", "a\t9b", `"a\0099b"`},
		{"utf8 passes through", "café ☕", `"café ☕"`},
		{"empty", ``, `""`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := luaString(tt.in); got != tt.want {
				t.Errorf("luaString(%q) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}
