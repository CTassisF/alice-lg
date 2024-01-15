package decoders

import (
	"testing"
)

func TestMaybePrefix(t *testing.T) {
	expected := []struct {
		string
		bool
	}{
		{"10.0.0", true},
		{"23.42.11.42/23", true},
		{"fa42:2342::/32", true},
		{"1.", true},
		{"200", false},
		{"200.", true},
		{"2001:", true},
		{"A", false},
		{"A b", false},
		{"23 Foo", false},
		{"Nordfoo", false},
		{"122.beef:", true}, // sloppy
		{"122.beef:", true}, // very
		{"122:beef", true},  // sloppy.
	}

	for _, e := range expected {
		if MaybePrefix(e.string) != e.bool {
			t.Error("Expected", e.string, "to be prefix:", e.bool)
		}
	}
}
