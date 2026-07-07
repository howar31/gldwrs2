package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetAllPagesFollowsPageTotal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		w.Header().Set("X-Page-Total", "3")
		w.WriteHeader(200)
		switch page {
		case "0":
			fmt.Fprint(w, `[{"id":1},{"id":2}]`)
		case "1":
			fmt.Fprint(w, `[{"id":3},{"id":4}]`)
		case "2":
			fmt.Fprint(w, `[{"id":5}]`)
		default:
			t.Errorf("unexpected page %q", page)
		}
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	out, err := c.GetAllPages(context.Background(), "/v2/colors", nil)
	if err != nil {
		t.Fatalf("GetAllPages: %v", err)
	}
	if len(out) != 5 {
		t.Fatalf("len = %d, want 5", len(out))
	}
}
