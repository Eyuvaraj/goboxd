# AI Usage Log

This document tracks AI interactions during the development of goboxd, as required by the hackathon specification.

---

## 2026-05-14 [Planning out how to structure the whole project]

**Prompt:** I have to build a Go HTTP service that takes code, runs it inside a sandbox, and returns results. I'm new to Go but have a basic understanding of how systems work. I have a repo with empty folders. Where do I start and how should I break this up?

**Response summary:**
- Split the project into separate folders: handlers, runner, sandbox, validation, config.
- Use a YAML file for languages instead of hardcoding them — adding a new one then requires no Go code change.
- Use `chi` as the HTTP router because it's lightweight and plays nicely with standard library middleware.

**What we used / didn't use:**
- Used the package layout almost exactly as described — it matched what the spec wanted.
- Skipped a dedicated `metrics` package; `sync/atomic` counters were enough.

---

## 2026-05-23 [Writing the first working version of everything]

**Prompt:** Can you help me implement this? I need: loading languages from YAML, a POST /run handler with input validation, a runner that limits how many jobs run at once, a sandbox package that builds the nsjail command, a status parser for what happened, health endpoints (/healthz, /readyz, /info), integration tests for each language, and a load test script.

**Response summary:**
- Generated the first working skeleton for all main packages in one session — config, registry, handler, runner, sandbox, validate, stats.
- Wrote the full `configs/languages.yaml` with all seven required languages and correct build/run shapes.
- Produced integration tests for Python, C, C++, Java, Bash, Node.js, and Verilog.

**What we used / didn't use:**
- Almost all of it was used directly — this formed the first real version of the project (~43 new files).
- Several bugs were found and fixed afterward (logged below), but the overall structure came from this session.
- Skipped the Prometheus metrics suggestion — overkill for a hackathon.

---

## 2026-05-23 [Running jobs and limiting server load]

**Prompt:** I need to run nsjail from my Go code, but I want to make sure it doesn't crash my server. Also, how do I make sure my server only runs a few jobs at a time so it doesn't freeze under heavy load?

**Response summary:**
- Use `io.LimitReader` to cap how much output the child process can write before it gets cut off.
- Use a buffered Go channel as a "waiting line" — only a fixed number of jobs run at once, the rest wait.

**What we used / didn't use:**
- Used both exactly as shown — no extra packages needed.

---

## 2026-05-23 [Adding Swagger API docs]

**Prompt:** I want to add Swagger documentation to my Go API so users can see the endpoints and test them. What's the easiest way to do this without writing everything by hand?

**Response summary:**
- Use `swaggo/swag` — it reads comments above Go functions and generates the Swagger files automatically.
- Gave example comment formats for routes.

**What we used / didn't use:**
- Used `swaggo/swag` because it keeps docs next to the code.
- Didn't serve a Swagger UI page at this stage — added that later on May 28.

---

## 2026-05-23 [Fixing file permissions]

**Prompt:** When I create temporary folders for the sandbox, I want to make sure other users on the server can't read the files inside. How do I set strict folder permissions in Go?

**Response summary:**
- `os.MkdirTemp` uses `0700` permissions by default — only the owner can read it.
- `os.Chmod` can lock down individual files written into the folder.

**What we used / didn't use:**
- Kept the safe defaults from `MkdirTemp` rather than manually calling `Chmod` everywhere.

---

## 2026-05-24 [Adding five bonus languages at once]

**Prompt:** The competition gives bonus points for extra languages beyond the required seven. I want to add Ruby, Lua, Rust, Kotlin, and OCaml. Can you write the YAML config blocks, the Dockerfile install lines, and the install shell scripts for each one?

**Response summary:**
- Wrote YAML configs for all five, including the right compiler commands and argument templates.
- For Kotlin: build with `kotlinc` into a `.jar`, run with `java -jar` — it runs on the JVM.
- Each language needs a working `--version` command so `/readyz` can confirm it's installed.

**What we used / didn't use:**
- Used all five configs with small path tweaks inside Docker.
- Rust had a slow compile time (expected) — noted that in the language docs.

---

## 2026-05-24 [Securing the sandbox]

**Prompt:** I need to make my nsjail sandbox secure. How do I block dangerous system calls? Also, how can I tell if a program got killed because it used too much memory?

**Response summary:**
- Provided a Kafel policy string for `--seccomp_string` to block dangerous syscalls (ptrace, bpf, io_uring, kexec, etc.).
- When cgroup v2 kills a process for using too much memory, it sends signal 9 (SIGKILL) — checking for that lets you return `memory_exceeded` instead of a generic error.

**What we used / didn't use:**
- Used the syscall block list and the signal 9 check as given.

---

## 2026-05-24 [Fixing bugs with logs and language order]

**Prompt:** I have two weird bugs. My code is marking everything as an internal error because it finds the word "nsjail" anywhere in the logs. Also, my API returns languages in a random order every time. How do I fix these?

**Response summary:**
- Go maps loop in random order by design — keep a separate ordered slice populated at load time.
- Real nsjail errors start with `[E][`, not just any line containing "nsjail".

**What we used / didn't use:**
- Added an ordered slice for languages; changed the log check to look for `[E][`.
- Skipped a third-party sorted-map package — the slice approach was simpler.

---

## 2026-05-24 [Upgrading Go in Docker]

**Prompt:** My linter is failing inside my Docker build. It says golangci-lint v2 requires Go >= 1.25 but I think my image has an older version. How do I upgrade it?

**Response summary:**
- Change `FROM golang:1.24` to `FROM golang:1.26-bookworm` — the `bookworm` tag uses a newer Linux base.

