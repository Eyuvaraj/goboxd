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

type runRequest struct {
	Language         string        `json:"language"`
	Source           string        `json:"source"`
	SourceFilename   string        `json:"source_filename,omitempty"`
	ArtifactFilename string        `json:"artifact_filename,omitempty"`
	Tests            []testCase    `json:"tests"`
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
