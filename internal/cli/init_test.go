package cli

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitDryRunPrintsPlanWithoutCreatingWorkspace(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".grandet")
	var err error
	output := captureStdout(t, func() {
		err = run([]string{"init", "--home", home, "--dry-run"})
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		home,
		filepath.Join(home, "logs"),
		filepath.Join(home, "traces"),
		filepath.Join(home, "cache"),
		filepath.Join(home, "policies"),
		filepath.Join(home, "evals"),
		filepath.Join(home, "evals", "golden"),
		filepath.Join(home, "evals", "regression"),
		filepath.Join(home, "evals", "safety"),
	} {
		if !strings.Contains(output, "create dir: "+path+"\n") {
			t.Errorf("dry-run output missing directory %s:\n%s", path, output)
		}
	}
	for _, path := range []string{
		filepath.Join(home, "config.yaml"),
		filepath.Join(home, "providers.yaml"),
		filepath.Join(home, "models.yaml"),
		filepath.Join(home, "user-profile.yaml"),
		filepath.Join(home, "policies", "stingy-v1.yaml"),
		filepath.Join(home, "grandet.db"),
	} {
		if !strings.Contains(output, "create file: "+path+"\n") {
			t.Errorf("dry-run output missing file %s:\n%s", path, output)
		}
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatalf("dry-run created workspace: %v", err)
	}
}

func TestRunAndAnalyzeBaseline(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".grandet")
	if err := run([]string{"init", "--home", home}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" || request.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("request = %s %q", request.URL.Path, request.Header.Get("Authorization"))
		}
		var payload struct {
			Model    string `json:"model"`
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || payload.Model != "test-model" {
			t.Errorf("model request = %#v, %v", payload, err)
		}
		switch payload.Messages[0].Content {
		case "http failure":
			writer.Header().Set("x-request-id", "request-http-failure")
			writer.WriteHeader(http.StatusServiceUnavailable)
			_, _ = writer.Write([]byte(`{"error":"unavailable"}`))
			return
		case "no choices":
			writer.Header().Set("x-request-id", "request-no-choices")
			_, _ = writer.Write([]byte(`{"choices":[]}`))
			return
		case "empty message":
			writer.Header().Set("x-request-id", "request-empty-message")
			_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":""}}]}`))
			return
		}
		writer.Header().Set("x-request-id", "request-1")
		_, _ = writer.Write([]byte(`{"id":"completion-1","choices":[{"message":{"content":"measured response"}}],"usage":{"prompt_tokens":3,"completion_tokens":5,"completion_tokens_details":{"reasoning_tokens":2},"cost":0.001}}`))
	}))
	defer server.Close()
	if err := os.WriteFile(filepath.Join(home, "providers.yaml"), []byte("schema_version: 1\nproviders:\n  test:\n    type: openai_compatible\n    base_url: "+server.URL+"/v1\n    api_key_env: GRANDET_TEST_API_KEY\n    enabled: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "models.yaml"), []byte("models:\n  - id: test/model\n    provider: test\n    upstream_name: test-model\n    enabled: true\nexecution_profiles:\n  - id: fixed-profile\n    model: test/model\n    enabled: true\n    max_output_tokens: 12\n    temperature: 0.2\n  - id: other-profile\n    model: test/model\n    enabled: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GRANDET_TEST_API_KEY", "test-key")
	output := captureStdout(t, func() {
		if err := run([]string{"run", "measure this", "--profile", "fixed-profile", "--task-family", "summarization", "--session", "matching-session", "--home", home}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(output, "Profile: fixed-profile\nStatus: completed\nmeasured response\n") {
		t.Fatalf("run output = %q", output)
	}
	if err := run([]string{"run", "--home", home, "--profile", "other-profile", "--session", "other-session", "measure this too"}); err != nil {
		t.Fatal(err)
	}
	output = captureStdout(t, func() {
		if err := run([]string{"analyze", "cost", "--home", home, "--last", "7d", "--session", "matching-session", "--profile", "fixed-profile", "--outcome", "completed"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(output, "Trajectories: 1\n") || !strings.Contains(output, "Known provider cost: $0.001000\n") {
		t.Fatalf("cost report = %q", output)
	}
	output = captureStdout(t, func() {
		if err := run([]string{"analyze", "task-distribution", "--home", home, "--last", "7d", "--session", "matching-session", "--profile", "fixed-profile", "--outcome", "completed"}); err != nil {
			t.Fatal(err)
		}
	})
	if output != "summarization: 1\n" {
		t.Fatalf("task distribution = %q", output)
	}
	output = captureStdout(t, func() {
		if err := run([]string{"analyze", "task-distribution", "--home", home, "--last", "7d", "--session", "other-session"}); err != nil {
			t.Fatal(err)
		}
	})
	if output != "general_qa: 1\n" {
		t.Fatalf("default task distribution = %q", output)
	}
	db, err := sql.Open("sqlite", filepath.Join(home, "grandet.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var snapshot string
	if err := db.QueryRow(`SELECT k.task_profile_json FROM tasks k JOIN trajectories t ON t.id = k.trajectory_id WHERE t.session_id = ?`, "matching-session").Scan(&snapshot); err != nil || !strings.Contains(snapshot, `"schema_version":"task-profile/v1"`) {
		t.Fatalf("task profile snapshot = %q, %v", snapshot, err)
	}
	for _, failure := range []struct{ prompt, requestID string }{{"http failure", "request-http-failure"}, {"no choices", "request-no-choices"}, {"empty message", "request-empty-message"}} {
		if err := run([]string{"run", "--home", home, "--profile", "fixed-profile", "--session", failure.prompt, failure.prompt}); err == nil {
			t.Fatalf("expected %q provider failure", failure.prompt)
		}
		var trajectoryStatus, callStatus, requestID string
		if err := db.QueryRow(`SELECT t.status, m.status, m.provider_request_id FROM trajectories t JOIN model_calls m ON m.trajectory_id = t.id WHERE t.session_id = ?`, failure.prompt).Scan(&trajectoryStatus, &callStatus, &requestID); err != nil {
			t.Fatal(err)
		}
		if trajectoryStatus != "FAILED" || callStatus != "FAILED" || requestID != failure.requestID {
			t.Fatalf("%q persisted statuses/request ID = %q, %q, %q", failure.prompt, trajectoryStatus, callStatus, requestID)
		}
	}
}

func TestTaskClassifyUsesLocalRules(t *testing.T) {
	context := filepath.Join(t.TempDir(), "pod.yaml")
	if err := os.WriteFile(context, []byte("kind: Pod\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var err error
	output := captureStdout(t, func() {
		err = run([]string{"task", "classify", "Diagnose this Kubernetes error", "--context", context, "--tool", "kubectl", "--schema", "result.json"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, `"value": "kubernetes_troubleshooting"`) || !strings.Contains(output, `"value": "json_schema"`) || !strings.Contains(output, `"l0:tool declarations"`) {
		t.Fatalf("classification output = %s", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = writer
	defer func() {
		os.Stdout = original
	}()

	fn()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	return string(output)
}
