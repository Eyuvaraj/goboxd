# Configuration and Running

Everything goboxd reads from the environment, plus the container privileges nsjail needs. All settings are environment variables, parsed in `internal/config/config.go`.

---

## Environment Variables

| Variable | Default | Meaning |
|----------|---------|---------|
| `PORT` | `8080` | HTTP listen port |
| `NSJAIL_PATH` | `/usr/local/bin/nsjail` | Path to the nsjail binary; checked for existence and executability at startup and on `/readyz` |
| `JAIL_DIR` | `/tmp/goboxd` | Parent directory for per-job `MkdirTemp` workspaces |
| `LANGUAGE_FILE` | `/etc/goboxd/languages.yaml` | Path to the language registry YAML (the compose file bind-mounts `configs/languages.yaml` here) |
| `MAX_SOURCE_BYTES` | `262144` (256 KiB) | Maximum source size; larger requests get `400 source_too_large` |
| `MAX_TESTS` | `50` | Maximum test cases per request; more gets `400 invalid_test_count` |
| `MAX_CONCURRENT_JOBS` | `config.AvailableCPUs()` | Semaphore capacity. Default is the cgroup v2 CPU quota when set and lower than `runtime.NumCPU()`, else `NumCPU()`. Also used for `GOMAXPROCS`. See [concurrency.md](concurrency.md). |
| `MAX_OUTPUT_BYTES` | `262144` (256 KiB) | Per-phase captured stdout/stderr cap; beyond it output is truncated with `\n[output truncated]` |
| `MAX_STDIN_BYTES` | `65536` (64 KiB) | Per-test `stdin` cap; also the cap for each `expected_stdout` |
| `ORPHAN_MAX_AGE_MINUTES` | `30` | Age gate for the startup and periodic orphan-workspace sweep |
| `MAX_QUEUE_DEPTH` | `0` (unbounded) | Waiting requests allowed before load-shedding with `503 + Retry-After`. `0` disables shedding. |

**Parsing note.** Integer variables only override the default when set to a value greater than 0 (`envInt` in `config.go`). A missing, empty, non-numeric, zero, or negative value falls back to the default. `MAX_QUEUE_DEPTH=0` is the off state, not a way to turn it on.

---

## Runtime Requirements

nsjail builds Linux namespaces, cgroups, and a seccomp filter around each job. Docker's defaults block several of the operations that requires, so `docker-compose.yml` grants the minimum set. Without these, every `/run` fails (`build.internal_error` for compiled languages, `runtime_error` for interpreted ones, because nsjail exits before the child starts).

| Compose setting | Why nsjail needs it |
|-----------------|---------------------|
| `cap_add: SYS_ADMIN` | Create mount/PID namespaces and mount a fresh `/proc` inside the jail |
| `cap_add: SYS_PTRACE` | Used during nsjail's namespace and process setup |
| `security_opt: seccomp:unconfined` | Docker's default seccomp profile blocks `clone3` and friends that nsjail uses to spawn the jailed child |
| `security_opt: systempaths=unconfined` | Docker masks `/proc` paths with overmounts; the kernel refuses nsjail's fresh `/proc` mount while those exist |
| `security_opt: apparmor:unconfined` | Ubuntu's `docker-default` AppArmor profile blocks `pivot_root`/`mount` used during chroot setup |
| `volumes: /sys/fs/cgroup:rw` | Docker mounts cgroupfs read-only; nsjail must create child cgroups to enforce `memory.max`/`pids.max` and read `memory.peak` |
| `cgroup: host` | Gives nsjail the root cgroup-v2 hierarchy it requires |

These are scoped to what nsjail needs. The container is not `--privileged`, and the sandboxed code itself runs under a Kafel seccomp deny-list, dropped to an unprivileged UID, with no network. See [security.md](security.md).

---

## Running

```bash
make build        # build the image (nsjail compiled from source; ~5 min cold)
make run          # docker compose up goboxd  => service on :8080
```

The compose file also defines a `tools` profile (the `builder` stage with Go and linters) used by `make test`, `make integration`, `make lint`, and `make swagger`, so no local Go toolchain is required.

To override a setting, add it under `environment:` for the `goboxd` service or pass `-e` to `docker run`.

---

<!-- nav-footer -->
<sub>[← Documentation index](README.md) · [API](api.md) · [Architecture](architecture.md) · [Concurrency](concurrency.md) · [Security](security.md) · [Languages](languages.md) · [Configuration](configuration.md)</sub>
