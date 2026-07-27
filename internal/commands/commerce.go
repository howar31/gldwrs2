package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/howar31/gldwrs2/internal/output"
	"github.com/spf13/cobra"
)

// commerceTransactionKinds and commerceTransactionTypes are the two
// positional-arg vocabularies accepted by `commerce transactions`, validated
// up front (before any network call) so a typo never reaches the API as a
// malformed path.
var (
	commerceTransactionKinds = map[string]bool{"current": true, "history": true}
	commerceTransactionTypes = map[string]bool{"buys": true, "sells": true}
)

// newCommerceCmd builds the "commerce" command tree covering the trading
// post: prices, listings, and the gem/coin exchange rate are public (no API
// key needed); your pending transactions and delivery box require an
// authenticated client (the "tradingpost" scope).
func newCommerceCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "commerce",
		Short: "Trading post prices, listings, and your orders",
	}
	cmd.AddCommand(
		newCommerceLookupCmd(app, "prices", "/v2/commerce/prices", renderPrices),
		newCommerceLookupCmd(app, "listings", "/v2/commerce/listings", renderListings),
		newCommerceExchangeCmd(app),
		newCommerceTransactionsCmd(app),
		newCommerceDeliveryCmd(app),
	)
	return cmd
}

// newCommerceLookupCmd builds the "prices" / "listings" leaf, the two
// commerce endpoints that follow a by-ids lookup shape (unlike catalogResource
// in data.go, there is no bare enumerate-all mode here: dumping the entire
// trading post is never useful, so at least one id is required). ids may be
// given positionally, via --ids, or both; they're merged before the fetch.
// Mirrors newCatalogResourceCmd's by-ids branch (data.go): merge the
// returned items back into a single JSON array for output.Render's raw arg.
func newCommerceLookupCmd(app *App, use, path string, render func([]json.RawMessage) string) *cobra.Command {
	covID := "commerce " + use
	var idsFlag string
	c := &cobra.Command{
		Use:   use + " [ids...]",
		Short: "Fetch " + covID,
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered(covID)
			idList := append([]string(nil), args...)
			if idsFlag != "" {
				idList = append(idList, splitIDs(idsFlag)...)
			}
			if len(idList) == 0 {
				return fmt.Errorf("%s requires at least one id (positional args or --ids)", covID)
			}
			ctx := context.Background()
			client, err := app.client()
			if err != nil {
				return err
			}
			items, err := client.GetByIDs(ctx, path, idList, nil)
			if err != nil {
				return err
			}
			concise := ""
			if app.Mode == output.ModeConcise && render != nil {
				concise = render(items)
			}
			merged, _ := json.Marshal(items)
			return output.Render(app.Out, merged, app.Mode, concise)
		},
	}
	c.Flags().StringVar(&idsFlag, "ids", "", "comma-separated ids (in addition to any positional ids)")
	return c
}

// newCommerceExchangeCmd builds the "exchange" parent with its two
// children, "coins" and "gems", both public and both requiring --quantity.
func newCommerceExchangeCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exchange",
		Short: "Gem/coin exchange rates",
	}
	cmd.AddCommand(
		newCommerceExchangeLeafCmd(app, "coins", "/v2/commerce/exchange/coins"),
		newCommerceExchangeLeafCmd(app, "gems", "/v2/commerce/exchange/gems"),
	)
	return cmd
}

// newCommerceExchangeLeafCmd builds one exchange direction ("coins" or
// "gems"). Both return a single object, not an array, so unlike
// newCommerceLookupCmd there's no items slice to remarshal: raw is passed to
// output.Render as-is, with the concise string computed here in RunE (via
// renderExchange) since output.Render's concise arg is a plain string.
func newCommerceExchangeLeafCmd(app *App, use, path string) *cobra.Command {
	covID := "commerce exchange " + use
	var quantity int
	c := &cobra.Command{
		Use:   use,
		Short: "Exchange rate: " + covID,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered(covID)
			if quantity <= 0 {
				return fmt.Errorf("%s requires --quantity (an integer > 0)", covID)
			}
			ctx := context.Background()
			client, err := app.client()
			if err != nil {
				return err
			}
			params := url.Values{}
			params.Set("quantity", strconv.Itoa(quantity))
			raw, err := client.Get(ctx, path, params)
			if err != nil {
				return err
			}
			concise := ""
			if app.Mode == output.ModeConcise {
				concise = renderExchange(raw, use)
			}
			return output.Render(app.Out, raw, app.Mode, concise)
		},
	}
	c.Flags().IntVar(&quantity, "quantity", 0, "amount to exchange (required, > 0)")
	return c
}

