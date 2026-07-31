package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestGetSendsSchemaLangAndToken(t *testing.T) {
	var gotPath, gotAuth, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.String()
		gotAuth = r.Header.Get("Authorization")
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(200)
		w.Write([]byte(`{"id":1}`))
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL), WithToken("KEY"), WithLang("de"), WithUserAgent("gw2-test"))
	raw, err := c.Get(context.Background(), "/v2/colors", url.Values{"ids": {"1"}})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(raw) != `{"id":1}` {
		t.Fatalf("body = %s", raw)
	}
	if gotAuth != "Bearer KEY" {
		t.Fatalf("auth = %q", gotAuth)
	}
	if gotUA != "gw2-test" {
		t.Fatalf("ua = %q", gotUA)
	}
	u, _ := url.Parse(gotPath)
	q := u.Query()
	if q.Get("v") != SchemaVersion {
		t.Fatalf("schema v = %q", q.Get("v"))
	}
	if q.Get("lang") != "de" {
		t.Fatalf("lang = %q", q.Get("lang"))
	}
	if q.Get("ids") != "1" {
		t.Fatalf("ids = %q", q.Get("ids"))
	}
	_ = json.RawMessage(raw)
}
