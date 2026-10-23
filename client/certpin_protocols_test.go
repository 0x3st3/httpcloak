package client

import (
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestCertificatePinsApplyToEveryProtocolAndStreaming(t *testing.T) {
	server := httptest.NewUnstartedServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()
	target, _ := url.Parse(server.URL)

	for name, force := range map[string]Option{"h1": WithForceHTTP1(), "h2": WithForceHTTP2()} {
		client := NewClient("chrome-149-macos", WithInsecureSkipVerify(), WithoutRetry(), force)
		client.PinCertificate("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", ForHost(target.Hostname()))

		_, err := client.Do(context.Background(), &Request{Method: "GET", URL: server.URL})
		if err == nil || !strings.Contains(err.Error(), "certificate pinning failed") {
			t.Errorf("%s Do: expected certificate pin failure, got %v", name, err)
		}
		stream, err := client.DoStream(context.Background(), &Request{Method: "GET", URL: server.URL})
		if stream != nil {
			stream.Close()
		}
		if err == nil || !strings.Contains(err.Error(), "certificate pinning failed") {
			t.Errorf("%s DoStream: expected certificate pin failure, got %v", name, err)
		}
		client.Close()
	}
}
