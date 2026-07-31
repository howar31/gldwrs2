package api

import (
	"errors"
	"testing"
)

func TestExitCodeMapping(t *testing.T) {
	cases := []struct {
		code int
		want int
	}{
		{403, 3}, {404, 4}, {429, 5}, {500, 1}, {503, 1},
	}
	for _, c := range cases {
		err := apiErrorFrom(c.code, []byte(`{"text":"boom"}`))
		if got := ExitCode(err); got != c.want {
			t.Fatalf("status %d -> exit %d, want %d", c.code, got, c.want)
		}
	}
	if ExitCode(nil) != 0 {
		t.Fatal("nil -> 0")
	}
	if ExitCode(errors.New("plain")) != 1 {
		t.Fatal("plain -> 1")
	}
}

func TestAuthHintOnInvalidKey(t *testing.T) {
	err := apiErrorFrom(403, []byte(`{"text":"invalid key"}`))
	var ae *APIError
	if !errors.As(err, &ae) || ae.Hint == "" {
		t.Fatalf("expected auth hint, got %+v", err)
	}
}
