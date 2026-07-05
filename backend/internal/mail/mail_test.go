package mail

import (
	"strings"
	"testing"
)

func TestBuildLoginBody_greeting(t *testing.T) {
	// Player name provided
	body := buildLoginBody("Alice", "FunGame", "XYZ", "http://example.com")
	if !strings.Contains(body, "Hey Alice") {
		t.Errorf("expected body to contain casual greeting 'Hey Alice', got:\n%s", body)
	}

	// No player name
	bodyNoName := buildLoginBody("", "Game", "CODE", "http://example.com")
	if !strings.Contains(bodyNoName, "Hey") {
		t.Errorf("expected body to contain casual greeting 'Hey' when name is empty, got:\n%s", bodyNoName)
	}
}

func TestBuildLoginBody_stillContainsLink(t *testing.T) {
	body := buildLoginBody("Bob", "TestGame", "TG", "http://example.com")
	if !strings.Contains(body, "http://example.com") {
		t.Errorf("expected body to contain the login link, got:\n%s", body)
	}
}
