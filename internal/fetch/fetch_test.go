package fetch

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func noSleep(context.Context, time.Duration) error { return nil }

func newClient(t *testing.T, srv *httptest.Server, mod func(*Options)) *Client {
	t.Helper()
	u, _ := url.Parse(srv.URL)
	o := Options{
		Timeout:         5 * time.Second,
		UserAgent:       "TestAgent/1.0",
		AcceptLanguage:  "en",
		BrowserHeaders:  true,
		SecCHUA:         `"Chromium";v="1"`,
		SecCHUAPlatform: `"Linux"`,
		AuditHeader:     "X-Audit: sitemap-audit",
		Headers:         map[string]string{"X-Secret": "s"},
		SiteHost:        u.Hostname(),
		Retries:         2,
		MaxRetryAfter:   time.Second,
		Sleep:           noSleep,
	}
	if mod != nil {
		mod(&o)
	}
	c, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	return c
}

func TestHeaders(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	resp := newClient(t, srv, nil).Get(context.Background(), Request{URL: srv.URL + "/"})
	if resp.Status != 200 || string(resp.Body) != "ok" {
		t.Fatalf("resp = %+v", resp)
	}
	for k, v := range map[string]string{
		"User-Agent": "TestAgent/1.0", "Accept-Language": "en", "X-Audit": "sitemap-audit", "X-Secret": "s",
		"Sec-Fetch-Mode": "navigate", "Sec-Ch-Ua": `"Chromium";v="1"`, "Sec-Ch-Ua-Platform": `"Linux"`, "Upgrade-Insecure-Requests": "1",
	} {
		if got.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, got.Get(k), v)
		}
	}
	plain := newClient(t, srv, func(o *Options) { o.BrowserHeaders = false; o.SiteHost = "other.org" })
	plain.Get(context.Background(), Request{URL: srv.URL})
	if got.Get("Sec-Fetch-Mode") != "" || got.Get("X-Secret") != "" {
		t.Error("browser headers off, and custom headers only for the site's host")
	}
}

func TestCustomHeadersStayOnTheSiteHost(t *testing.T) {
	var leaked atomic.Bool
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked.Store(r.Header.Get("X-Secret") != "")
		_, _ = w.Write([]byte("other"))
	}))
	defer other.Close()
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, strings.Replace(other.URL, "127.0.0.1", "localhost", 1)+"/x", http.StatusFound)
	}))
	defer site.Close()
	resp := newClient(t, site, nil).Get(context.Background(), Request{URL: site.URL, Follow: true})
	if resp.Status != 200 || resp.PrimaryStatus != 302 || !strings.HasSuffix(resp.PrimaryLocation, "/x") {
		t.Fatalf("resp = %+v", resp)
	}
	if leaked.Load() {
		t.Error("a custom header followed a redirect to another host")
	}
}

func TestNoFollowAndCookies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/set":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "1", Path: "/"})
			http.Redirect(w, r, "/home", http.StatusFound)
		case "/home":
			if c, err := r.Cookie("session"); err == nil && c.Value == "1" {
				_, _ = w.Write([]byte("with cookie"))
				return
			}
			_, _ = w.Write([]byte("no cookie"))
		case "/crawl":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "2", Path: "/"})
			http.Redirect(w, r, "../target?x=1", http.StatusMovedPermanently)
		}
	}))
	defer srv.Close()
	c := newClient(t, srv, nil)
	ctx := context.Background()
	if resp := c.Get(ctx, Request{URL: srv.URL + "/set", Follow: true, StoreCookies: true}); string(resp.Body) != "with cookie" || resp.FinalURL != srv.URL+"/home" {
		t.Fatalf("follow = %+v", resp)
	}
	resp := c.Get(ctx, Request{URL: srv.URL + "/crawl"})
	if resp.Status != 301 || resp.Location != srv.URL+"/target?x=1" {
		t.Fatalf("no follow = %+v", resp)
	}
	if got := c.Get(ctx, Request{URL: srv.URL + "/home"}); string(got.Body) != "with cookie" {
		t.Error("a crawl response must not change the session cookies")
	}
}

