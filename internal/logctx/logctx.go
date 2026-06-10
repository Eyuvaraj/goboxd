// Package logctx carries per-request execution fields from the run handler
// to the structured-logger middleware after ServeHTTP returns.
package logctx

import "context"

type key struct{}

type Fields struct {
	Language        string
	ExecStatus      string
	BuildDurationMs int64
	TotalCpuMs      int64
	TestsTotal      int
	TestsAccepted   int
}

func Set(ctx context.Context, f Fields) context.Context {
	return context.WithValue(ctx, key{}, f)
}

func Get(ctx context.Context) Fields {
	if f, ok := ctx.Value(key{}).(Fields); ok {
		return f
	}
	return Fields{}
}
