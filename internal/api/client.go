package api

import (
	"context"
	"encoding/json"
	"fmt"
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
	maxBackoff       = 30 * time.Second

	// LangNone, set as the "lang" param value by a caller, suppresses the
	// client's automatic lang injection for that request: non-localized
	// endpoints don't take a lang and shouldn't be sent one. The sentinel is
	// stripped before the request is built, so it never reaches the API.
	LangNone = "none"
)

type Client struct {
	baseURL   string
	token     string
	lang      string
	userAgent string
	httpc     *http.Client
	limiter   *Limiter
	sleep     func(time.Duration) // test hook; nil -> context-aware wait
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
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// buildURL injects the schema (v) and lang params unless the caller already
// set them (LangNone suppresses lang entirely) and returns the full request
// URL.
func (c *Client) buildURL(path string, params url.Values) string {
	if params == nil {
		params = url.Values{}
	} else {
		params = cloneValues(params)
	}
	if params.Get("v") == "" && SchemaVersion != "" {
		params.Set("v", SchemaVersion)
	}
	switch params.Get("lang") {
	case LangNone:
		params.Del("lang")
	case "":
		if c.lang != "" {
			params.Set("lang", c.lang)
		}
	}
	u := c.baseURL + path
	if enc := params.Encode(); enc != "" {
		u += "?" + enc
	}
	return u
}

// pause waits for d or until ctx is cancelled. The injectable sleep hook
// (WithSleeper) takes over in tests so retries are instant.
func (c *Client) pause(ctx context.Context, d time.Duration) error {
	if c.sleep != nil {
		c.sleep(d)
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// do performs one GET against a fully-built URL, retrying transient
// statuses (429, 502-504) with capped backoff. It returns the body and the
// response headers (for callers that need pagination metadata). This is the
// single request pipeline shared by Get and getWithPageTotal, so both paths
// get identical auth, rate limiting, and retry behavior.
func (c *Client) do(ctx context.Context, u string) (json.RawMessage, http.Header, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if c.limiter != nil {
			if err := c.limiter.Wait(ctx); err != nil {
				return nil, nil, err
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("User-Agent", c.userAgent)
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		resp, err := c.httpc.Do(req)
		if err != nil {
			return nil, nil, err
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, nil, fmt.Errorf("read response body: %w", readErr)
		}

		if resp.StatusCode < 400 {
			return json.RawMessage(body), resp.Header, nil
		}
		if isTransient(resp.StatusCode) && attempt < maxRetries {
			lastErr = apiErrorFrom(resp.StatusCode, body)
			if err := c.pause(ctx, backoff(attempt, resp.Header.Get("Retry-After"))); err != nil {
				return nil, nil, err
			}
			continue
		}
		return nil, nil, apiErrorFrom(resp.StatusCode, body)
	}
	return nil, nil, lastErr
}

// Get performs a single GET, injecting schema (v) and lang params unless the
// caller already set them. Transient statuses (429, 502-504) are retried.
func (c *Client) Get(ctx context.Context, path string, params url.Values) (json.RawMessage, error) {
	body, _, err := c.do(ctx, c.buildURL(path, params))
	return body, err
}

func isTransient(code int) bool {
	return code == http.StatusTooManyRequests || (code >= 502 && code <= 504)
}

// backoff picks the wait before the next retry attempt. A server-supplied
// Retry-After (seconds) is honored but capped at maxBackoff -- an absurd or
// hostile value must not make the CLI sleep for hours; past the cap it is
// better to keep the bounded retry schedule and ultimately fail with exit
// code 5. Without Retry-After, exponential backoff from 500ms, same cap.
func backoff(attempt int, retryAfter string) time.Duration {
	if retryAfter != "" {
		if secs, err := time.ParseDuration(retryAfter + "s"); err == nil {
			if secs < 0 {
				secs = 0
			}
			if secs > maxBackoff {
				secs = maxBackoff
			}
			return secs
		}
	}
	d := 500 * time.Millisecond
	for i := 0; i < attempt; i++ {
		d *= 2
	}
	if d > maxBackoff {
		d = maxBackoff
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

// getWithPageTotal is like Get but also returns the X-Page-Total header
// value. It shares Get's request pipeline (do), including transient-status
// retry, so a mid-pagination 429/503 retries instead of failing the whole
// multi-page fetch.
func (c *Client) getWithPageTotal(ctx context.Context, path string, params url.Values) (json.RawMessage, int, error) {
	body, hdr, err := c.do(ctx, c.buildURL(path, params))
	if err != nil {
		return nil, 0, err
	}
	total := 1
	if v := hdr.Get("X-Page-Total"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			total = n
		}
	}
	return body, total, nil
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
