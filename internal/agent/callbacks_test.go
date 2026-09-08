package agent

import (
	"reflect"
	"testing"
)

// bothPathCallbacks lists the GenerateCallbacks fields that are honoured by
// both the streaming and non-streaming paths, and therefore must NOT force
// the streaming path on their own. Every other field must be covered by
// HasAnyCallback.
var bothPathCallbacks = map[string]bool{
	"OnResponse":       true,
	"OnToolOutput":     true,
	"OnPasswordPrompt": true,
}

// TestHasAnyCallbackCoversAllFields sets each GenerateCallbacks field in
// isolation and checks HasAnyCallback agrees with the allowlist above. It
// fails when a new callback field is added to the struct without updating
// HasAnyCallback, which would otherwise silently route callers to the
// non-streaming path where the new callback never fires.
func TestHasAnyCallbackCoversAllFields(t *testing.T) {
	typ := reflect.TypeFor[GenerateCallbacks]()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Type.Kind() != reflect.Func {
			t.Fatalf("field %s is %s, expected a func type", field.Name, field.Type.Kind())
		}

		var cb GenerateCallbacks
		v := reflect.ValueOf(&cb).Elem()
		// A non-nil func of the right signature; it is never called.
		v.Field(i).Set(reflect.MakeFunc(field.Type, func(args []reflect.Value) []reflect.Value {
			out := make([]reflect.Value, field.Type.NumOut())
			for j := range out {
				out[j] = reflect.Zero(field.Type.Out(j))
			}
			return out
		}))

		got := cb.HasAnyCallback()
		want := !bothPathCallbacks[field.Name]
		if got != want {
			t.Errorf("HasAnyCallback with only %s set = %v, want %v (update HasAnyCallback or bothPathCallbacks)", field.Name, got, want)
		}
	}
}

func TestHasAnyCallbackZeroValue(t *testing.T) {
	if (GenerateCallbacks{}).HasAnyCallback() {
		t.Fatal("zero GenerateCallbacks must report no callbacks")
	}
}
