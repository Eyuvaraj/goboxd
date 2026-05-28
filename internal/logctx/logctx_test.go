package logctx_test

import (
	"context"
	"testing"

	"github.com/thesouldev/goboxd/internal/logctx"
)

func TestGetReturnsZeroOnEmptyContext(t *testing.T) {
	f := logctx.Get(context.Background())
	if f.Language != "" || f.ExecStatus != "" || f.TestsTotal != 0 {
		t.Errorf("expected zero Fields on empty context, got %+v", f)
	}
}

func TestSetGetRoundtrip(t *testing.T) {
	want := logctx.Fields{
		Language:        "py3",
		ExecStatus:      "accepted",
		BuildDurationMs: 412,
		TestsTotal:      3,
		TestsAccepted:   3,
	}
	ctx := logctx.Set(context.Background(), want)
	got := logctx.Get(ctx)
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestSetDoesNotMutateParentContext(t *testing.T) {
	parent := context.Background()
	child := logctx.Set(parent, logctx.Fields{Language: "cpp"})

	if logctx.Get(parent).Language != "" {
		t.Error("Set mutated the parent context")
	}
	if logctx.Get(child).Language != "cpp" {
		t.Error("Set did not store value in child context")
	}
}
