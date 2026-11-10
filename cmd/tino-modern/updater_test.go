package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type updateTransport func(*http.Request) (*http.Response, error)

func (fn updateTransport) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func TestCompareVersions(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"v0.17.3", "0.17.2", 1}, {"0.17.2", "v0.17.2", 0}, {"0.16.9", "0.17.0", -1}, {"dev", "0.17.0", 0},
	} {
		if got := compareVersions(tc.a, tc.b); got != tc.want {
			t.Fatalf("compareVersions(%q,%q)=%d want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestDownloadUpdateVerifiesOfficialAssetDigest(t *testing.T) {
	payload := []byte("signed release payload")
	digest := sha256.Sum256(payload)
	assetURL := "https://github.com/gadevsbr/Tino/releases/download/v0.17.3/Tino.exe"
	client := &http.Client{Transport: updateTransport(func(req *http.Request) (*http.Response, error) {
		body := ""
		if req.URL.String() == "https://api.test/latest" {
			body = fmt.Sprintf(`{"tag_name":"v0.17.3","assets":[{"name":"Tino.exe","browser_download_url":%q,"digest":"sha256:%s","size":%d}]}`, assetURL, hex.EncodeToString(digest[:]), len(payload))
		} else if req.URL.String() == assetURL {
			body = string(payload)
		} else {
			return nil, fmt.Errorf("unexpected URL %s", req.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	path, version, err := downloadUpdate(context.Background(), client, "https://api.test/latest", "0.17.2", t.TempDir())
	if err != nil || version != "v0.17.3" || !strings.HasSuffix(path, "Tino-0.17.3.exe") {
		t.Fatalf("path=%q version=%q err=%v", path, version, err)
	}
}

func TestDownloadUpdateRejectsWrongDigest(t *testing.T) {
	assetURL := "https://github.com/gadevsbr/Tino/releases/download/v0.17.3/Tino.exe"
	client := &http.Client{Transport: updateTransport(func(req *http.Request) (*http.Response, error) {
		body := `{"tag_name":"v0.17.3","assets":[{"name":"Tino.exe","browser_download_url":"` + assetURL + `","digest":"sha256:` + strings.Repeat("0", 64) + `","size":3}]}`
		if req.URL.String() == assetURL {
			body = "bad"
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	if _, _, err := downloadUpdate(context.Background(), client, "https://api.test/latest", "0.17.2", t.TempDir()); err == nil {
		t.Fatal("wrong digest accepted")
	}
}
