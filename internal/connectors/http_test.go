package connectors

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DongDuong2001/graft/internal/models"
	"github.com/DongDuong2001/graft/internal/observability"
)

func TestHTTPForwarder_Send_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if string(b) != `{"ok":true}` {
			t.Errorf("body %s", b)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	before := observability.SnapshotNow()
	f := NewHTTPForwarder(HTTPConfig{Timeout: 2 * time.Second, MaxRetries: 0})
	code, attempts, err := f.Send(context.Background(), &models.Rule{
		DestinationURL: srv.URL,
	}, []byte(`{"ok":true}`))
	after := observability.SnapshotNow()

	if err != nil || code != 200 || attempts != 1 {
		t.Fatalf("Send: code=%d attempts=%d err=%v", code, attempts, err)
	}
	if after.ForwardsTotal-before.ForwardsTotal != 1 {
		t.Fatalf("metrics forwards: before=%v after=%v", before, after)
	}
}

func TestHTTPForwarder_Send_EmptyURL(t *testing.T) {
	f := NewHTTPForwarder(HTTPConfig{})
	_, _, err := f.Send(context.Background(), &models.Rule{}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHTTPForwarder_Send_RetriesOn503(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	before := observability.SnapshotNow()
	f := NewHTTPForwarder(HTTPConfig{
		Timeout:       3 * time.Second,
		MaxRetries:    4,
		BaseRetryWait: time.Millisecond,
	})
	code, attempts, err := f.Send(context.Background(), &models.Rule{
		DestinationURL: srv.URL,
	}, []byte("{}"))
	after := observability.SnapshotNow()

	if err != nil || code != 200 || attempts != 3 {
		t.Fatalf("Send: code=%d attempts=%d err=%v calls=%d", code, attempts, err, calls.Load())
	}
	if after.ForwardsTotal-before.ForwardsTotal != 3 {
		t.Fatalf("forwards delta: %d", after.ForwardsTotal-before.ForwardsTotal)
	}
}

func TestHTTPForwarder_DefaultMethodPOST(t *testing.T) {
	var method string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	f := NewHTTPForwarder(HTTPConfig{Timeout: time.Second, MaxRetries: 0})
	_, _, err := f.Send(context.Background(), &models.Rule{DestinationURL: srv.URL}, []byte("{}"))
	if err != nil || method != http.MethodPost {
		t.Fatalf("method=%q err=%v", method, err)
	}
}

func TestIsRetryableNetErr(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{stringError("net/http: request canceled (Client.Timeout exceeded while awaiting headers)"), true},
		{stringError("connection reset by peer"), true},
		{stringError("EOF"), true},
		{stringError("Temporary failure"), true},
		{stringError("some other error"), false},
		{nil, false},
	}
	for _, tc := range tests {
		if got := isRetryableNetErr(tc.err); got != tc.want {
			t.Errorf("%v: got %v want %v", tc.err, got, tc.want)
		}
	}
}

type stringError string

func (e stringError) Error() string { return string(e) }
