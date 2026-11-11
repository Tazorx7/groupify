package static

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newSite(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "index.html"),
		[]byte(`<!doctype html><div id="root">shell</div>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "app-abc123.js"),
		[]byte("console.log('hi')"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func request(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestHandlerDisabledWhenDirEmpty(t *testing.T) {
	if Handler("") != nil {
		t.Fatal("expected nil handler for an empty STATIC_DIR")
	}
}

// A deep link like /admin/registrations must return the shell so the client
// router can take over; otherwise a refresh breaks the console.
func TestUnknownPathServesIndex(t *testing.T) {
	handler := Handler(newSite(t))

	rec := request(t, handler, "/admin/registrations")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "shell") {
		t.Fatalf("body = %q, want the index shell", rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", got)
	}
}

func TestHashedAssetsAreImmutable(t *testing.T) {
	handler := Handler(newSite(t))

	rec := request(t, handler, "/assets/app-abc123.js")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if string(body) != "console.log('hi')" {
		t.Fatalf("body = %q", body)
	}
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Fatalf("Cache-Control = %q, want immutable", got)
	}
}

func TestIndexIsNotCached(t *testing.T) {
	handler := Handler(newSite(t))

	rec := request(t, handler, "/")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", got)
	}
}

// http.FileServer lists directories; a static site must never do that.
func TestDirectoryListingIsNotServed(t *testing.T) {
	handler := Handler(newSite(t))

	rec := request(t, handler, "/assets/")

	if strings.Contains(rec.Body.String(), "app-abc123.js") {
		t.Fatalf("directory listing leaked: %q", rec.Body.String())
	}
}

func TestExistsRequiresIndex(t *testing.T) {
	dir := t.TempDir()
	if Exists(dir) {
		t.Fatal("directory without index.html reported as existing")
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !Exists(dir) {
		t.Fatal("directory with index.html reported as missing")
	}
}
