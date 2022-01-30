package pbinfo_test

import (
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gocolly/colly/v2/debug"
)

//go:embed testdata/*
var data embed.FS

type handler struct {
	tb testing.TB
	fs fs.FS
}

var _ http.Handler = (*handler)(nil)

func newTestServer(tb testing.TB) *httptest.Server {
	tb.Helper()

	return httptest.NewServer(handler{tb, data})
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if t := r.URL.Query().Get("t"); t != "" {
		timeout, err := strconv.Atoi(t)
		if err == nil {
			time.Sleep(time.Duration(timeout) * time.Millisecond)
		}
	}

	switch {
	case r.URL.Path == "/ajx-module/ajx-problema-afisare-teste.php":
		h.getTestCases(w, r)
	case r.URL.Path == "/php/descarca-test.php":
		h.downloadTest(w, r)
	case strings.HasPrefix(r.URL.Path, "/probleme"):
		h.getProblem(w, r)
	default:
		h.error(w, http.StatusNotFound, nil)
	}
}

func (h handler) getTestCases(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	h.serveFile(w, r, path.Join(id, "test-cases.htm"))
}

func (h handler) downloadTest(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	name := q.Get("id") + "." + q.Get("tip")

	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	h.serveFile(w, r, path.Join("chunks", name))
}

func (h handler) getProblem(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[strings.LastIndex(r.URL.Path, "/"):]
	h.serveFile(w, r, path.Join(id, "index.html"))
}

func (h handler) serveFile(w http.ResponseWriter, r *http.Request, filename string) {
	f, err := h.fs.Open(path.Join("testdata", filename))
	if err != nil {
		h.error(w, http.StatusNotFound, err)
		return
	}

	stat, err := f.Stat()
	if err != nil {
		h.error(w, http.StatusInternalServerError, err)
		return
	}

	rs, ok := f.(io.ReadSeeker)
	if !ok {
		h.error(w, http.StatusInternalServerError, fmt.Errorf("%q does not implement io.ReadSeeker", filename))
	}

	http.ServeContent(w, r, filename, stat.ModTime(), rs)
}

func (h handler) logErr(err error) {
	h.tb.Logf("server error: %v", err)
}

func (h handler) error(w http.ResponseWriter, code int, err error) {
	text := http.StatusText(code)
	if err != nil {
		h.logErr(err)
		text = err.Error() + "; status text "
	}

	w.WriteHeader(code)
	io.WriteString(w, text)
}

type transport struct {
	baseURL *url.URL
	rt      http.RoundTripper
}

var _ http.RoundTripper = (*transport)(nil)

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Host = t.baseURL.Host
	req.URL.Scheme = t.baseURL.Scheme
	req.URL.User = t.baseURL.User

	deadline, ok := req.Context().Deadline()
	if ok && deadline.After(time.Now()) {
		timeout := time.Until(deadline).Milliseconds()
		q := req.URL.Query()
		q.Add("t", strconv.FormatInt(timeout*2, 10))
		req.URL.RawQuery = q.Encode()
	}

	return t.rt.RoundTrip(req)
}

func newTestTransport(base string, rt http.RoundTripper) http.RoundTripper {
	baseURL, err := url.Parse(base)
	if err != nil {
		panic(err)
	}
	return &transport{baseURL, rt}
}

type testDebugger struct {
	tb      testing.TB
	start   time.Time
	counter int32
}

func newTestDebugger(tb testing.TB) *testDebugger {
	tb.Helper()

	return &testDebugger{tb: tb}
}

var _ debug.Debugger = (*testDebugger)(nil)

func (t *testDebugger) Init() error {
	t.counter = 0
	t.start = time.Now()
	return nil
}

func (t *testDebugger) Event(e *debug.Event) {
	i := atomic.AddInt32(&t.counter, 1)
	t.tb.Logf("[%06d] %d [%06d - %s] %q	(%s)", i, e.CollectorID, e.RequestID, e.Type, e.Values, time.Since(t.start))
}

func timeoutContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout == 0 {
		return context.Background(), func() {}
	}

	return context.WithTimeout(context.Background(), timeout)
}
