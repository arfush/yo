package watchparty

import (
	"net/http"
	"strings"
	"testing"
)

// TestHandshakeError covers the replies a misconfigured Worker address gives.
// A 200 is the interesting one: the address is alive but it is not the relay,
// which used to surface as a bare status code.
func TestHandshakeError(t *testing.T) {
	tests := []struct {
		status int
		want   string
	}{
		{status: http.StatusBadRequest, want: "ключ"},
		{status: http.StatusNotFound, want: "не найдена"},
		{status: http.StatusOK, want: "не комната"},
		{status: http.StatusInternalServerError, want: "500"},
	}
	for _, test := range tests {
		err := handshakeError(test.status)
		if err == nil {
			t.Fatalf("handshakeError(%d) = nil", test.status)
		}
		if !strings.Contains(err.Error(), test.want) {
			t.Fatalf("handshakeError(%d) = %q, want it to mention %q", test.status, err, test.want)
		}
	}
}
