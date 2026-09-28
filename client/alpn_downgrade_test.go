package client

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sardanioss/httpcloak/transport"
)

// h1OnlyServer answers only HTTP/1.1, the way a server that declines h2 in
// ALPN does. With nextProtos nil it negotiates no ALPN at all.
func h1OnlyServer(t *testing.T, nextProtos []string) (string, func() map[string]int) {
	t.Helper()
	var mu sync.Mutex
	remotes := map[string]int{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		remotes[r.RemoteAddr]++
		mu.Unlock()
		fmt.Fprintf(w, "proto=%s", r.Proto)
	})

	cert := httptest.NewUnstartedServer(nil)
	cert.StartTLS()
	certs := cert.TLS.Certificates
	cert.Close()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	tlsLn := tls.NewListener(ln, &tls.Config{Certificates: certs, NextProtos: nextProtos})
	server := &http.Server{Handler: handler, TLSNextProto: map[string]func(*http.Server, *tls.Conn, http.Handler){}}
	go server.Serve(tlsLn)
	t.Cleanup(func() { server.Close() })

	return "https://" + ln.Addr().String() + "/", func() map[string]int {
		mu.Lock()
		defer mu.Unlock()
		out := make(map[string]int, len(remotes))
		for k, v := range remotes {
			out[k] = v
		}
		return out
	}
}

func TestALPNDowngradeToHTTP1(t *testing.T) {
	cases := []struct {
		name       string
		nextProtos []string
	}{
		{"http/1.1 negotiated", []string{"http/1.1"}},
		{"no ALPN", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			url, remotes := h1OnlyServer(t, tc.nextProtos)
			c := NewClient("chrome-latest", WithDisableHTTP3(), WithInsecureSkipVerify(), WithoutRetry(), WithTimeout(10*time.Second))
			defer c.Close()
			ctx := context.Background()

			for i := 0; i < 3; i++ {
				stream, err := c.DoStream(ctx, &Request{Method: http.MethodGet, URL: url})
				if err != nil {
					t.Fatalf("stream %d: %v", i, err)
				}
				body, err := stream.ReadAll()
				stream.Close()
				if err != nil {
					t.Fatalf("stream %d body: %v", i, err)
				}
				if stream.StatusCode != 200 || string(body) != "proto=HTTP/1.1" || stream.Protocol != "h1" {
					t.Fatalf("stream %d: status=%d protocol=%s body=%q", i, stream.StatusCode, stream.Protocol, body)
				}
			}

			resp, err := c.Do(ctx, &Request{Method: http.MethodPost, URL: url, Body: strings.NewReader("a=b")})
			if err != nil {
				t.Fatalf("do: %v", err)
			}
			if resp.StatusCode != 200 || resp.Protocol != "h1" {
				t.Fatalf("do: status=%d protocol=%s", resp.StatusCode, resp.Protocol)
			}

			if got := remotes(); len(got) != 1 {
				t.Fatalf("expected every request on the connection that declined h2, got %d: %v", len(got), got)
			}
		})
	}
}

func TestALPNDowngradeForcedHTTP2Fails(t *testing.T) {
	url, remotes := h1OnlyServer(t, []string{"http/1.1"})
	c := NewClient("chrome-latest", WithForceHTTP2(), WithInsecureSkipVerify(), WithoutRetry(), WithTimeout(10*time.Second))
	defer c.Close()

	_, err := c.DoStream(context.Background(), &Request{Method: http.MethodGet, URL: url})
	if !errors.Is(err, transport.ErrALPNMismatch) {
		t.Fatalf("expected ALPN mismatch, got %v", err)
	}
	if got := remotes(); len(got) != 0 {
		t.Fatalf("forced HTTP/2 must not send a request: %v", got)
	}
}
