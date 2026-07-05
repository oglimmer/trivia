package mail

import (
	"strings"
	"testing"
)

func TestBuildLoginBody_Greeting(t *testing.T) {
	tests := []struct {
		name       string
		playerName string
		wantGreet  string
	}{
		{"empty", "", "Hey"},
		{"named", "Alice", "Hey Alice"},
		{"name with space", "Bob Jones", "Hey Bob Jones"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			body := buildLoginBody(tc.playerName, "Test Game", "XYZ12", "https://example.com/link")
			if !strings.Contains(body, tc.wantGreet) {
				t.Errorf("greeting missing %q in body:\n%s", tc.wantGreet, body)
			}
		})
	}
}
