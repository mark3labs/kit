package extensions

import (
	"reflect"
	"testing"
)

func TestParseToolArgs(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want map[string]any
	}{
		{name: "empty", raw: "", want: nil},
		{name: "whitespace only", raw: "  \n\t", want: nil},
		{name: "malformed json", raw: "{not json", want: nil},
		{name: "json array is not a map", raw: `[1,2]`, want: nil},
		{name: "json null yields nil map", raw: `null`, want: nil},
		{name: "empty object", raw: `{}`, want: map[string]any{}},
		{
			name: "object with values",
			raw:  `{"path":"a.go","count":2}`,
			want: map[string]any{"path": "a.go", "count": float64(2)},
		},
		{
			name: "leading whitespace is tolerated",
			raw:  "  {\"k\":\"v\"}",
			want: map[string]any{"k": "v"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseToolArgs(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseToolArgs(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}
