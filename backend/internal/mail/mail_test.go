package mail

import (
	"strings"
	"testing"
)

func TestBuildLoginBodyGreeting(t *testing.T) {
	tests := []struct {
		name             string
		playerName       string
		expectedGreeting string
	}{
		{name: "empty name", playerName: "", expectedGreeting: "Hi"},
		{name: "named player", playerName: "Alice", expectedGreeting: "Hi Alice"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := buildLoginBody(tt.playerName, "Fun Quiz", "ABC123", "http://example.com/login")

			if !strings.Contains(body, tt.expectedGreeting) {
				t.Errorf("expected greeting %q in body, got:\n%s", tt.expectedGreeting, body)
			}
			if strings.Contains(body, "Hey") {
				t.Errorf("body should NOT contain casual 'Hey' greeting, got:\n%s", body)
			}
		})
	}
}
