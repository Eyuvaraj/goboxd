# The 7 Deliberate Loopholes in pyjail (and their Go fixes)

The Python reference (`pyjail/`) contains seven intentional security holes. All seven are closed in `goboxd`.

---

## 1. Path traversal via filename

**Where:** `code_manager.py:117–119`  
**What:** The client-supplied `filename` field is written directly into the language config — `language_settings.filename = self.request.code.filename` — with no validation. A value like `../../etc/passwd` escapes the jail directory on the host.  
**Go fix:** `internal/validate/request.go` — `validate.Filename()` rejects absolute paths, slashes, leading dots, non-ASCII chars, and names longer than 64 chars.

---

## 2. Shell injection via `os.system`

**Where:** `code_manager.py:103, 135, 193`  
**What:** Directories are created and deleted by formatting shell strings — `os.system(f"mkdir -p {self.root_directory}/proc")`, `os.system(f"rm -rf {self.root_directory}")`. Any special character in a path component becomes a shell injection.  
**Go fix:** `internal/sandbox/workspace.go` — uses `os.MkdirTemp`, `os.MkdirAll`, `os.RemoveAll`. Zero shell invocations for filesystem operations.

---

## 3. Compiler-flag injection

**Where:** `code_runner.py:168–172`  
**What:** Client-supplied `extra_args` are joined straight into the compiler argv without filtering. Flags like `-fplugin=evil.so` or `-B/tmp` give compile-time code execution.  
**Go fix:** `internal/validate/request.go` — `validate.Flags()` checks every flag against a per-language `flag_allowlist` from `configs/languages.yaml`. Anything not on the list returns HTTP 400.

---

## 4. No request size limits

**Where:** `server.py:45–52`  
**What:** `reqparse.add_argument("message", ...)` has no `max_length`. Source, stdin, expected output, and evaluation script are all unbounded — a large body can exhaust memory or disk.  
**Go fix:** Four layers — HTTP body cap (`http.MaxBytesReader`), source size check, per-test stdin size check, stdout capped with `io.LimitReader` + `\n[output truncated]` marker.

---

## 5. UID collisions under load

**Where:** `code_manager.py:74–76`  
**What:** A UID is picked randomly from a 30 000-wide range with only three retries on collision. Under concurrent load two requests can share the same directory, letting one overwrite the other's source or stdin.  
**Go fix:** `internal/sandbox/workspace.go` — `os.MkdirTemp` lets the OS kernel guarantee uniqueness atomically. No UID scheme, no collision possible.

---

## 6. Unbounded child output in memory

**Where:** `code_runner.py:301`  
**What:** `out, err = self.run_command(args)` buffers the child's full stdout into memory. A program printing gigabytes OOMs the host process.  
**Go fix:** `internal/sandbox/nsjail.go` — stdout is read through `io.LimitReader(stdoutPipe, maxOutputBytes+1)`. Excess bytes are drained to `io.Discard` so the child is not blocked.

---

## 7. Stale jail directories on panic / early return

**Where:** `server.py` / `code_manager.py` — no guaranteed cleanup  
**What:** There is no `finally` block or equivalent around the per-request directory lifecycle. A panic or early-return between directory creation and cleanup leaks the directory permanently, filling disk over time.  
**Go fix (two mechanisms):**
- `internal/runner/runner.go` — `defer ws.Cleanup()` is set immediately after `NewWorkspace()`, so it runs on every exit path including recovered panics.
- `internal/sandbox/workspace.go` — `SweepOrphans()` is called once at startup to remove any `goboxd-*` directories older than 30 minutes left by a prior unclean shutdown.

---

## Summary table

| # | Hole | pyjail location | Go fix location |
|---|------|----------------|-----------------|
| 1 | Path traversal via filename | `code_manager.py:117` | `internal/validate/request.go` |
| 2 | Shell injection via `os.system` | `code_manager.py:103,135,193` | `internal/sandbox/workspace.go` |
| 3 | Compiler-flag injection | `code_runner.py:168` | `internal/validate/request.go` |
| 4 | No request size limits | `server.py:45` | `middleware.go`, `validate`, `nsjail.go` |
| 5 | UID collisions under load | `code_manager.py:74` | `internal/sandbox/workspace.go` |
| 6 | Unbounded child output | `code_runner.py:301` | `internal/sandbox/nsjail.go` |
| 7 | Stale jail directories | (no cleanup guarantee) | `runner.go` + `workspace.go` |
