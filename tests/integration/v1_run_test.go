//go:build integration

package integration_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/thesouldev/goboxd/internal/validate"
)

type evaluatorSpec struct {
	Language string `json:"language"`
	Source   string `json:"source"`
}

type v1RunRequest struct {
	Language  string         `json:"language"`
	Source    string         `json:"source"`
	Stdin     string         `json:"stdin,omitempty"`
	Evaluator *evaluatorSpec `json:"evaluator,omitempty"`
	Tests     []testCase     `json:"tests"`
}

type v1Response struct {
	Status string `json:"status"`
	Build  struct {
		Status string `json:"status"`
	} `json:"build"`
	Tests []struct {
		Status   string   `json:"status"`
		Stdout   string   `json:"stdout"`
		ExitCode int      `json:"exit_code"`
		Verdict  string   `json:"verdict"`
		Score    *float64 `json:"score"`
		Message  string   `json:"message"`
	} `json:"tests"`
}

func postV1Run(t *testing.T, req v1RunRequest) (v1Response, int) {
	t.Helper()
	body, _ := json.Marshal(req)
	resp, err := http.Post(baseURL+"/v1/run", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /v1/run: %v", err)
	}
	defer resp.Body.Close()
	var r v1Response
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return r, resp.StatusCode
}

// Raw mode: empty tests, run once against stdin, no grading. exit_code present.
func TestV1RawExecution(t *testing.T) {
	resp, code := postV1Run(t, v1RunRequest{
		Language: "py3",
		Source:   "import sys; sys.stdout.write(sys.stdin.read().upper())\n",
		Stdin:    "hello\n",
		Tests:    []testCase{},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if len(resp.Tests) != 1 {
		t.Fatalf("raw mode should yield one result, got %d", len(resp.Tests))
	}
	if resp.Tests[0].Status != validate.StatusAccepted {
		t.Fatalf("raw status: want %s, got %q", validate.StatusAccepted, resp.Tests[0].Status)
	}
	if resp.Tests[0].Stdout != "HELLO\n" {
		t.Fatalf("raw stdout: want %q, got %q", "HELLO\n", resp.Tests[0].Stdout)
	}
}

// Evaluator mode: a custom checker accepts when output is double the input.
func TestV1EvaluatorMode(t *testing.T) {
	checker := `import json
inp = int(open('input').read().strip())
out = int(open('output').read().strip())
ok = out == inp * 2
print(json.dumps({"verdict": "accepted" if ok else "rejected", "score": 1.0 if ok else 0.0}))
`
	// Candidate doubles its input → evaluator accepts.
	resp, code := postV1Run(t, v1RunRequest{
		Language:  "py3",
		Source:    "import sys; print(int(sys.stdin.read().strip()) * 2)\n",
		Evaluator: &evaluatorSpec{Language: "py3", Source: checker},
		Tests:     []testCase{{Stdin: "5\n"}},
	})
	if code != 200 {
		t.Fatalf("expected HTTP 200, got %d", code)
	}
	if resp.Status != validate.StatusAccepted {
		t.Fatalf("top-level: want %s, got %q", validate.StatusAccepted, resp.Status)
	}
	if resp.Tests[0].Verdict != "accepted" {
		t.Fatalf("verdict: want accepted, got %q (status=%q msg=%q)", resp.Tests[0].Verdict, resp.Tests[0].Status, resp.Tests[0].Message)
	}

	// Candidate triples its input → evaluator rejects → wrong_output.
	resp, _ = postV1Run(t, v1RunRequest{
		Language:  "py3",
		Source:    "import sys; print(int(sys.stdin.read().strip()) * 3)\n",
		Evaluator: &evaluatorSpec{Language: "py3", Source: checker},
		Tests:     []testCase{{Stdin: "5\n"}},
	})
	if resp.Status != validate.StatusWrongOutput {
		t.Fatalf("top-level: want %s, got %q", validate.StatusWrongOutput, resp.Status)
	}
	if resp.Tests[0].Verdict != "rejected" {
		t.Fatalf("verdict: want rejected, got %q", resp.Tests[0].Verdict)
	}
}
