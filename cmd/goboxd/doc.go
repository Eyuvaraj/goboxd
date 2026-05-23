// Package main is the entry point for goboxd.
//
// @title           goboxd API
// @version         1.0.0
// @description     Sandboxed code-execution service. Compiles and runs source code inside an nsjail jail against a set of test cases.
// @description
// @description     **Execution flow:** Each `POST /run` request spins up an isolated nsjail jail, optionally builds the source, runs all test cases, then tears down the jail. Results include per-test status, stdout, stderr, timing, and peak memory.
// @description
// @description     **Resource limits** (wall time, memory, max processes) are enforced per phase via nsjail cgroups and can be partially overridden per-request within language-defined allowlists.
// @description
// @description     **HTTP status codes:** The API returns `200` for all valid requests — execution results (build failure, TLE, wrong output) are encoded in body status fields, not HTTP codes. Only `400` (validation), `500` (server fault), and `503` (capacity/cancelled) indicate HTTP-level errors.
//
// @license.name    MIT
//
// @host            localhost:8080
// @BasePath        /
//
// @tag.name        execution
// @tag.description Submit source code for sandboxed compilation and test execution
// @tag.name        health
// @tag.description Service liveness, readiness, and operational metadata
package main
