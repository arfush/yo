package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func testClient(payload []byte, status int) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    status,
			Status:        http.StatusText(status),
			ContentLength: int64(len(payload)),
			Header:        make(http.Header),
			Body:          io.NopCloser(bytes.NewReader(payload)),
			Request:       request,
		}, nil
	})}
}

func testPayload() []byte {
	return append([]byte{'M', 'Z'}, bytes.Repeat([]byte{0x90, 0x01, 0x02, 0x03}, 5000)...)
}

func testRelease(payload []byte, digest string) Release {
	return Release{
		Version:   "v1.4.0",
		Size:      int64(len(payload)),
		URL:       "https://github.com/arfush/yo/releases/download/v1.4.0/yo_v1.4.0_amd64.exe",
		SHA256:    digest,
		AssetName: "yo_v1.4.0_amd64.exe",
	}
}

func TestDownloadVerifiesAndWritesFile(t *testing.T) {
	payload := testPayload()
	sum := sha256.Sum256(payload)
	release := testRelease(payload, hex.EncodeToString(sum[:]))
	destination := filepath.Join(t.TempDir(), "yo.exe")
	var calls atomic.Int32
	path, err := Download(context.Background(), testClient(payload, http.StatusOK), release, destination, func(downloaded, total int64) {
		calls.Add(1)
		if total != release.Size {
			t.Errorf("total = %d", total)
		}
		if downloaded < 0 || downloaded > total {
			t.Errorf("downloaded = %d", downloaded)
		}
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if path != destination {
		t.Fatalf("path = %q", path)
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, payload) {
		t.Fatal("downloaded bytes differ")
	}
	if calls.Load() < 2 {
		t.Fatalf("progress calls = %d", calls.Load())
	}
	if _, err := os.Stat(destination + ".part"); !os.IsNotExist(err) {
		t.Fatal("temporary file was left behind")
	}
}

func TestDownloadRejectsBadHash(t *testing.T) {
	payload := testPayload()
	release := testRelease(payload, strings.Repeat("0", 64))
	destination := filepath.Join(t.TempDir(), "yo.exe")
	_, err := Download(context.Background(), testClient(payload, http.StatusOK), release, destination, nil)
	if err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("err = %v", err)
	}
	if _, statErr := os.Stat(destination); !os.IsNotExist(statErr) {
		t.Fatal("unverified file was kept")
	}
}

func TestDownloadRejectsBadSize(t *testing.T) {
	payload := testPayload()
	sum := sha256.Sum256(payload)
	release := testRelease(payload, hex.EncodeToString(sum[:]))
	release.Size += 1
	_, err := Download(context.Background(), testClient(payload, http.StatusOK), release, filepath.Join(t.TempDir(), "yo.exe"), nil)
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestDownloadRejectsNonExe(t *testing.T) {
	payload := bytes.Repeat([]byte("hello"), 100)
	sum := sha256.Sum256(payload)
	release := testRelease(payload, hex.EncodeToString(sum[:]))
	_, err := Download(context.Background(), testClient(payload, http.StatusOK), release, filepath.Join(t.TempDir(), "yo.exe"), nil)
	if err == nil || !strings.Contains(err.Error(), "Windows") {
		t.Fatalf("err = %v", err)
	}
}

func TestDownloadRejectsHTTPError(t *testing.T) {
	payload := testPayload()
	sum := sha256.Sum256(payload)
	release := testRelease(payload, hex.EncodeToString(sum[:]))
	_, err := Download(context.Background(), testClient(payload, http.StatusNotFound), release, filepath.Join(t.TempDir(), "yo.exe"), nil)
	if err == nil {
		t.Fatal("expected an error")
	}
}
