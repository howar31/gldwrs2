package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetByIDsChunksAt200AndMerges(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		ids := strings.Split(r.URL.Query().Get("ids"), ",")
		var b strings.Builder
		b.WriteString("[")
		for i, id := range ids {
			if i > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(&b, `{"id":%s}`, id)
		}
		b.WriteString("]")
		w.Write([]byte(b.String()))
	}))
	defer srv.Close()

	ids := make([]string, 0, 250)
	for i := 1; i <= 250; i++ {
		ids = append(ids, fmt.Sprintf("%d", i))
	}
	c := New(WithBaseURL(srv.URL))
	out, err := c.GetByIDs(context.Background(), "/v2/items", ids, nil)
	if err != nil {
		t.Fatalf("GetByIDs: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 chunked calls, got %d", calls)
	}
	if len(out) != 250 {
		t.Fatalf("merged len = %d, want 250", len(out))
	}
}

func TestChunkIDs(t *testing.T) {
	got := chunkIDs([]string{"a", "b", "c"}, 2)
	if len(got) != 2 || len(got[0]) != 2 || len(got[1]) != 1 {
		t.Fatalf("chunk = %v", got)
	}
}
