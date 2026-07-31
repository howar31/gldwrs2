package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderRawPassthrough(t *testing.T) {
	var b bytes.Buffer
	if err := Render(&b, []byte(`{"id":1}`), ModeRaw, "human"); err != nil {
		t.Fatal(err)
	}
	if b.String() != `{"id":1}` {
		t.Fatalf("raw = %q", b.String())
	}
}

func TestRenderJSONPretty(t *testing.T) {
	var b bytes.Buffer
	if err := Render(&b, []byte(`{"id":1}`), ModeJSON, "human"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "\n  \"id\": 1") {
		t.Fatalf("json = %q", b.String())
	}
}

func TestRenderConciseUsesString(t *testing.T) {
	var b bytes.Buffer
	if err := Render(&b, []byte(`{"id":1}`), ModeConcise, "red #FF0000"); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(b.String()) != "red #FF0000" {
		t.Fatalf("concise = %q", b.String())
	}
}

func TestRenderConciseFallsBackToJSON(t *testing.T) {
	var b bytes.Buffer
	if err := Render(&b, []byte(`{"id":1}`), ModeConcise, ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "\"id\": 1") {
		t.Fatalf("fallback = %q", b.String())
	}
}
