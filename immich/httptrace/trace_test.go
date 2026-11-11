package httptrace

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func newTestResponse() *http.Response {
	return &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
	}
}

func doRoundTrip(t *testing.T, ht *Tracer) *http.Response {
	t.Helper()
	rt := ht.DecorateRT(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newTestResponse(), nil
	}))
	req, err := http.NewRequest(http.MethodGet, "http://example.test/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// Closing a dumpReader twice (which net/http is allowed to do on request
// bodies) must not panic.
func TestDumpReaderDoubleClose(t *testing.T) {
	dr := newDumpReader(io.NopCloser(strings.NewReader("data")), 0)
	if err := dr.Close(); err != nil {
		t.Fatal(err)
	}
	if err := dr.Close(); err != nil {
		t.Fatal(err)
	}
}

// Tracer.Close must wait for in-flight dumps: it cannot return while a
// response body has not been closed, and must write the trace output once the
// body is closed.
func TestTracerCloseWaitsForInflightDump(t *testing.T) {
	var out bytes.Buffer
	ht := NewTracer(&out)

	resp := doRoundTrip(t, ht)

	done := make(chan struct{})
	go func() {
		ht.Close()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("Close returned while a dump was still in flight")
	case <-time.After(50 * time.Millisecond):
	}

	resp.Body.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not return after the response body was closed")
	}

	if !strings.Contains(out.String(), "client #") {
		t.Error("expected the dump to be written to the trace output")
	}
}

// Many concurrent round trips racing Tracer.Close must not panic ("WaitGroup
// is reused before previous Wait has returned") nor deadlock. Run with -race.
func TestTracerCloseRacesRoundTrips(t *testing.T) {
	var out bytes.Buffer
	ht := NewTracer(&out)

	var wg sync.WaitGroup
	stop := make(chan struct{})
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rt := ht.DecorateRT(roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return newTestResponse(), nil
			}))
			for {
				select {
				case <-stop:
					return
				default:
				}
				req, err := http.NewRequest(http.MethodGet, "http://example.test/x", nil)
				if err != nil {
					return
				}
				resp, err := rt.RoundTrip(req)
				if err == nil {
					resp.Body.Close()
				}
			}
		}()
	}

	// Let a few round trips start, then close while more are in flight.
	time.Sleep(5 * time.Millisecond)
	if err := ht.Close(); err != nil {
		t.Fatal(err)
	}
	close(stop)
	wg.Wait()

	// A round trip started after Close must be served but its dump dropped:
	// nothing may be written to the trace output any more.
	size := out.Len()
	for range 5 {
		resp := doRoundTrip(t, ht)
		resp.Body.Close()
	}
	if out.Len() != size {
		t.Error("a dump was written after Tracer.Close")
	}
}

// Tracer.Close must not hang forever when a body is never closed: the wait is
// bounded.
func TestTracerCloseIsBounded(t *testing.T) {
	var out bytes.Buffer
	ht := NewTracer(&out)
	ht.closeTimeout = 50 * time.Millisecond

	resp := doRoundTrip(t, ht)
	defer resp.Body.Close()

	start := time.Now()
	if err := ht.Close(); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("Close blocked for %s although a body was never closed", elapsed)
	}
}
