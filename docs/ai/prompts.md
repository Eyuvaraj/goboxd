# AI Usage Log

Tracks AI interactions during the development of goboxd, as required by the hackathon specification.

---

## 2026-05-23: Writing the first working version of everything

**Prompt:** Can you help me implement this? I need: loading languages from YAML, a POST /run handler with input validation, a runner that limits how many jobs run at once, a sandbox package that builds the nsjail command, a status parser for what happened, health endpoints (/healthz, /readyz, /info), integration tests for each language, and a load test script.

**Response summary:**
- Generated the first working skeleton for all main packages in one session: config, registry, handler, runner, sandbox, validate, stats.
- Wrote the full `configs/languages.yaml` with all seven required languages and correct build/run shapes.
- Produced integration tests for Python, C, C++, Java, Bash, Node.js, and Verilog.

**What we used / didn't use:** Almost all of it was used directly, forming the first real version of the project (~43 new files). Several bugs were found and fixed afterward (logged below). Skipped the Prometheus metrics suggestion; overkill for a hackathon.

---

## 2026-05-23: Bounding concurrency and capping output

**Prompt:** I need to run nsjail from my Go code, but I want to make sure it doesn't crash my server. Also, how do I make sure my server only runs a few jobs at a time so it doesn't freeze under heavy load?

**Response summary:**
- Use `io.LimitReader` to cap how much output the child process can write before it gets cut off.
- Use a buffered Go channel as a counting semaphore; only a fixed number of jobs run at once, the rest wait.

**What we used / didn't use:** Used both exactly as described. No extra packages needed.

---

## 2026-05-23: Adding Swagger API docs

**Prompt:** I want to add Swagger documentation to my Go API so users can see the endpoints and test them. What's the easiest way to do this without writing everything by hand?

**Response summary:**
- Use `swaggo/swag`; it reads comments above Go functions and generates the Swagger files automatically.
- Provided example comment formats for routes.

**What we used / didn't use:** Used `swaggo/swag` because it keeps docs next to the code. Deferred serving a Swagger UI page until May 28.

---

## 2026-05-23: Setting workspace permissions

**Prompt:** When I create temporary folders for the sandbox, I want to make sure other users on the server can't read the files inside. How do I set strict folder permissions in Go?

**Response summary:**
- `os.MkdirTemp` uses `0700` permissions by default; only the owner can read it.
- `os.Chmod` can lock down individual files written into the folder.

**What we used / didn't use:** Kept the safe defaults from `MkdirTemp` rather than manually calling `Chmod` everywhere.

---

## 2026-05-24: Adding five additional languages

**Prompt:** The competition gives bonus points for extra languages beyond the required seven. I want to add Ruby, Lua, Rust, Kotlin, and OCaml. Can you write the YAML config blocks, the Dockerfile install lines, and the install shell scripts for each one?

**Response summary:**
- Wrote YAML configs for all five, including the correct compiler commands and argument templates.
- For Kotlin: build with `kotlinc` into a `.jar`, run with `java -jar`; it runs on the JVM.
- Each language needs a working `--version` command so `/readyz` can confirm it's installed.

**What we used / didn't use:** Used all five configs with small path tweaks inside Docker. Rust's slow compile time was expected and is noted in the language docs.

---

## 2026-05-24: Securing the sandbox

**Prompt:** I need to make my nsjail sandbox secure. How do I block dangerous system calls? Also, how can I tell if a program got killed because it used too much memory?

**Response summary:**
- Provided a Kafel policy string for `--seccomp_string` to block dangerous syscalls (ptrace, bpf, io_uring, kexec, etc.).
- When cgroup v2 kills a process for exceeding memory, it sends SIGKILL; checking for that distinguishes `memory_exceeded` from a generic runtime error.

**What we used / didn't use:** Used the syscall block list and the signal check as given.

---

## 2026-05-24: Fixing log parsing and language ordering

**Prompt:** I have two bugs. My code marks everything as an internal error because it finds the word "nsjail" anywhere in the logs. Also, my API returns languages in a random order every time. How do I fix these?

**Response summary:**
- Go maps iterate in random order by design; keep a separate ordered slice populated at load time.
- Real nsjail errors start with `[E][`, not just any line containing "nsjail".

**What we used / didn't use:** Added an ordered slice for languages; changed the log check to look for `[E][`. Skipped a third-party sorted-map package; the slice approach was simpler.

---

## 2026-05-24: Upgrading Go in Docker

**Prompt:** My linter is failing inside my Docker build. It says golangci-lint v2 requires Go >= 1.25 but I think my image has an older version. How do I upgrade it?

**Response summary:** Change `FROM golang:1.24` to `FROM golang:1.26-bookworm`; the `bookworm` tag uses a newer Linux base.

