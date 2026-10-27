package browser

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAuthorizeIntegration(t *testing.T) {
	if os.Getenv("YOANIME_AUTH_TEST") == "" {
		t.Skip("set YOANIME_AUTH_TEST=1 to launch Chromium")
	}

	withToken := `<!doctype html><html><body>ok</body><script>
localStorage.setItem('auth', JSON.stringify({token:{access_token:'test-token',refresh_token:'refresh',expires_in:3600,timestamp:Date.now(),token_type:'Bearer'}}));
</script></html>`
	withoutToken := `<!doctype html><html><body>wait</body></html>`

	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, withToken)
	})
	mux.HandleFunc("/wait", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, withoutToken)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	manager := New()
	if !manager.Installed() {
		t.Skip("Chromium is not installed")
	}

	token, err := manager.Authorize(context.Background(), server.URL+"/login", filepath.Join(t.TempDir(), "profile"), nil)
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if token.AccessToken != "test-token" {
		t.Fatalf("access token = %q", token.AccessToken)
	}
	if token.ExpiresIn != 3600 || token.RefreshToken != "refresh" {
		t.Fatalf("token metadata = %+v", token)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if _, err := manager.Authorize(ctx, server.URL+"/wait", filepath.Join(t.TempDir(), "profile2"), nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Authorize without token returned %v", err)
	}
	if err := manager.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
