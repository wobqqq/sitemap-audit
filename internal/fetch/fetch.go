// Package fetch sends the audit's HTTP requests.
package fetch

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Options configures a client.
type Options struct {
	Timeout            time.Duration
	UserAgent          string
	AcceptLanguage     string
	BrowserHeaders     bool
	SecCHUA            string
	SecCHUAPlatform    string
	AuditHeader        string
	Headers            map[string]string
	SiteHost           string
	InsecureSkipVerify bool
	MaxBodySize        int64
	Retries            int
	RetryDelay         func() time.Duration
	MaxRetryAfter      time.Duration
	MaxRedirects       int
	Sleep              func(context.Context, time.Duration) error
}

// Client sends requests that share one cookie jar, like a browser tab.
type Client struct {
	opts      Options
	jar       http.CookieJar
	transport *http.Transport
}

// New builds a client with an empty cookie jar.
func New(opts Options) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("cookie jar: %w", err)
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 15 * time.Second
	}
	if opts.MaxBodySize <= 0 {
		opts.MaxBodySize = 64 << 20
	}
	if opts.MaxRedirects <= 0 {
		opts.MaxRedirects = 10
	}
	if opts.Sleep == nil {
		opts.Sleep = Sleep
	}
	if opts.RetryDelay == nil {
		opts.RetryDelay = func() time.Duration { return 0 }
	}
	tr := http.DefaultTransport.(*http.Transport).Clone() //nolint:forcetypeassert // the default transport is always *http.Transport
	tr.Proxy = http.ProxyFromEnvironment
	tr.MaxIdleConnsPerHost = 64
	tr.ResponseHeaderTimeout = opts.Timeout
	if opts.InsecureSkipVerify {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // opt-in via config
	}
	return &Client{opts: opts, jar: jar, transport: tr}, nil
}

// Close releases idle connections.
func (c *Client) Close() { c.transport.CloseIdleConnections() }

// Sleep waits for d or until the context ends.
func Sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
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

// Request describes one GET.
type Request struct {
	URL          string
	Follow       bool
	Keep         int64
	StoreCookies bool
	NoRetry      bool
}

// Response is what came back, or why nothing did.
type Response struct {
	RequestURL      string
	Status          int
	PrimaryStatus   int
	PrimaryLocation string
	FinalURL        string
	Location        string
	Header          http.Header
	ContentType     string
	Body            []byte
	Size            int64
	Truncated       bool
	Duration        time.Duration
	Err             error
	Retries         int
}

// ErrText is the error message without Go's request prefix.
func (r *Response) ErrText() string {
	if r.Err == nil {
		return ""
	}
	return ErrorText(r.Err)
}

// ErrorText shortens transport errors to what a person needs to read.
func ErrorText(err error) string {
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	return err.Error()
}

// Get sends the request, retrying when there is no response or a 429/503 with Retry-After.
func (c *Client) Get(ctx context.Context, req Request) *Response {
	var resp *Response
	for attempt := 0; ; attempt++ {
		resp = c.once(ctx, req)
		resp.Retries = attempt
		if ctx.Err() != nil || req.NoRetry || attempt >= c.opts.Retries {
			return resp
		}
		var wait time.Duration
		switch {
		case resp.Err != nil:
			wait = c.opts.RetryDelay()
		case resp.Status == http.StatusTooManyRequests || resp.Status == http.StatusServiceUnavailable:
			ra, ok := RetryAfter(resp.Header.Get("Retry-After"), time.Now())
			if !ok {
				return resp
			}
			wait = min(ra, c.opts.MaxRetryAfter)
		default:
			return resp
		}
		if err := c.opts.Sleep(ctx, wait); err != nil {
			return resp
		}
	}
}

// RetryAfter reads a Retry-After header: seconds or an HTTP date.
func RetryAfter(v string, now time.Time) (time.Duration, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	if n, err := strconv.Atoi(v); err == nil {
		if n < 0 {
			return 0, false
		}
		return time.Duration(n) * time.Second, true
	}
	t, err := http.ParseTime(v)
	if err != nil {
		return 0, false
	}
	d := t.Sub(now)
	if d < 0 {
		d = 0
	}
	return d, true
}