// newCommerceTransactionsCmd builds `commerce transactions <current|history>
// <buys|sells>`, authed (needs the "tradingpost" scope). Both positional
// args are validated against their fixed vocabularies before the network
// call, so a typo'd arg errors immediately instead of hitting the API with a
// malformed path. The endpoint is paginated, so this walks every page via
// GetAllPages -- a single unpaginated GET would silently truncate any
// history beyond the API's first page.
func newCommerceTransactionsCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "transactions <current|history> <buys|sells>",
		Short: "Your trading post transactions",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered("commerce transactions")
			kind, typ := args[0], args[1]
			if !commerceTransactionKinds[kind] {
				return fmt.Errorf("unknown transactions kind %q; valid: current, history", kind)
			}
			if !commerceTransactionTypes[typ] {
				return fmt.Errorf("unknown transactions type %q; valid: buys, sells", typ)
			}
			ctx := context.Background()
			client, err := app.authedClient()
			if err != nil {
				return err
			}
			items, err := client.GetAllPages(ctx, "/v2/commerce/transactions/"+kind+"/"+typ, nil)
			if err != nil {
				return err
			}
			concise := ""
			if app.Mode == output.ModeConcise {
				concise = renderTransactions(items)
			}
			merged, _ := json.Marshal(items)
			return output.Render(app.Out, merged, app.Mode, concise)
		},
	}
}

// newCommerceDeliveryCmd builds `commerce delivery`, authed. Like exchange,
// this returns a single object: renderDelivery takes the raw object
// directly.
func newCommerceDeliveryCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "delivery",
		Short: "Items and coins waiting in your delivery box",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			markCovered("commerce delivery")
			ctx := context.Background()
			client, err := app.authedClient()
			if err != nil {
				return err
			}
			raw, err := client.Get(ctx, "/v2/commerce/delivery", nil)
			if err != nil {
				return err
			}
			concise := ""
			if app.Mode == output.ModeConcise {
				concise = renderDelivery(raw)
			}
			return output.Render(app.Out, raw, app.Mode, concise)
		},
	}
}

// formatCoin formats an amount of copper as gold/silver/copper, e.g. 123456
// -> "12g 34s 56c" (10000 copper = 1 gold, 100 copper = 1 silver). Units
// above copper are omitted once they and everything more significant are
// zero; copper is always shown, even when it's the only nonzero unit (e.g.
// 0 -> "0c").
func formatCoin(copper int) string {
	sign := ""
	if copper < 0 {
		sign = "-"
		copper = -copper
	}
	gold := copper / 10000
	silver := (copper % 10000) / 100
	c := copper % 100
	var b strings.Builder
	if gold > 0 {
		fmt.Fprintf(&b, "%dg ", gold)
	}
	if gold > 0 || silver > 0 {
		fmt.Fprintf(&b, "%ds ", silver)
	}
	fmt.Fprintf(&b, "%dc", c)
	return sign + b.String()
}

// renderPrices renders /v2/commerce/prices items, each
// {id, buys:{unit_price,quantity}, sells:{unit_price,quantity}}, as one line
// per item: buy/sell unit price and quantity, plus the sell-minus-buy
// spread.
func renderPrices(items []json.RawMessage) string {
	var b strings.Builder
	for _, it := range items {
		var v struct {
			ID   int `json:"id"`
			Buys struct {
				UnitPrice int `json:"unit_price"`
				Quantity  int `json:"quantity"`
			} `json:"buys"`
			Sells struct {
				UnitPrice int `json:"unit_price"`
				Quantity  int `json:"quantity"`
			} `json:"sells"`
		}
		if err := json.Unmarshal(it, &v); err != nil {
			continue
		}
		spread := v.Sells.UnitPrice - v.Buys.UnitPrice
		fmt.Fprintf(&b, "%d\tbuy %s x%d\tsell %s x%d\tspread %s\n",
			v.ID,
			formatCoin(v.Buys.UnitPrice), v.Buys.Quantity,
			formatCoin(v.Sells.UnitPrice), v.Sells.Quantity,
			formatCoin(spread))
	}
	return strings.TrimRight(b.String(), "\n")
}

