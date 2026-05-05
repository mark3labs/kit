package session

import (
	"strings"
	"testing"
)

// TestEncodeCwdForDir verifies the working-directory → session-directory
// name encoding strips characters that are illegal on Windows (notably the
// drive-letter colon, see issue #18) while preserving the previous output
// for the typical Unix paths.
func TestEncodeCwdForDir(t *testing.T) {
	tests := []struct {
		name string
		cwd  string
		want string
	}{
		{
			name: "unix absolute path",
			cwd:  "/home/user/proj",
			want: "home--user--proj",
		},
		{
			name: "unix relative path",
			cwd:  "proj/sub",
			want: "proj--sub",
		},
		{
			name: "windows drive root",
			cwd:  `C:\test`,
			want: "C--test",
		},
		{
			name: "windows nested path",
			cwd:  `C:\Users\User\code`,
			want: "C--Users--User--code",
		},
		{
			name: "windows secondary drive",
			cwd:  `S:\work\repo`,
			want: "S--work--repo",
		},
		{
			name: "windows mixed separators",
			cwd:  `C:\Users/User\code`,
			want: "C--Users--User--code",
		},
		{
			name: "windows other illegal chars stripped",
			cwd:  `C:\a<b>c|d?e*f"g`,
			want: "C--abcdefg",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := encodeCwdForDir(tc.cwd)
			if got != tc.want {
				t.Errorf("encodeCwdForDir(%q) = %q, want %q", tc.cwd, got, tc.want)
			}
			// Encoded directory must never contain characters that are
			// illegal in Windows directory names.
			for _, bad := range []string{":", "<", ">", "\"", "|", "?", "*", "\\", "/"} {
				if strings.Contains(got, bad) {
					t.Errorf("encodeCwdForDir(%q) = %q contains illegal char %q", tc.cwd, got, bad)
				}
			}
		})
	}
}