func TestRetries(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		switch r.URL.Path {
		case "/busy":
			if n == 1 {
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			_, _ = w.Write([]byte("ok"))
		case "/down":
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer srv.Close()
	c := newClient(t, srv, nil)
	resp := c.Get(context.Background(), Request{URL: srv.URL + "/busy"})
	if resp.Status != 200 || resp.Retries != 1 {
		t.Errorf("busy = %d after %d retries", resp.Status, resp.Retries)
	}
	calls.Store(0)
	if resp := c.Get(context.Background(), Request{URL: srv.URL + "/down"}); resp.Status != 503 || resp.Retries != 0 || calls.Load() != 1 {
		t.Errorf("a 503 without Retry-After is the answer: %+v", resp)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	var waits int
	dead := newClient(t, srv, func(o *Options) {
		o.RetryDelay = func() time.Duration { return time.Millisecond }
		o.Sleep = func(context.Context, time.Duration) error { waits++; return nil }
	})
	resp = dead.Get(context.Background(), Request{URL: "http://" + addr + "/"})
	if resp.Err == nil || resp.Status != 0 || resp.Retries != 2 || waits != 2 || resp.ErrText() == "" {
		t.Errorf("dead = %+v, waits %d", resp, waits)
	}
	if got := dead.Get(context.Background(), Request{URL: "http://" + addr + "/", NoRetry: true}); got.Retries != 0 {
		t.Error("NoRetry")
	}
}

func TestBodyLimits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("a", 3<<20)))
	}))
	defer srv.Close()
	c := newClient(t, srv, func(o *Options) { o.MaxBodySize = 2 << 20 })
	resp := c.Get(context.Background(), Request{URL: srv.URL, Keep: 10})
	if len(resp.Body) != 10 || resp.Size != 2<<20 || !resp.Truncated {
		t.Errorf("body %d, size %d, truncated %v", len(resp.Body), resp.Size, resp.Truncated)
	}
	full := newClient(t, srv, nil).Get(context.Background(), Request{URL: srv.URL})
	if full.Size != 3<<20 || len(full.Body) != 3<<20 || full.Truncated {
		t.Errorf("full: size %d", full.Size)
	}
}

func TestTimeoutAndCancel(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-block:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(block)
	c := newClient(t, srv, func(o *Options) { o.Timeout = 50 * time.Millisecond; o.Retries = 0 })
	resp := c.Get(context.Background(), Request{URL: srv.URL})
	if resp.ErrText() != "timeout" {
		t.Errorf("timeout text = %q", resp.ErrText())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := c.Get(ctx, Request{URL: srv.URL}); got.ErrText() != "canceled" {
		t.Errorf("canceled text = %q", got.ErrText())
	}
	if err := Sleep(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Error("Sleep must stop on cancel")
	}
	if err := Sleep(context.Background(), time.Millisecond); err != nil {
		t.Error(err)
	}
	if bad := c.Get(context.Background(), Request{URL: "http://%zz"}); bad.Err == nil {
		t.Error("an invalid URL must fail")
	}
}

func TestHelpers(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for v, want := range map[string]time.Duration{"5": 5 * time.Second, "Thu, 01 Jan 2026 00:00:10 GMT": 10 * time.Second, "Wed, 31 Dec 2025 00:00:00 GMT": 0} {
		got, ok := RetryAfter(v, now)
		if !ok || got != want {
			t.Errorf("RetryAfter(%q) = %v, %v", v, got, ok)
		}
	}
	for _, v := range []string{"", "-1", "soon"} {
		if _, ok := RetryAfter(v, now); ok {
			t.Errorf("RetryAfter(%q) should fail", v)
		}
	}
	if got := EncodeURL("https://e.com/a b/ă?x=\"1\"&y=%20"); got != "https://e.com/a%20b/%C4%83?x=%221%22&y=%20" {
		t.Errorf("EncodeURL = %q", got)
	}
	if BareHost("WWW.Example.com") != "example.com" {
		t.Error("BareHost")
	}
	if ErrorText(errors.New("boom")) != "boom" || (&Response{}).ErrText() != "" {
		t.Error("ErrorText")
	}
	if _, err := New(Options{InsecureSkipVerify: true}); err != nil {
		t.Error(err)
	}
}
