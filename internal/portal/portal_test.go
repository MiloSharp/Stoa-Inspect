package portal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const event = `{"id":"10000000-0000-4000-8000-000000000001","sequence":1,"type":"world.set","simulatedAt":0,"actor":"system","visibility":"shared","target":"ready"}`

func TestPortalServesPageAndState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.ndjson")
	if err := os.WriteFile(path, []byte(event+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	handler := New(path, "test", false).Handler()
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("unexpected page response: %d", page.Code)
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/state", nil))
	var state State
	if err := json.NewDecoder(response.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if state.View == nil || state.View.VisibleEvents != 1 {
		t.Fatalf("unexpected state: %#v", state)
	}
}

func TestPortalKeepsLastGoodState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.ndjson")
	if err := os.WriteFile(path, []byte(event+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	portal := New(path, "test", false)
	if first := portal.load(); first.View == nil || first.Stale {
		t.Fatalf("unexpected initial state: %#v", first)
	}
	if err := os.WriteFile(path, []byte(event+"\n{\"incomplete\":"), 0600); err != nil {
		t.Fatal(err)
	}
	if second := portal.load(); second.View == nil || !second.Stale || second.Error == "" {
		t.Fatalf("expected stale last-good state: %#v", second)
	}
}

func TestLoopbackAddress(t *testing.T) {
	if !IsLoopbackAddress("127.0.0.1:8787") || !IsLoopbackAddress("localhost:8787") || IsLoopbackAddress("0.0.0.0:8787") {
		t.Fatal("loopback address detection failed")
	}
}

func TestValidateSourceRejectsRemoteURL(t *testing.T) {
	if err := ValidateSource("https://example.com/api/state"); err == nil {
		t.Fatal("expected remote URL to be rejected")
	}
	if err := ValidateSource("http://127.0.0.1:8787/api/state"); err != nil {
		t.Fatalf("expected local Stoa URL to be accepted: %v", err)
	}
}