**What we used / didn't use:** Used `golang:1.26-bookworm` exactly; linter passed immediately.

---

## 2026-05-24: Capping user-supplied limits

**Prompt:** My API lets users send custom time and memory limits for their code. But right now, someone could send a time limit of 99999 seconds and freeze a slot in the queue forever. How do I stop this?

**Response summary:**
- Validate the user's requested limit against the language default; reject anything more than double the default.
- Return a 400 error with a clear message if the limit is too high.

**What we used / didn't use:** Used the "double the default" rule, a fair balance between flexibility and safety.

---

## 2026-05-24: Tracking queue size for /info

**Prompt:** I am using a channel to limit how many jobs run at once. How can I safely check how many jobs are currently waiting so I can show it on my /info endpoint?

**Response summary:**
- `len(channel)` gives the number of currently occupied slots in a buffered channel.
- `sync/atomic` counters are safe for tracking totals across concurrent requests.

**What we used / didn't use:** Used atomic counters for totals and `len(channel)` for active queue size; made `/info` stats much more useful.

---

## 2026-05-26: Running Go code offline inside the sandbox

**Prompt:** I'm trying to let users run Go code in my sandbox, but it keeps failing because it's trying to download modules and there's no internet inside the jail. What environment variables do I need to set to make `go build` work completely offline?

**Response summary:**
- `GO111MODULE=off`: stops Go from looking for a go.mod or downloading modules.
- `CGO_ENABLED=0`: keeps the build simple, no C compiler needed.
- `GOCACHE=/.cache/go-build`: points the build cache to a folder the jail can write to.

**What we used / didn't use:** Added all three to the language's `env` list in `configs/languages.yaml`; Go compiled on the first try.

---

## 2026-05-28: Debugging why integration tests were all failing

**Prompt:** My integration tests are all failing but I can't figure out why. `/readyz` always says nsjail is broken even though it's installed. Compiled languages like C++ say their probe fails even though g++ is there. And running any compiled language gives `internal_error`. Can you look at the code and figure out what's happening?

**Response summary:**
- **Bug 1 (nsjail probe):** `nsjail --version` doesn't exist; nsjail always exits 255, which my code read as failure. Fix: check the binary exists with `os.Stat`, then run `nsjail --help` (exit 255 from `--help` is acceptable; it just means the binary ran).
- **Bug 2 (compiler probe):** For compiled languages, the code was checking if the output binary (`/solution`) exists, not the compiler (`/usr/bin/g++`). That file never exists at startup.
- **Bug 3 (bind-mount crash):** The code added the directory containing `/solution` to nsjail's bind-mounts. Since `filepath.Dir("/solution")` is `/`, it tried to mount the entire root filesystem. nsjail logged that as `[E][`, which the status parser read as `internal_error`.

**What we used / didn't use:** All three fixes were applied; tests went from all-failing to all-passing. The bind-mount issue was completely non-obvious without stepping through it.

---

## 2026-05-28: Reading peak memory usage after a job finishes

**Prompt:** My API always shows `memory_peak_kb: 0` in the response. The spec example shows a real number there. How do I actually track peak memory for each sandbox job?

**Response summary:**
- With cgroup v2, Linux writes `memory.peak` inside the job's cgroup folder after the process exits.
- Name each job's cgroup uniquely, then read that file after `cmd.Wait()` returns and divide by 1024.
- Test inside the Docker container; cgroup v2 paths differ depending on how Docker is configured.

**What we used / didn't use:** Used this approach with a small path adjustment for Docker. Every response now shows a real `memory_peak_kb`.

---

## 2026-05-28: Building an interactive playground

**Prompt:** I want to add a simple web page at /playground where I can type code, pick a language, and click run to see the output, so I can test the API without using curl every time. Can you build a self-contained HTML page for this?

**Response summary:**
- A single HTML file with no external dependencies: code textarea, language dropdown, run button, output panel.
- Uses the browser's `fetch()` to call `POST /run` directly.
- Suggested adding Swagger UI at `/docs/` so judges can browse the full API spec interactively.

**What we used / didn't use:** Used the HTML as a starting point and cleaned up styling afterward. Both the playground and Swagger UI are embedded in the Go binary using the `embed` package.

---

## 2026-05-28: Java crashing on ARM64 with compressed class space error

**Prompt:** My Java programs are crashing in the sandbox with an error about compressed class space on my device. My memory limit is set to 512 MB. Why is this happening and how do I fix it?

**Response summary:**
- Java on ARM64 reserves ~1 GB of virtual memory just to start up, even if it doesn't use it.
- Raise `RLIMIT_AS` (virtual address space limit) to ~4096 MB; cgroup `memory.max` still enforces real RAM usage.

