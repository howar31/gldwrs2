package resolve

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/howar31/gldwrs2/internal/api"
)

// TestGuildIDPassesThroughGUID verifies a GUID-shaped input is returned
// unchanged with no HTTP call at all -- the httptest server fails the test
// if it's ever hit, proving GuildID short-circuits before any request.
func TestGuildIDPassesThroughGUID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected HTTP call for a GUID passthrough: %s", r.URL.String())
	}))
	defer srv.Close()

	c := api.New(api.WithBaseURL(srv.URL))
	const guid = "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE"
	got, err := GuildID(context.Background(), c, guid)
	if err != nil {
		t.Fatalf("GuildID: %v", err)
	}
	if got != guid {
		t.Fatalf("GuildID(%q) = %q, want it unchanged", guid, got)
	}
}

// TestGuildIDResolvesName verifies a non-GUID input is resolved via
// GET /v2/guild/search?name=<name>, returning the first id in the response.
func TestGuildIDResolvesName(t *testing.T) {
	var gotPath, gotName string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotName = r.URL.Query().Get("name")
		w.Write([]byte(`["GUID-1"]`))
	}))
	defer srv.Close()

	c := api.New(api.WithBaseURL(srv.URL))
	got, err := GuildID(context.Background(), c, "Foo")
	if err != nil {
		t.Fatalf("GuildID: %v", err)
	}
	if got != "GUID-1" {
		t.Fatalf("GuildID(%q) = %q, want %q", "Foo", got, "GUID-1")
	}
	if gotPath != "/v2/guild/search" {
		t.Fatalf("request path = %q, want /v2/guild/search", gotPath)
	}
	if gotName != "Foo" {
		t.Fatalf("name query = %q, want %q", gotName, "Foo")
	}
}

// TestGuildIDNoMatchErrors verifies an empty search result surfaces a
// helpful "no guild found" error rather than an empty id or a panic.
func TestGuildIDNoMatchErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := api.New(api.WithBaseURL(srv.URL))
	_, err := GuildID(context.Background(), c, "Nobody's Guild")
	if err == nil {
		t.Fatal("GuildID with no search results should error")
	}
}
