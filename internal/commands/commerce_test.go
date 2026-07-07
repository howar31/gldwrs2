package commands

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/howar31/gldwrs2/internal/api"
	"github.com/howar31/gldwrs2/internal/output"
	"github.com/spf13/cobra"
)

// TestCommercePrices exercises `commerce prices <id>`, public, GetByIDs.
// It also exercises `commerce listings` in a subtest (same lookup shape),
// so both leaves are covered without inflating the brief's 6-test list.
func TestCommercePrices(t *testing.T) {
	var gotPath, gotIDs string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotIDs = r.URL.Query().Get("ids")
		w.Write([]byte(`[{"id":24,"buys":{"unit_price":100,"quantity":5},"sells":{"unit_price":156,"quantity":3}}]`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newCommerceCmd(app))
	root.SetArgs([]string{"commerce", "prices", "24"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPath != "/v2/commerce/prices" {
		t.Fatalf("path = %q, want /v2/commerce/prices", gotPath)
	}
	if gotIDs != "24" {
		t.Fatalf("ids query = %q, want %q", gotIDs, "24")
	}
	if !strings.Contains(out.String(), "1s 56c") {
		t.Fatalf("output = %q, want a formatted coin string", out.String())
	}

	t.Run("listings", func(t *testing.T) {
		var gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.Write([]byte(`[{"id":24,"buys":[{"unit_price":100,"quantity":5,"listings":1}],"sells":[{"unit_price":156,"quantity":3,"listings":1}]}]`))
		}))
		defer srv.Close()

		var out bytes.Buffer
		app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
		root := &cobra.Command{Use: "gw2"}
		root.AddCommand(newCommerceCmd(app))
		root.SetArgs([]string{"commerce", "listings", "24"})
		if err := root.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
		if gotPath != "/v2/commerce/listings" {
			t.Fatalf("path = %q, want /v2/commerce/listings", gotPath)
		}
		if !strings.Contains(out.String(), "1s 56c") {
			t.Fatalf("output = %q, want a formatted coin string", out.String())
		}
	})
}

// TestCommerceExchangeCoins exercises `commerce exchange coins --quantity`,
// public. It also exercises `commerce exchange gems` in a subtest (the
// symmetric endpoint), so both leaves are covered.
func TestCommerceExchangeCoins(t *testing.T) {
	var gotPath, gotQuantity string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuantity = r.URL.Query().Get("quantity")
		w.Write([]byte(`{"coins_per_gem":3500,"quantity":2}`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newCommerceCmd(app))
	root.SetArgs([]string{"commerce", "exchange", "coins", "--quantity", "10000"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPath != "/v2/commerce/exchange/coins" {
		t.Fatalf("path = %q, want /v2/commerce/exchange/coins", gotPath)
	}
	if gotQuantity != "10000" {
		t.Fatalf("quantity query = %q, want %q", gotQuantity, "10000")
	}
	if !strings.Contains(out.String(), "3500") {
		t.Fatalf("output = %q, want it to show the rate", out.String())
	}

	t.Run("gems", func(t *testing.T) {
		var gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.Write([]byte(`{"coins_per_gem":3500,"quantity":350000}`))
		}))
		defer srv.Close()

		var out bytes.Buffer
		app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
		root := &cobra.Command{Use: "gw2"}
		root.AddCommand(newCommerceCmd(app))
		root.SetArgs([]string{"commerce", "exchange", "gems", "--quantity", "100"})
		if err := root.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
		if gotPath != "/v2/commerce/exchange/gems" {
			t.Fatalf("path = %q, want /v2/commerce/exchange/gems", gotPath)
		}
		if !strings.Contains(out.String(), "3500") {
			t.Fatalf("output = %q, want it to show the rate", out.String())
		}
	})
}

// TestCommerceExchangeRequiresQuantity verifies --quantity is mandatory
// (int > 0): omitting it must error without ever reaching the server.
func TestCommerceExchangeRequiresQuantity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request without --quantity: %s", r.URL.String())
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := &App{Out: &out, BaseURL: srv.URL, Lang: "en", Mode: output.ModeConcise, Limiter: api.NewLimiter(300, 5)}
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newCommerceCmd(app))
	root.SetArgs([]string{"commerce", "exchange", "coins"})
	if err := root.Execute(); err == nil {
		t.Fatal("commerce exchange coins with no --quantity should error")
	}
}

// TestCommerceTransactionsAuthedAndValidated verifies `commerce
// transactions` goes through authedClient (Authorization header present)
// and validates both positional args before ever hitting the network.
func TestCommerceTransactionsAuthedAndValidated(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`[{"id":1,"item_id":24,"price":156,"quantity":3,"created":"2020-01-01T00:00:00Z"}]`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	app := authedTestApp(t, &out, srv)
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newCommerceCmd(app))
	root.SetArgs([]string{"commerce", "transactions", "current", "buys"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPath != "/v2/commerce/transactions/current/buys" {
		t.Fatalf("path = %q, want /v2/commerce/transactions/current/buys", gotPath)
	}
	if gotAuth != "Bearer TESTTOKEN" {
		t.Fatalf("Authorization header = %q, want %q", gotAuth, "Bearer TESTTOKEN")
	}
	if !strings.Contains(out.String(), "1s 56c") {
		t.Fatalf("output = %q, want a formatted coin string", out.String())
	}

	// A bogus kind must be rejected before any request reaches the server.
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request for invalid transactions args: %s", r.URL.Path)
	}))
	defer srv2.Close()
	var out2 bytes.Buffer
	app2 := authedTestApp(t, &out2, srv2)
	root2 := &cobra.Command{Use: "gw2"}
	root2.AddCommand(newCommerceCmd(app2))
	root2.SetArgs([]string{"commerce", "transactions", "bogus", "buys"})
	if err := root2.Execute(); err == nil {
		t.Fatal("commerce transactions bogus buys should error")
	}
}

// TestCommerceDeliveryRequiresToken verifies `commerce delivery` goes
// through authedClient: with no profile/token configured, it must fail with
// guidance to run `gw2 auth set`, never silently send an anonymous request.
func TestCommerceDeliveryRequiresToken(t *testing.T) {
	var out bytes.Buffer
	app := testApp(t, &out) // no profile configured -> empty token
	root := &cobra.Command{Use: "gw2"}
	root.AddCommand(newCommerceCmd(app))
	root.SetArgs([]string{"commerce", "delivery"})
	err := root.Execute()
	if err == nil {
		t.Fatal("commerce delivery with no configured key should error")
	}
	if !strings.Contains(err.Error(), "gw2 auth set") {
		t.Fatalf("error = %v, want it to mention gw2 auth set", err)
	}
}

// TestFormatCoin table-tests the gold/silver/copper formatter.
func TestFormatCoin(t *testing.T) {
	cases := []struct {
		copper int
		want   string
	}{
		{0, "0c"},
		{56, "56c"},
		{156, "1s 56c"},
		{123456, "12g 34s 56c"},
	}
	for _, tc := range cases {
		if got := formatCoin(tc.copper); got != tc.want {
			t.Errorf("formatCoin(%d) = %q, want %q", tc.copper, got, tc.want)
		}
	}
}
