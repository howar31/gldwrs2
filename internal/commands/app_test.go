package commands

import (
	"bytes"
	"testing"
)

// TestClientExplicitMissingProfileErrors covers the folded-in fix: an
// explicit --profile that doesn't resolve in the store must surface an
// error, never fall back to an anonymous request.
func TestClientExplicitMissingProfileErrors(t *testing.T) {
	var out bytes.Buffer
	app := testApp(t, &out)
	app.Profile = "ghost"

	_, err := app.client()
	if err == nil {
		t.Fatal("client() with a nonexistent explicit --profile should return an error, got nil")
	}
}

// TestClientNoProfileIsAnonymous covers the case where no --profile was
// given and no default profile is configured: falling back to an anonymous
// (empty-token) client is fine for public endpoints.
func TestClientNoProfileIsAnonymous(t *testing.T) {
	var out bytes.Buffer
	app := testApp(t, &out)
	app.Profile = ""

	_, err := app.client()
	if err != nil {
		t.Fatalf("client() with no profile and no default configured should be anonymous, got err: %v", err)
	}
}
