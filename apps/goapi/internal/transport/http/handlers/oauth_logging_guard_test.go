package handlers

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Ensures OAuth HTTP handlers do not wire logging calls that could accidentally emit secrets.
func TestOAuthHandlerSourceDoesNotUseLogger(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	b, err := os.ReadFile(filepath.Join(dir, "oauth.go"))
	if err != nil {
		t.Fatalf("read oauth.go: %v", err)
	}
	src := string(b)
	forbidden := []string{"logger.", "zerolog", "Log.Info", "Log.Debug", "Log.Warn", "Log.Error"}
	for _, frag := range forbidden {
		if strings.Contains(src, frag) {
			t.Fatalf("oauth.go must not contain %q (avoid accidental secret logging)", frag)
		}
	}
}