func (c *Client) once(ctx context.Context, req Request) *Response {
	out := &Response{RequestURL: req.URL}
	target := EncodeURL(req.URL)
	hreq, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		out.Err = err
		return out
	}
	c.setHeaders(hreq)
	jar := c.jar
	if !req.StoreCookies {
		jar = readOnlyJar{c.jar}
	}
	hc := &http.Client{Transport: c.transport, Jar: jar, Timeout: c.opts.Timeout}
	if req.Follow {
		hc.CheckRedirect = func(next *http.Request, via []*http.Request) error {
			if len(via) == 1 && next.Response != nil {
				out.PrimaryStatus = next.Response.StatusCode
				out.PrimaryLocation = next.URL.String()
			}
			if len(via) >= c.opts.MaxRedirects {
				return http.ErrUseLastResponse
			}
			if !c.ownHost(next.URL) {
				for name := range c.opts.Headers {
					next.Header.Del(name)
				}
			}
			return nil
		}
	} else {
		hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	}
	start := time.Now()
	resp, err := hc.Do(hreq)
	if err != nil {
		out.Err = err
		out.Duration = time.Since(start)
		return out
	}
	defer func() { _ = resp.Body.Close() }()
	out.Status = resp.StatusCode
	if out.PrimaryStatus == 0 {
		out.PrimaryStatus = resp.StatusCode
	}
	out.Header = resp.Header
	out.ContentType = resp.Header.Get("Content-Type")
	out.FinalURL = resp.Request.URL.String()
	if loc := resp.Header.Get("Location"); loc != "" && resp.StatusCode >= 300 && resp.StatusCode < 400 {
		if u, perr := resp.Request.URL.Parse(loc); perr == nil {
			out.Location = u.String()
		} else {
			out.Location = loc
		}
	}
	out.Body, out.Size, out.Truncated, out.Err = readBody(resp.Body, c.opts.MaxBodySize, req.Keep)
	out.Duration = time.Since(start)
	return out
}

func readBody(r io.Reader, limit, keep int64) ([]byte, int64, bool, error) {
	if keep <= 0 || keep > limit {
		keep = limit
	}
	lr := io.LimitReader(r, limit+1)
	body, err := io.ReadAll(io.LimitReader(lr, keep))
	size := int64(len(body))
	if err == nil {
		n, cerr := io.Copy(io.Discard, lr)
		size += n
		err = cerr
	}
	truncated := size > limit
	if truncated {
		size = limit
	}
	return body, size, truncated, err
}

func (c *Client) setHeaders(r *http.Request) {
	h := r.Header
	if c.opts.BrowserHeaders {
		if c.opts.SecCHUA != "" {
			h.Set("Sec-Ch-Ua", c.opts.SecCHUA)
			h.Set("Sec-Ch-Ua-Mobile", "?0")
		}
		if c.opts.SecCHUAPlatform != "" {
			h.Set("Sec-Ch-Ua-Platform", c.opts.SecCHUAPlatform)
		}
		h.Set("Upgrade-Insecure-Requests", "1")
		h.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
		h.Set("Sec-Fetch-Site", "none")
		h.Set("Sec-Fetch-Mode", "navigate")
		h.Set("Sec-Fetch-User", "?1")
		h.Set("Sec-Fetch-Dest", "document")
		h.Set("Priority", "u=0, i")
	}
	if c.opts.UserAgent != "" {
		h.Set("User-Agent", c.opts.UserAgent)
	}
	if c.opts.AcceptLanguage != "" {
		h.Set("Accept-Language", c.opts.AcceptLanguage)
	}
	if name, val, ok := strings.Cut(c.opts.AuditHeader, ":"); ok && strings.TrimSpace(name) != "" {
		h.Set(strings.TrimSpace(name), strings.TrimSpace(val))
	}
	if c.ownHost(r.URL) {
		for name, val := range c.opts.Headers {
			h.Set(name, val)
		}
	}
}

func (c *Client) ownHost(u *url.URL) bool {
	if c.opts.SiteHost == "" {
		return false
	}
	return BareHost(u.Hostname()) == BareHost(c.opts.SiteHost)
}

// BareHost lower-cases a host and drops "www.".
func BareHost(h string) string {
	h = strings.ToLower(h)
	return strings.TrimPrefix(h, "www.")
}

type readOnlyJar struct{ http.CookieJar }

func (readOnlyJar) SetCookies(*url.URL, []*http.Cookie) {}

// EncodeURL percent-encodes spaces, non-ASCII and invalid characters like a browser does.
func EncodeURL(raw string) string {
	var b strings.Builder
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c < 0x21 || c > 0x7e || strings.IndexByte("\"<>`{}|\\^", c) >= 0 {
			fmt.Fprintf(&b, "%%%02X", c)
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}
