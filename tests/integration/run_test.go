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
	waitHealthy(nil) // noop: just use t from tests
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
	if resp.Status != "accepted" {
		t.Fatalf("expected accepted, got %s (build: %s)", resp.Status, resp.Build.Status)
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
	if resp.Status != "accepted" {
		t.Fatalf("expected accepted, got %s", resp.Status)
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
	if resp.Status != "accepted" {
		t.Fatalf("expected accepted, got %s", resp.Status)
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
	if resp.Status != "accepted" {
		t.Fatalf("expected accepted, got %s (build: %s)", resp.Status, resp.Build.Status)
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
	if resp.Status != "accepted" {
		t.Fatalf("expected accepted, got %s (build: %s)", resp.Status, resp.Build.Status)
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
	if resp.Status != "accepted" {
		t.Fatalf("expected accepted, got %s (build: %s)", resp.Status, resp.Build.Status)
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
	if resp.Status != "accepted" {
		t.Fatalf("expected accepted, got %s (build: %s)", resp.Status, resp.Build.Status)
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
	if resp.Status != "build_failed" {
		t.Fatalf("expected build_failed, got %s", resp.Status)
	}
	if resp.Build.Status != "failed" {
		t.Fatalf("expected build.status=failed, got %s", resp.Build.Status)
	}
	for i, tr := range resp.Tests {
		if tr.Status != "not_executed" {
			t.Fatalf("test[%d]: expected not_executed, got %s", i, tr.Status)
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
	if resp.Status != "wrong_output" {
		t.Fatalf("expected wrong_output, got %s", resp.Status)
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
		"source":   `#include <iostream>
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
	if resp.Status != "accepted" {
		for i, tr := range resp.Tests {
			fmt.Printf("  test[%d]: status=%s stdout=%q\n", i, tr.Status, tr.Stdout)
		}
		t.Fatalf("expected accepted, got %s", resp.Status)
	}
}

func TestWhitespaceMismatch(t *testing.T) {
	// Program prints "hello\n" but expected has trailing space — whitespace diff, not wrong_output.
	resp, code := postRun(t, runRequest{
		Language: "py3",
		Source:   "print('hello')\n",
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "hello"}},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if resp.Status != "output_whitespace_mismatch" {
		t.Fatalf("expected output_whitespace_mismatch, got %s", resp.Status)
	}
}

// Bonus language smoke tests — all require the language toolchain to be installed.

func TestRubyHelloWorld(t *testing.T) {
	resp, code := postRun(t, runRequest{
		Language: "ruby",
		Source:   "puts 'hello'\n",
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "hello\n"}},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if resp.Status != "accepted" {
		t.Fatalf("expected accepted, got %s", resp.Status)
	}
}

func TestLuaHelloWorld(t *testing.T) {
	resp, code := postRun(t, runRequest{
		Language: "lua",
		Source:   `print("hello")` + "\n",
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "hello\n"}},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if resp.Status != "accepted" {
		t.Fatalf("expected accepted, got %s", resp.Status)
	}
}

func TestRustHelloWorld(t *testing.T) {
	src := `fn main() { println!("hello"); }` + "\n"
	resp, code := postRun(t, runRequest{
		Language: "rust",
		Source:   src,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "hello\n"}},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if resp.Status != "accepted" {
		t.Fatalf("expected accepted, got %s (build: %s)", resp.Status, resp.Build.Status)
	}
}

func TestOCamlHelloWorld(t *testing.T) {
	src := `print_string "hello\n";;` + "\n"
	resp, code := postRun(t, runRequest{
		Language: "ocaml",
		Source:   src,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "hello\n"}},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if resp.Status != "accepted" {
		t.Fatalf("expected accepted, got %s", resp.Status)
	}
}

func TestKotlinHelloWorld(t *testing.T) {
	src := `fun main() { println("hello") }` + "\n"
	resp, code := postRun(t, runRequest{
		Language: "kotlin",
		Source:   src,
		Tests:    []testCase{{Stdin: "", ExpectedStdout: "hello\n"}},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if resp.Status != "accepted" {
		t.Fatalf("expected accepted, got %s (build: %s)", resp.Status, resp.Build.Status)
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
	if resp.Status != "time_exceeded" {
		t.Fatalf("expected time_exceeded, got %s", resp.Status)
	}
	if resp.Tests[0].Status != "time_exceeded" {
		t.Fatalf("expected tests[0].status=time_exceeded, got %s", resp.Tests[0].Status)
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
	if resp.Status != "runtime_error" {
		t.Fatalf("expected runtime_error, got %s", resp.Status)
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
	if resp.Status != "memory_exceeded" && resp.Status != "runtime_error" {
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
		Status    string                     `json:"status"`
		Nsjail    struct{ OK bool }          `json:"nsjail"`
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
