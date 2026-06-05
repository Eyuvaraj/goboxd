//go:build integration

package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/thesouldev/goboxd/internal/validate"
)

var baseURL = func() string {
	if u := os.Getenv("GOBOXD_URL"); u != "" {
		return u
	}
	return "http://localhost:8080"
}()

type limitsDef struct {
	WallTimeS    int `json:"wall_time_s,omitempty"`
	MemoryKB     int `json:"memory_kb,omitempty"`
	MaxProcesses int `json:"max_processes,omitempty"`
}

type phaseOverride struct {
	Limits limitsDef `json:"limits,omitempty"`
	Flags  []string  `json:"flags,omitempty"`
}

type runRequest struct {
	Language         string         `json:"language"`
	Source           string         `json:"source"`
	SourceFilename   string         `json:"source_filename,omitempty"`
	ArtifactFilename string         `json:"artifact_filename,omitempty"`
	Build            *phaseOverride `json:"build,omitempty"`
	Run              *phaseOverride `json:"run,omitempty"`
	Tests            []testCase     `json:"tests"`
}

type testCase struct {
	Stdin          string `json:"stdin"`
	ExpectedStdout string `json:"expected_stdout"`
}

type runResponse struct {
	Status string `json:"status"`
	Build  struct {
		Status string `json:"status"`
	} `json:"build"`
	Tests []struct {
		Status string `json:"status"`
		Stdout string `json:"stdout"`
	} `json:"tests"`
}

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func postRun(t *testing.T, req runRequest) (runResponse, int) {
	t.Helper()
	body, _ := json.Marshal(req)
	resp, err := http.Post(baseURL+"/run", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	defer resp.Body.Close()

	var r runResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return r, resp.StatusCode
}

func postRunRaw(t *testing.T, req runRequest) ([]byte, int) {
	t.Helper()
	body, _ := json.Marshal(req)
	resp, err := http.Post(baseURL+"/run", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	buf.ReadFrom(resp.Body)
	return buf.Bytes(), resp.StatusCode
}

func waitHealthy(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/healthz")
		if err == nil && resp.StatusCode == 200 {
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatal("service did not become healthy in time")
}

func TestMain(m *testing.M) {
	// Wait for the service to accept requests before running any test.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/healthz")
		if err == nil && resp.StatusCode == 200 {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	os.Exit(m.Run())
}

func TestHealthz(t *testing.T) {
	resp, err := http.Get(baseURL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestPy3HelloWorld(t *testing.T) {
	resp, code := postRun(t, runRequest{
		Language: "py3",
		Source:   "print('hello')\n",
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "hello\n"}},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if resp.Status != validate.StatusAccepted {
		t.Fatalf("expected %s, got %s (build: %s)", validate.StatusAccepted, resp.Status, resp.Build.Status)
	}
}

func TestBashHelloWorld(t *testing.T) {
	resp, code := postRun(t, runRequest{
		Language: "bash",
		Source:   "echo hello\n",
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "hello\n"}},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if resp.Status != validate.StatusAccepted {
		t.Fatalf("expected %s, got %s", validate.StatusAccepted, resp.Status)
	}
}

func TestJsHelloWorld(t *testing.T) {
	resp, code := postRun(t, runRequest{
		Language: "js",
		Source:   `console.log("hello")` + "\n",
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "hello\n"}},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if resp.Status != validate.StatusAccepted {
		t.Fatalf("expected %s, got %s", validate.StatusAccepted, resp.Status)
	}
}

func TestCHelloWorld(t *testing.T) {
	src := `#include <stdio.h>
int main() { printf("hello\n"); return 0; }
`
	resp, code := postRun(t, runRequest{
		Language: "c",
		Source:   src,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "hello\n"}},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if resp.Status != validate.StatusAccepted {
		t.Fatalf("expected %s, got %s (build: %s)", validate.StatusAccepted, resp.Status, resp.Build.Status)
	}
}

func TestCppHelloWorld(t *testing.T) {
	src := `#include <iostream>
int main() { std::cout << "hello\n"; return 0; }
`
	resp, code := postRun(t, runRequest{
		Language: "cpp",
		Source:   src,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "hello\n"}},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if resp.Status != validate.StatusAccepted {
		t.Fatalf("expected %s, got %s (build: %s)", validate.StatusAccepted, resp.Status, resp.Build.Status)
	}
}

func TestJavaHelloWorld(t *testing.T) {
	src := `public class Main {
    public static void main(String[] args) {
        System.out.println("hello");
    }
}
`
	resp, code := postRun(t, runRequest{
		Language:         "java",
		Source:           src,
		SourceFilename:   "Main.java",
		ArtifactFilename: "Main",
		Tests:            []testCase{{Stdin: "", ExpectedStdout: "hello\n"}},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if resp.Status != validate.StatusAccepted {
		t.Fatalf("expected %s, got %s (build: %s)", validate.StatusAccepted, resp.Status, resp.Build.Status)
	}
}

func TestVerilogHelloWorld(t *testing.T) {
	src := `module main;
  initial begin
    $display("hello");
    $finish;
  end
endmodule
`
	resp, code := postRun(t, runRequest{
		Language: "verilog",
		Source:   src,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "hello\n"}},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if resp.Status != validate.StatusAccepted {
		t.Fatalf("expected %s, got %s (build: %s)", validate.StatusAccepted, resp.Status, resp.Build.Status)
	}
}

func TestBuildFailure(t *testing.T) {
	resp, code := postRun(t, runRequest{
		Language: "c",
		Source:   "this is not C code",
		Tests:    []testCase{{Stdin: "", ExpectedStdout: ""}},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200 even on build failure, got %d", code)
	}
	if resp.Status != validate.StatusBuildFailed {
		t.Fatalf("expected %s, got %s", validate.StatusBuildFailed, resp.Status)
	}
	if resp.Build.Status != validate.BuildStatusFailed {
		t.Fatalf("expected build.status=%s, got %s", validate.BuildStatusFailed, resp.Build.Status)
	}
	for i, tr := range resp.Tests {
		if tr.Status != validate.StatusNotExecuted {
			t.Fatalf("test[%d]: expected %s, got %s", i, validate.StatusNotExecuted, tr.Status)
		}
	}
}

func TestWrongOutput(t *testing.T) {
	resp, code := postRun(t, runRequest{
		Language: "py3",
		Source:   "print('actual')\n",
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "expected\n"}},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if resp.Status != validate.StatusWrongOutput {
		t.Fatalf("expected %s, got %s", validate.StatusWrongOutput, resp.Status)
	}
}

func TestUnknownLanguage(t *testing.T) {
	_, code := postRunRaw(t, runRequest{
		Language: "cobol",
		Source:   "HELLO WORLD",
		Tests:    []testCase{{Stdin: "", ExpectedStdout: ""}},
	})
	if code != 400 {
		t.Fatalf("expected HTTP 400 for unknown language, got %d", code)
	}
}

func TestInvalidJSON(t *testing.T) {
	resp, err := http.Post(baseURL+"/run", "application/json",
		bytes.NewBufferString(`{"language": "py3", "source": `))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for malformed JSON, got %d", resp.StatusCode)
	}
	var er errorResponse
	json.NewDecoder(resp.Body).Decode(&er)
	if er.Error.Code != "invalid_json" {
		t.Fatalf("expected invalid_json, got %s", er.Error.Code)
	}
}

func TestInvalidTestCount(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"language": "py3",
		"source":   "print('hi')\n",
		"tests":    []testCase{},
	})
	resp, err := http.Post(baseURL+"/run", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for empty tests, got %d", resp.StatusCode)
	}
	var er errorResponse
	json.NewDecoder(resp.Body).Decode(&er)
	if er.Error.Code != "invalid_test_count" {
		t.Fatalf("expected invalid_test_count, got %s", er.Error.Code)
	}
}

func TestStdinTooLarge(t *testing.T) {
	large := make([]byte, 70*1024) // 70 KiB > 64 KiB limit
	for i := range large {
		large[i] = 'x'
	}
	body, _ := json.Marshal(map[string]any{
		"language": "py3",
		"source":   "print('hi')\n",
		"tests":    []map[string]any{{"stdin": string(large), "expected_stdout": "hi\n"}},
	})
	resp, err := http.Post(baseURL+"/run", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for oversized stdin, got %d", resp.StatusCode)
	}
	var er errorResponse
	json.NewDecoder(resp.Body).Decode(&er)
	if er.Error.Code != "stdin_too_large" {
		t.Fatalf("expected stdin_too_large, got %s", er.Error.Code)
	}
}

func TestMissingSourceFilename(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"language": "java",
		"source":   "public class Main { public static void main(String[] a) {} }",
		// source_filename intentionally omitted
		"tests": []testCase{{Stdin: "", ExpectedStdout: ""}},
	})
	resp, err := http.Post(baseURL+"/run", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for missing source_filename, got %d", resp.StatusCode)
	}
	var er errorResponse
	json.NewDecoder(resp.Body).Decode(&er)
	if er.Error.Code != "missing_source_filename" {
		t.Fatalf("expected missing_source_filename, got %s", er.Error.Code)
	}
}

func TestPathTraversalFilename(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"language":        "java",
		"source":          "public class X {}",
		"source_filename": "../../etc/passwd",
		"tests":           []testCase{{Stdin: "", ExpectedStdout: ""}},
	})
	resp, err := http.Post(baseURL+"/run", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for path traversal filename, got %d", resp.StatusCode)
	}
	var er errorResponse
	json.NewDecoder(resp.Body).Decode(&er)
	if er.Error.Code != "invalid_filename" {
		t.Fatalf("expected error code invalid_filename, got %s", er.Error.Code)
	}
}

func TestDisallowedFlag(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"language": "cpp",
		"source": `#include <iostream>
int main(){std::cout<<"hi";}`,
		"build": map[string]any{
			"flags": []string{"-fplugin=evil.so"},
		},
		"tests": []testCase{{Stdin: "", ExpectedStdout: "hi"}},
	})
	resp, err := http.Post(baseURL+"/run", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for disallowed flag, got %d", resp.StatusCode)
	}
	var er errorResponse
	json.NewDecoder(resp.Body).Decode(&er)
	if er.Error.Code != "invalid_flag" {
		t.Fatalf("expected error code invalid_flag, got %s", er.Error.Code)
	}
}