**What we used / didn't use:**
- Used `golang:1.26-bookworm` exactly; linter passed immediately.
- Didn't pin to an exact patch version (1.26.x) — easier to let it grab the latest patch.

---

## 2026-05-24 [Capping user-supplied limits]

**Prompt:** My API lets users send custom time and memory limits for their code. But right now, someone could send a time limit of 99999 seconds and freeze up a slot in the queue forever. How do I stop this?

**Response summary:**
- Validate the user's requested limit against the language's default — reject anything more than double the default.
- Return a 400 error with a clear message if the limit is too high.

**What we used / didn't use:**
- Used the "double the default" rule — fair balance between flexibility and safety.

---

## 2026-05-24 [Tracking the queue size]

**Prompt:** I am using a channel to limit how many jobs run at once. How can I safely check how many jobs are currently waiting so I can show it on my /info endpoint?

**Response summary:**
- `len(channel)` gives the number of currently occupied slots in a buffered channel.
- `sync/atomic` counters are safe for tracking totals across concurrent requests.

**What we used / didn't use:**
- Used atomic counters for totals and `len(channel)` for active queue size — made `/info` stats much more useful.

---

## 2026-05-26 [Adding Go language support]

**Prompt:** I'm trying to let users run Go code in my sandbox, but it keeps failing because it's trying to download modules and there's no internet inside the jail. What environment variables do I need to set to make `go build` work completely offline?

**Response summary:**
- `GO111MODULE=off` — stops Go from looking for a go.mod or downloading modules.
- `CGO_ENABLED=0` — keeps the build simple, no C compiler needed.
- `GOCACHE=/.cache/go-build` — points the build cache to a folder the jail can write to.

**What we used / didn't use:**
- Added all three to the language's `env` list in `configs/languages.yaml` — Go compiled on the first try.

---

## 2026-05-28 [Debugging why integration tests were all failing]

**Prompt:** My integration tests are all failing but I can't figure out why. `/readyz` always says nsjail is broken even though it's installed. Compiled languages like C++ say their probe fails even though g++ is there. And running any compiled language gives `internal_error`. Can you look at the code and figure out what's happening?

**Response summary:**
- Bug 1 — nsjail probe: `nsjail --version` doesn't work; nsjail has no `--version` flag and always exits with code 255, which my code read as failure. Fix: check the binary exists with `os.Stat`, then run `nsjail --help` (exit 255 from `--help` is fine — it just means the binary ran).
- Bug 2 — compiler probe: For compiled languages, the code was checking if the *output binary* (`/solution`) exists, not the *compiler* (`/usr/bin/g++`). That file never exists at startup.
- Bug 3 — bind-mount crash: The code added the directory containing `/solution` to nsjail's bind-mounts. Since `filepath.Dir("/solution")` is `/`, it tried to mount the entire root filesystem — nsjail logged that as `[E][`, which my status parser read as `internal_error`.

**What we used / didn't use:**
- All three fixes were used; tests went from all-failing to all-passing.
- The bind-mount one was completely non-obvious — wouldn't have spotted it without walking through it step by step.

---

## 2026-05-28 [Reading peak memory usage after a job finishes]

**Prompt:** My API always shows `memory_peak_kb: 0` in the response. The spec example shows a real number there. How do I actually track peak memory for each sandbox job?

**Response summary:**
- With cgroup v2, Linux writes a file called `memory.peak` inside the job's cgroup folder after the process exits.
- Name each job's cgroup uniquely, then read that file after `cmd.Wait()` returns and divide by 1024.
- Test inside the Docker container, not on the host — cgroup v2 paths differ depending on how Docker is set up.

**What we used / didn't use:**
- Used this approach; had to adjust the cgroup path slightly for Docker, but the core idea was correct.
- Now every response shows a real `memory_peak_kb` instead of 0.

---

## 2026-05-28 [Building an interactive playground to test the API]

**Prompt:** I want to add a simple web page at /playground where I can type code, pick a language, and click run to see the output — so I can test the API without using curl every time. Can you build a self-contained HTML page for this?

**Response summary:**
- A single HTML file with no external dependencies: code text area, language dropdown, run button, output panel.
- Uses the browser's `fetch()` to call `POST /run` directly.
- Also suggested adding Swagger UI at `/docs/` so judges can browse the full API spec interactively.

**What we used / didn't use:**
- Used the HTML as a starting point and cleaned up styling afterward.
- Both playground and Swagger UI are embedded in the Go binary using the `embed` package — no extra files at runtime.

---

## 2026-05-28 [Java crashing on Device]

**Prompt:** My Java programs are crashing in the sandbox with an error about compressed class space on my Device. My memory limit is set to 512 MB. Why is this happening and how do I fix it?

**Response summary:**
- Java on ARM64 reserves ~1 GB of *virtual* memory just to start up, even if it doesn't use it.
- Raise `RLIMIT_AS` (virtual address space limit) to something like 4096 MB — cgroup memory.max still enforces real RAM usage.

**What we used / didn't use:**
- Raised the virtual memory limit to 4096 MB; Java started working immediately.
- The cgroup limit still correctly stops programs that try to use too much real memory.

---

## 2026-05-28 [Writing tests and checking code]

**Prompt:** can you build and test all the apis and its output and verify them and do the needful updates... and audit the code and write tests

**Response summary:**
- Noticed there were no tests for the API handlers — wrote tests for bad JSON, missing filenames, oversized source, etc.
- Caught a bug: code was trimming leading spaces from output, but the spec says only trailing spaces should be trimmed.

**What we used / didn't use:**
- Used all the generated tests — they covered edge cases I hadn't thought of.
- Used the `TrimRight` fix so output comparison follows the spec exactly.
