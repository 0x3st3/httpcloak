package client

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"context"
	"io"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
)

func TestDeflateBodiesDecodeZlibWrappedRawAndEmpty(t *testing.T) {
	var wrapped, raw bytes.Buffer
	zw := zlib.NewWriter(&wrapped)
	_, _ = zw.Write([]byte("decoded"))
	_ = zw.Close()
	fw, _ := flate.NewWriter(&raw, flate.DefaultCompression)
	_, _ = fw.Write([]byte("decoded"))
	_ = fw.Close()

	for name, test := range map[string]struct {
		body []byte
		want string
	}{
		"zlib":  {wrapped.Bytes(), "decoded"},
		"raw":   {raw.Bytes(), "decoded"},
		"empty": {nil, ""},
	} {
		got, err := Decompress(test.body, "deflate")
		if err != nil || string(got) != test.want {
			t.Errorf("%s Decompress = %q, %v; want %q", name, got, err, test.want)
		}

		server := httptest.NewTLSServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
			w.Header().Set("Content-Encoding", "deflate")
			_, _ = w.Write(test.body)
		}))
		client := NewClient("chrome-149-macos", WithInsecureSkipVerify(), WithForceHTTP1(), WithoutRetry())

		resp, err := client.Do(context.Background(), &Request{Method: "GET", URL: server.URL})
		if err != nil {
			t.Errorf("%s Do: %v", name, err)
		} else if body, _ := resp.Bytes(); string(body) != test.want {
			t.Errorf("%s Do body = %q, want %q", name, body, test.want)
		}

		stream, err := client.DoStream(context.Background(), &Request{Method: "GET", URL: server.URL})
		if err != nil {
			t.Errorf("%s DoStream: %v", name, err)
		} else {
			body, readErr := io.ReadAll(stream)
			stream.Close()
			if readErr != nil || string(body) != test.want {
				t.Errorf("%s DoStream body = %q, %v; want %q", name, body, readErr, test.want)
			}
		}
		client.Close()
		server.Close()
	}
}
