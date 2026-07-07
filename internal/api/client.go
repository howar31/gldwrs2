package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultBaseURL   = "https://api.guildwars2.com"
	SchemaVersion    = "2026-07-07T00:00:00Z"
	MaxIDsPerRequest = 200
	defaultUserAgent = "gw2-cli"
	maxRetries       = 5
)

type Client struct {
	baseURL   string
	token     string
	lang      string
	userAgent string
	httpc     *http.Client
	limiter   *Limiter
	sleep     func(time.Duration)
}

type Option func(*Client)

func WithBaseURL(u string) Option              { return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") } }
func WithToken(t string) Option                { return func(c *Client) { c.token = t } }
func WithLang(l string) Option                 { return func(c *Client) { c.lang = l } }
func WithUserAgent(ua string) Option           { return func(c *Client) { c.userAgent = ua } }
func WithHTTPClient(h *http.Client) Option     { return func(c *Client) { c.httpc = h } }
func WithLimiter(l *Limiter) Option            { return func(c *Client) { c.limiter = l } }
func WithSleeper(f func(time.Duration)) Option { return func(c *Client) { c.sleep = f } }

func New(opts ...Option) *Client {
	c := &Client{
		baseURL:   DefaultBaseURL,
		lang:      "en",
		userAgent: defaultUserAgent,
		httpc:     &http.Client{Timeout: 30 * time.Second},
		sleep:     time.Sleep,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Get performs a single GET, injecting schema (v) and lang params unless the
// caller already set them. Transient statuses (429, 502-504) are retried.
func (c *Client) Get(ctx context.Context, path string, params url.Values) (json.RawMessage, error) {
	if params == nil {
		params = url.Values{}
	} else {
		params = cloneValues(params)
	}
	if params.Get("v") == "" && SchemaVersion != "" {
		params.Set("v", SchemaVersion)
	}
	if params.Get("lang") == "" && c.lang != "" {
		params.Set("lang", c.lang)
	}
	u := c.baseURL + path
	if enc := params.Encode(); enc != "" {
		u += "?" + enc
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if c.limiter != nil {
			if err := c.limiter.Wait(ctx); err != nil {
				return nil, err
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", c.userAgent)
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		resp, err := c.httpc.Do(req)
		if err != nil {
			return nil, err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode < 400 {
			return json.RawMessage(body), nil
		}
		if isTransient(resp.StatusCode) && attempt < maxRetries {
			c.sleep(backoff(attempt, resp.Header.Get("Retry-After")))
			lastErr = apiErrorFrom(resp.StatusCode, body)
			continue
		}
		return nil, apiErrorFrom(resp.StatusCode, body)
	}
	return nil, lastErr
}

func isTransient(code int) bool {
	return code == http.StatusTooManyRequests || (code >= 502 && code <= 504)
}

func backoff(attempt int, retryAfter string) time.Duration {
	if retryAfter != "" {
		if secs, err := time.ParseDuration(retryAfter + "s"); err == nil {
			return secs
		}
	}
	d := 500 * time.Millisecond
	for i := 0; i < attempt; i++ {
		d *= 2
	}
	if d > 30*time.Second {
		d = 30 * time.Second
	}
	return d
}

func cloneValues(v url.Values) url.Values {
	out := make(url.Values, len(v))
	for k, vs := range v {
		out[k] = append([]string(nil), vs...)
	}
	return out
}

// GetByIDs fetches multiple resources, chunking ids at MaxIDsPerRequest and
// merging the returned arrays.
func (c *Client) GetByIDs(ctx context.Context, path string, ids []string, params url.Values) ([]json.RawMessage, error) {
	var all []json.RawMessage
	for _, chunk := range chunkIDs(ids, MaxIDsPerRequest) {
		p := url.Values{}
		if params != nil {
			p = cloneValues(params)
		}
		p.Set("ids", strings.Join(chunk, ","))
		raw, err := c.Get(ctx, path, p)
		if err != nil {
			return nil, err
		}
		var batch []json.RawMessage
		if err := json.Unmarshal(raw, &batch); err != nil {
			return nil, err
		}
		all = append(all, batch...)
	}
	return all, nil
}

func chunkIDs(ids []string, size int) [][]string {
	if size <= 0 {
		size = MaxIDsPerRequest
	}
	var out [][]string
	for i := 0; i < len(ids); i += size {
		end := i + size
		if end > len(ids) {
			end = len(ids)
		}
		out = append(out, ids[i:end])
	}
	return out
}

// getWithHeader is like Get but also returns the X-Page-Total header value.
func (c *Client) getWithPageTotal(ctx context.Context, path string, params url.Values) (json.RawMessage, int, error) {
	if params == nil {
		params = url.Values{}
	} else {
		params = cloneValues(params)
	}
	if params.Get("v") == "" {
		params.Set("v", SchemaVersion)
	}
	if params.Get("lang") == "" && c.lang != "" {
		params.Set("lang", c.lang)
	}
	u := c.baseURL + path
	if enc := params.Encode(); enc != "" {
		u += "?" + enc
	}
	if c.limiter != nil {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, 0, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.httpc.Do(req)
	if err != nil {
		return nil, 0, err
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, 0, apiErrorFrom(resp.StatusCode, body)
	}
	total := 1
	if v := resp.Header.Get("X-Page-Total"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			total = n
		}
	}
	return json.RawMessage(body), total, nil
}

// GetAllPages walks every page (page_size 200) using X-Page-Total.
func (c *Client) GetAllPages(ctx context.Context, path string, params url.Values) ([]json.RawMessage, error) {
	base := url.Values{}
	if params != nil {
		base = cloneValues(params)
	}
	base.Set("page_size", "200")
	var all []json.RawMessage
	total := 1
	for page := 0; page < total; page++ {
		p := cloneValues(base)
		p.Set("page", strconv.Itoa(page))
		raw, t, err := c.getWithPageTotal(ctx, path, p)
		if err != nil {
			return nil, err
		}
		total = t
		var batch []json.RawMessage
		if err := json.Unmarshal(raw, &batch); err != nil {
			return nil, err
		}
		all = append(all, batch...)
	}
	return all, nil
}
