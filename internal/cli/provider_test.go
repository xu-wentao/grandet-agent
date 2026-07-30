package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderTestAcceptsHomeAfterProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/models" {
			t.Fatalf("path = %q, want /v1/models", request.URL.Path)
		}
		writer.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	home := t.TempDir()
	config := "schema_version: 1\nproviders:\n  local:\n    type: openai_compatible\n    base_url: " + server.URL + "/v1\n    enabled: true\n"
	if err := os.WriteFile(filepath.Join(home, "providers.yaml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	var err error
	output := captureStdout(t, func() {
		err = run([]string{"provider", "test", "local", "--home", home})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "provider local is healthy") {
		t.Fatalf("output = %q", output)
	}
}
