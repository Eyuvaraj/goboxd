// Package logctx carries per-request log fields between the handler and the
// structured-logger middleware. The handler writes fields before returning;
// the middleware reads them after next.ServeHTTP() completes.
package logctx

import "context"

type key struct{}

// Fields holds the execution-level fields a handler wants logged.
type Fields struct {
	Language       string
	ExecStatus     string
	BuildDurationMs int64
	TestsTotal     int
	TestsAccepted  int
}

// Set stores fields in ctx.
func Set(ctx context.Context, f Fields) context.Context {
	return context.WithValue(ctx, key{}, f)
}

// Get retrieves fields from ctx. Returns zero value if not set.
func Get(ctx context.Context) Fields {
	if f, ok := ctx.Value(key{}).(Fields); ok {
		return f
	}
	return Fields{}
}