// renderListings renders /v2/commerce/listings items, each
// {id, buys:[{unit_price,quantity,listings}], sells:[...]}, as one line per
// item summarizing the best (first) buy order and sell listing. The API
// already returns both arrays sorted best-first, so index 0 is the top
// offer; empty arrays (no orders on one side) are guarded.
func renderListings(items []json.RawMessage) string {
	var b strings.Builder
	for _, it := range items {
		var v struct {
			ID   int `json:"id"`
			Buys []struct {
				UnitPrice int `json:"unit_price"`
				Quantity  int `json:"quantity"`
			} `json:"buys"`
			Sells []struct {
				UnitPrice int `json:"unit_price"`
				Quantity  int `json:"quantity"`
			} `json:"sells"`
		}
		if err := json.Unmarshal(it, &v); err != nil {
			continue
		}
		buyPart := "no buy orders"
		if len(v.Buys) > 0 {
			buyPart = fmt.Sprintf("top buy %s (qty %d)", formatCoin(v.Buys[0].UnitPrice), v.Buys[0].Quantity)
		}
		sellPart := "no sell listings"
		if len(v.Sells) > 0 {
			sellPart = fmt.Sprintf("top sell %s", formatCoin(v.Sells[0].UnitPrice))
		}
		fmt.Fprintf(&b, "%d\t%s\t%s\n", v.ID, buyPart, sellPart)
	}
	return strings.TrimRight(b.String(), "\n")
}

// renderExchange renders /v2/commerce/exchange/{coins,gems}, a single object
// {coins_per_gem, quantity}. The "quantity" field's unit depends on which
// endpoint produced it: for the coins endpoint it's the number of gems the
// requested coins would buy; for the gems endpoint it's the number of coins
// the requested gems would buy. kind ("coins" or "gems") disambiguates so
// the human-readable line can format it correctly.
func renderExchange(raw json.RawMessage, kind string) string {
	var v struct {
		CoinsPerGem int `json:"coins_per_gem"`
		Quantity    int `json:"quantity"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return ""
	}
	human := fmt.Sprintf("%d gems", v.Quantity)
	if kind == "gems" {
		human = formatCoin(v.Quantity)
	}
	return fmt.Sprintf("quantity: %d  coins_per_gem: %d  -> %s", v.Quantity, v.CoinsPerGem, human)
}

// renderTransactions renders /v2/commerce/transactions/* items, each
// {id, item_id, price, quantity, created}, as one line per transaction.
func renderTransactions(items []json.RawMessage) string {
	var b strings.Builder
	for _, it := range items {
		var v struct {
			ItemID   int    `json:"item_id"`
			Price    int    `json:"price"`
			Quantity int    `json:"quantity"`
			Created  string `json:"created"`
		}
		if err := json.Unmarshal(it, &v); err != nil {
			continue
		}
		fmt.Fprintf(&b, "item %d\t%s x%d\t(%s)\n", v.ItemID, formatCoin(v.Price), v.Quantity, v.Created)
	}
	return strings.TrimRight(b.String(), "\n")
}

// renderDelivery renders /v2/commerce/delivery, a single object
// {coins, items:[{id,count}]}, as a coins summary line plus one line per
// waiting item.
func renderDelivery(raw json.RawMessage) string {
	var v struct {
		Coins int `json:"coins"`
		Items []struct {
			ID    int `json:"id"`
			Count int `json:"count"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "coins: %s\titems: %d\n", formatCoin(v.Coins), len(v.Items))
	for _, it := range v.Items {
		fmt.Fprintf(&b, "%d\tx%d\n", it.ID, it.Count)
	}
	return strings.TrimRight(b.String(), "\n")
}