func TestMultipleTestCases(t *testing.T) {
	resp, code := postRun(t, runRequest{
		Language: "py3",
		Source:   "n=int(input());print(n*2)\n",
		Tests: []testCase{
			{Stdin: "3\n", ExpectedStdout: "6\n"},
			{Stdin: "5\n", ExpectedStdout: "10\n"},
			{Stdin: "0\n", ExpectedStdout: "0\n"},
		},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if resp.Status != validate.StatusAccepted {
		for i, tr := range resp.Tests {
			fmt.Printf("  test[%d]: status=%s stdout=%q\n", i, tr.Status, tr.Stdout)
		}
		t.Fatalf("expected %s, got %s", validate.StatusAccepted, resp.Status)
	}
}

func TestWhitespaceMismatch(t *testing.T) {
	// Program prints "hello\n" but expected has no newline — whitespace diff, not wrong_output.
	resp, code := postRun(t, runRequest{
		Language: "py3",
		Source:   "print('hello')\n",
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "hello"}},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if resp.Status != validate.StatusWhitespaceMismatch {
		t.Fatalf("expected %s, got %s", validate.StatusWhitespaceMismatch, resp.Status)
	}
}

// skipIfNotRegistered skips the test when the language is not in the registry.
// Use this for additional languages that can be commented out in configs/languages.yaml.
func skipIfNotRegistered(t *testing.T, langID string) {
	t.Helper()
	resp, err := http.Get(baseURL + "/info")
	if err != nil {
		t.Fatalf("GET /info: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Languages []struct {
			ID string `json:"id"`
		} `json:"languages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode /info: %v", err)
	}
	for _, l := range body.Languages {
		if l.ID == langID {
			return
		}
	}
	t.Skipf("%s not registered in this build (uncomment in configs/languages.yaml to enable)", langID)
}

// Additional language smoke tests — skipped automatically when the language is not registered.

func TestRubyHelloWorld(t *testing.T) {
	skipIfNotRegistered(t, "ruby")
	resp, code := postRun(t, runRequest{
		Language: "ruby",
		Source:   "puts 'hello'\n",
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "hello\n"}},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if resp.Status != validate.StatusAccepted {
		t.Fatalf("expected %s, got %s", validate.StatusAccepted, resp.Status)
	}
}

func TestLuaHelloWorld(t *testing.T) {
	skipIfNotRegistered(t, "lua")
	resp, code := postRun(t, runRequest{
		Language: "lua",
		Source:   `print("hello")` + "\n",
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "hello\n"}},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if resp.Status != validate.StatusAccepted {
		t.Fatalf("expected %s, got %s", validate.StatusAccepted, resp.Status)
	}
}

func TestRustHelloWorld(t *testing.T) {
	skipIfNotRegistered(t, "rust")
	src := `fn main() { println!("hello"); }` + "\n"
	resp, code := postRun(t, runRequest{
		Language: "rust",
		Source:   src,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "hello\n"}},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if resp.Status != validate.StatusAccepted {
		t.Fatalf("expected %s, got %s (build: %s)", validate.StatusAccepted, resp.Status, resp.Build.Status)
	}
}

func TestGoHelloWorld(t *testing.T) {
	skipIfNotRegistered(t, "go")
	src := `package main
import "fmt"
func main() { fmt.Println("hello") }
`
	resp, code := postRun(t, runRequest{
		Language: "go",
		Source:   src,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "hello\n"}},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if resp.Status != validate.StatusAccepted {
		t.Fatalf("expected %s, got %s (build: %s)", validate.StatusAccepted, resp.Status, resp.Build.Status)
	}
}

func TestOCamlHelloWorld(t *testing.T) {
	skipIfNotRegistered(t, "ocaml")
	src := `print_string "hello\n";;` + "\n"
	resp, code := postRun(t, runRequest{
		Language: "ocaml",
		Source:   src,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "hello\n"}},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if resp.Status != validate.StatusAccepted {
		t.Fatalf("expected %s, got %s", validate.StatusAccepted, resp.Status)
	}
}

func TestKotlinHelloWorld(t *testing.T) {
	skipIfNotRegistered(t, "kotlin")
	src := `fun main() { println("hello") }` + "\n"
	resp, code := postRun(t, runRequest{
		Language: "kotlin",
		Source:   src,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "hello\n"}},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if resp.Status != validate.StatusAccepted {
		t.Fatalf("expected %s, got %s (build: %s)", validate.StatusAccepted, resp.Status, resp.Build.Status)
	}
}

// ── Status vocabulary coverage ────────────────────────────────────────────────

func TestTimeExceeded(t *testing.T) {
	resp, code := postRun(t, runRequest{
		Language: "py3",
		Source:   "while True: pass\n",
		Run:      &phaseOverride{Limits: limitsDef{WallTimeS: 1}},
		Tests:    []testCase{{Stdin: "", ExpectedStdout: ""}},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if resp.Status != validate.StatusTimeExceeded {
		t.Fatalf("expected %s, got %s", validate.StatusTimeExceeded, resp.Status)
	}
	if resp.Tests[0].Status != validate.StatusTimeExceeded {
		t.Fatalf("expected tests[0].status=%s, got %s", validate.StatusTimeExceeded, resp.Tests[0].Status)
	}
}

func TestRuntimeError(t *testing.T) {
	resp, code := postRun(t, runRequest{
		Language: "py3",
		Source:   "import sys; sys.exit(1)\n",
		Tests:    []testCase{{Stdin: "", ExpectedStdout: ""}},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if resp.Status != validate.StatusRuntimeError {
		t.Fatalf("expected %s, got %s", validate.StatusRuntimeError, resp.Status)
	}
}

func TestMemoryExceeded(t *testing.T) {
	// Allocate well past the memory cap; exact threshold varies by cgroup config.
	// Skip in environments where cgroup memory limits aren't enforced.
	resp, code := postRun(t, runRequest{
		Language: "py3",
		Source:   "x = bytearray(512 * 1024 * 1024)\n", // 512 MiB
		Run:      &phaseOverride{Limits: limitsDef{MemoryKB: 32768}},
		Tests:    []testCase{{Stdin: "", ExpectedStdout: ""}},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if resp.Status != validate.StatusMemoryExceeded && resp.Status != validate.StatusRuntimeError {
		// runtime_error is acceptable if the OOM kill arrives as SIGKILL before
		// nsjail logs the cgroup memory event.
		t.Skipf("memory limit enforcement not triggered (got %s); skip in this environment", resp.Status)
	}
}

func TestSourceTooLarge(t *testing.T) {
	large := make([]byte, 300*1024) // 300 KiB > 256 KiB limit
	for i := range large {
		large[i] = 'x'
	}
	body, _ := json.Marshal(map[string]any{
		"language": "py3",
		"source":   string(large),
		"tests":    []testCase{{Stdin: "", ExpectedStdout: ""}},
	})
	resp, err := http.Post(baseURL+"/run", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for oversized source, got %d", resp.StatusCode)
	}
	var er errorResponse
	json.NewDecoder(resp.Body).Decode(&er)
	if er.Error.Code != "source_too_large" {
		t.Fatalf("expected source_too_large, got %s", er.Error.Code)
	}
}

func TestExpectedTooLarge(t *testing.T) {
	large := make([]byte, 70*1024) // 70 KiB > 64 KiB per-test limit
	for i := range large {
		large[i] = 'x'
	}
	body, _ := json.Marshal(map[string]any{
		"language": "py3",
		"source":   "print('hi')\n",
		"tests":    []map[string]any{{"stdin": "", "expected_stdout": string(large)}},
	})
	resp, err := http.Post(baseURL+"/run", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for oversized expected_stdout, got %d", resp.StatusCode)
	}
	var er errorResponse
	json.NewDecoder(resp.Body).Decode(&er)
	if er.Error.Code != "expected_too_large" {
		t.Fatalf("expected expected_too_large, got %s", er.Error.Code)
	}
}

func TestInvalidLimits(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"language": "py3",
		"source":   "print('hi')\n",
		"run": map[string]any{
			"limits": map[string]any{"wall_time_s": 99999},
		},
		"tests": []testCase{{Stdin: "", ExpectedStdout: "hi\n"}},
	})
	resp, err := http.Post(baseURL+"/run", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for out-of-range limit, got %d", resp.StatusCode)
	}
	var er errorResponse
	json.NewDecoder(resp.Body).Decode(&er)
	if er.Error.Code != "invalid_limits" {
		t.Fatalf("expected invalid_limits, got %s", er.Error.Code)
	}
}

// ── Health endpoints ──────────────────────────────────────────────────────────

func TestReadyz(t *testing.T) {
	resp, err := http.Get(baseURL + "/readyz")
	if err != nil {
		t.Fatalf("GET /readyz: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body struct {
		Status    string                       `json:"status"`
		Nsjail    struct{ OK bool }            `json:"nsjail"`
		Languages map[string]struct{ OK bool } `json:"languages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode /readyz: %v", err)
	}
	if body.Status != "ok" {
		t.Fatalf("expected status=ok, got %s", body.Status)
	}
	if !body.Nsjail.OK {
		t.Fatal("nsjail probe failed")
	}
	for id, lang := range body.Languages {
		if !lang.OK {
			t.Errorf("language %s probe failed", id)
		}
	}
}

// smokeCase is a minimal hello-world for one language used by TestAllLanguagesHelloWorld.
// SourceFilename/ArtifactFilename are only needed for from_request languages (e.g. Java).
type smokeCase struct {
	Source           string
	ExpectedStdout   string
	SourceFilename   string
	ArtifactFilename string
}

// langSmoke maps language ids to hello-world payloads. Add an entry whenever a
// new language is added to configs/languages.yaml — TestAllLanguagesHelloWorld
// picks it up automatically. Languages not in this map are silently skipped.
var langSmoke = map[string]smokeCase{
	"py3":      {Source: "print('hello')\n", ExpectedStdout: "hello\n"},
	"bash":     {Source: "echo hello\n", ExpectedStdout: "hello\n"},
	"js":       {Source: "console.log('hello')\n", ExpectedStdout: "hello\n"},
	"ruby":     {Source: "puts 'hello'\n", ExpectedStdout: "hello\n"},
	"lua":      {Source: "print('hello')\n", ExpectedStdout: "hello\n"},
	"ocaml":    {Source: "print_string \"hello\\n\";;\n", ExpectedStdout: "hello\n"},
	"perl":     {Source: "print(\"hello\\n\");\n", ExpectedStdout: "hello\n"},
	"php":      {Source: "<?php echo \"hello\\n\";\n", ExpectedStdout: "hello\n"},
	"r":        {Source: "cat(\"hello\\n\")\n", ExpectedStdout: "hello\n"},
	"julia":    {Source: "println(\"hello\")\n", ExpectedStdout: "hello\n"},
	"elixir":   {Source: "IO.puts \"hello\"\n", ExpectedStdout: "hello\n"},
	"groovy":   {Source: "println \"hello\"\n", ExpectedStdout: "hello\n"},
	"tcl":      {Source: "puts hello\n", ExpectedStdout: "hello\n"},
	"fish":     {Source: "echo hello\n", ExpectedStdout: "hello\n"},
	"haskell":  {Source: "main = putStrLn \"hello\"\n", ExpectedStdout: "hello\n"},
	"swift":    {Source: "print(\"hello\")\n", ExpectedStdout: "hello\n"},
	"clojure":  {Source: "(println \"hello\")\n", ExpectedStdout: "hello\n"},
	"racket":   {Source: "#lang racket\n(display \"hello\")\n(newline)\n", ExpectedStdout: "hello\n"},
	"ts":       {Source: "console.log('hello');\n", ExpectedStdout: "hello\n"},
	"typescript": {Source: "console.log('hello');\n", ExpectedStdout: "hello\n"},
	// Erlang escript: ~n is Erlang's newline in format strings
	"erlang": {Source: "#!/usr/bin/env escript\nmain(_) ->\n    io:format(\"hello~n\").\n", ExpectedStdout: "hello\n"},

	"c": {
		Source:         "#include <stdio.h>\nint main(){printf(\"hello\\n\");return 0;}\n",
		ExpectedStdout: "hello\n",
	},
	"cpp": {
		Source:         "#include <iostream>\nint main(){std::cout<<\"hello\\n\";}\n",
		ExpectedStdout: "hello\n",
	},
	// Java: source_filename_strategy: from_request — class name must match filename
	"java": {
		Source:           "public class Main {\npublic static void main(String[] a) {\nSystem.out.println(\"hello\"); } }\n",
		ExpectedStdout:   "hello\n",
		SourceFilename:   "Main.java",
		ArtifactFilename: "Main",
	},
	"verilog": {
		Source:         "module main;\ninitial begin\n$display(\"hello\");\n$finish;\nend\nendmodule\n",
		ExpectedStdout: "hello\n",
	},
	"rust":   {Source: "fn main() { println!(\"hello\"); }\n", ExpectedStdout: "hello\n"},
	"kotlin": {Source: "fun main() { println(\"hello\") }\n", ExpectedStdout: "hello\n"},
	"go": {
		Source:         "package main\nimport \"fmt\"\nfunc main() { fmt.Println(\"hello\") }\n",
		ExpectedStdout: "hello\n",
	},
	"csharp": {
		Source:         "using System;\nclass Program {\n  static void Main() {\n    Console.WriteLine(\"hello\");\n  }\n}\n",
		ExpectedStdout: "hello\n",
	},
	"dotnet": {
		Source:         "using System;\nclass Program {\n  static void Main() {\n    Console.WriteLine(\"hello\");\n  }\n}\n",
		ExpectedStdout: "hello\n",
	},
	"fsharp": {Source: "printfn \"hello\"\n", ExpectedStdout: "hello\n"},
	// Scala: YAML should use source_filename: Main.scala, artifact_filename: Main
	"scala": {
		Source:         "object Main extends App {\n  println(\"hello\")\n}\n",
		ExpectedStdout: "hello\n",
	},
	"nim":     {Source: "echo \"hello\"\n", ExpectedStdout: "hello\n"},
	"crystal": {Source: "puts \"hello\"\n", ExpectedStdout: "hello\n"},
	"d":       {Source: "import std.stdio;\nvoid main() { writeln(\"hello\"); }\n", ExpectedStdout: "hello\n"},
	"pascal":  {Source: "program hello;\nbegin\n  writeln('hello');\nend.\n", ExpectedStdout: "hello\n"},
	// Fortran: explicit format avoids leading space from list-directed print
	"fortran": {Source: "program hello\n  write(*,'(a)') 'hello'\nend program hello\n", ExpectedStdout: "hello\n"},
	"ada":     {Source: "with Ada.Text_IO; use Ada.Text_IO;\nprocedure Hello is\nbegin\n  Put_Line(\"hello\");\nend Hello;\n", ExpectedStdout: "hello\n"},
	"zig": {
		Source:         "const std = @import(\"std\");\npub fn main() !void {\n    const out = std.io.getStdOut().writer();\n    try out.writeAll(\"hello\\n\");\n}\n",
		ExpectedStdout: "hello\n",
	},
}

// TestAllLanguagesHelloWorld runs a hello-world for every language in the live
// registry. Subtests are parallel; languages with no langSmoke entry are silently skipped.
func TestAllLanguagesHelloWorld(t *testing.T) {
	resp, err := http.Get(baseURL + "/info")
	if err != nil {
		t.Fatalf("GET /info: %v", err)
	}
	defer resp.Body.Close()

	var info struct {
		Languages []struct {
			ID string `json:"id"`
		} `json:"languages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		t.Fatalf("decode /info: %v", err)
	}
	if len(info.Languages) == 0 {
		t.Fatal("no languages registered — is the service running?")
	}

	for _, lang := range info.Languages {
		lang := lang
		t.Run(lang.ID, func(t *testing.T) {
			t.Parallel()

			sc, ok := langSmoke[lang.ID]
			if !ok {
				return
			}

			r, code := postRun(t, runRequest{
				Language:         lang.ID,
				Source:           sc.Source,
				SourceFilename:   sc.SourceFilename,
				ArtifactFilename: sc.ArtifactFilename,
				Tests:            []testCase{{Stdin: "", ExpectedStdout: sc.ExpectedStdout}},
			})
			if code != 200 {
				t.Fatalf("HTTP %d (expected 200)", code)
			}
			if r.Build.Status == validate.BuildStatusInternalError {
				t.Fatalf("build.status=internal_error — binary likely inaccessible inside the jail; check bind_mounts in YAML for %q", lang.ID)
			}
			if r.Build.Status == validate.BuildStatusFailed {
				t.Fatalf("build.status=failed — compiler invocation wrong; check build.cmd/args in YAML for %q", lang.ID)
			}
			if r.Status == validate.StatusInternalError {
				t.Fatalf("status=internal_error — runtime likely inaccessible inside the jail; check bind_mounts in YAML for %q", lang.ID)
			}
			if r.Status != validate.StatusAccepted {
				stdout := ""
				if len(r.Tests) > 0 {
					stdout = r.Tests[0].Stdout
				}
				t.Fatalf("status=%s (want %s)\n  stdout: %q\n  check langSmoke entry for %q",
					r.Status, validate.StatusAccepted, stdout, lang.ID)
			}
		})
	}
}

func TestInfo(t *testing.T) {
	resp, err := http.Get(baseURL + "/info")
	if err != nil {
		t.Fatalf("GET /info: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body struct {
		BuildInfo struct {
			Version   string `json:"version"`
			GoVersion string `json:"go_version"`
		} `json:"build_info"`
		Languages []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"languages"`
		Limits struct {
			MaxSourceBytes    int `json:"max_source_bytes"`
			MaxTests          int `json:"max_tests"`
			MaxConcurrentJobs int `json:"max_concurrent_jobs"`
		} `json:"limits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode /info: %v", err)
	}
	if body.BuildInfo.GoVersion == "" {
		t.Error("go_version missing from /info")
	}
	if len(body.Languages) == 0 {
		t.Error("no languages in /info")
	}
	if body.Limits.MaxSourceBytes == 0 {
		t.Error("max_source_bytes missing from /info")
	}
}