**What we used / didn't use:** Raised the virtual memory limit to 4096 MB; Java started working immediately. The cgroup limit still correctly stops programs that try to use too much real memory.

---

## 2026-05-28: Writing handler tests and auditing output comparison

**Prompt:** can you build and test all the apis and its output and verify them and do the needful updates... and audit the code and write tests

**Response summary:**
- Identified missing handler tests; wrote tests for bad JSON, missing filenames, oversized source, and other validation edge cases.
- Caught a bug: code was trimming leading spaces from output, but the spec says only trailing spaces should be trimmed.

**What we used / didn't use:** Used all the generated tests; they covered edge cases that hadn't been considered. Applied the `TrimRight` fix so output comparison follows the spec exactly.

---

## 2026-06-07: Concurrency / Go-usage audit

**Prompt:** Are we using Go concurrency feature well, can anything help here? Audit the code.

**Response summary:**
- Concurrency model is sound; no changes needed.
- gRPC: skip it. The spec fixes an HTTP+JSON contract and lists binary wire formats as out of scope.
- Two real gaps: child stderr was captured unbounded (host-OOM risk); `MAX_CONCURRENT_JOBS`/`GOMAXPROCS` used `NumCPU()`, which over-counts in a CPU-limited container.

**What we used / didn't use:** Capped stderr via `StderrPipe` + a shared `readCapped()` helper. Added `config.AvailableCPUs()` (reads cgroup v2 `cpu.max`, falls back to `NumCPU()`) for the semaphore default and `GOMAXPROCS` — preferred over a new `automaxprocs` dependency. Skipped gRPC.

---

## 2026-06-07: Load-test regression and correcting an old claim

**Prompt:** `make load` numbers dropped ~2x vs the committed benchmarks — find out why, and be honest about whether our concurrency design and writeup are correct.

**Response summary:**
- The container provisioning was unchanged (10 cores, no quota, `MAX_CONCURRENT_JOBS=10`), so it was not a code/config regression. During load the container peaked at ~233% CPU of 1000% available — 4x idle headroom — so the workload is **setup-latency-bound, not CPU-bound** for trivial programs.
- A throwaway instance at `MAX_CONCURRENT_JOBS=20` raised py3 throughput 257 → 562 req/s (2.2x) with CPU still half-idle. This **disproved** the `architecture.md` claim that "no queueing discipline can raise that ceiling" — true for compiled/CPU-bound work, false for setup-bound work.
- The ~2x benchmark drop was environmental: host contention (load avg ~2.2, active dev session, fanless M4 Air time-sharing the Docker VM), not a regression.

**What we used / didn't use:** Reworded the `architecture.md` concurrency justification to the honest two-regime explanation (CPU-bound vs setup-bound) instead of the absolute claim, and added a host-sensitivity caveat to `benchmarks.md`. Deliberately did **not** change the default (`= cores`) or any code — oversubscription multiplies peak memory and chasing a non-representative hello-world number would be gaming the benchmark.

---

## 2026-06-10: Full API-contract conformance sweep against a live container

**Prompt:** Hit the endpoints and check whether we comply with the API contract 100%.

**Response summary:**
- Ran the service (`make run`) and exercised every endpoint and every status/error path in the spec — ~22 cases. `/healthz`, `/readyz`, `/info` shapes all correct; `memory_peak_kb` returns real kernel values; every test status (accepted, wrong_output, output_whitespace_mismatch, time_exceeded, memory_exceeded, runtime_error, build_failed→not_executed) and the first-non-accepted ordering rule all correct; all 12 input error codes return 400 with the `{error:{code,message}}` shape; `GET /run` → 405 (not 5xx).
- One gray-area finding: a malformed `source_filename` sent to a fixed-filename language (e.g. cpp) returned `200` instead of `400`, because `resolveFilename` ignored the client value entirely when the strategy wasn't `from_request`. Not a security hole (the value is never path-joined for fixed-filename langs; the real `from_request`/Java vector was correctly rejected), but a strict reading of the contract ("must be a single path component") wants it rejected.

**What we used / didn't use:** Hardened `resolveFilename` (`handler/run.go`) to validate a non-empty client filename even for fixed-filename languages while still using the fixed name — a change that can only reject inputs that were previously silently ignored, so no valid request changes behaviour. Added a regression unit test (`TestRunHandler_FixedFilenameLang_RejectsMalformedFilename`). Verified end to end: rebuilt the image, re-ran the live sweep (the case now returns `400 invalid_filename`; valid/omitted filenames still run), `make test` green, `make integration` 50/0/1, `make lint` 0 issues. Sweep is now 22/22.