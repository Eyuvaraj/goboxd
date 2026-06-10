# Adding a Language

One YAML block plus one install script, no Go code change. This is the runbook for doing it under the 30-minute demo-day bar.

For the field-by-field schema reference and the current catalog, see [languages.md](languages.md).

---

## How It Works

The registry (`internal/registry`) loads `configs/languages.yaml` at startup and treats every language as data. The runner expands three template tokens into a pure `[]string` argv and hands it to nsjail. Nothing in Go is per-language.

```
configs/languages.yaml      declare the language (build/run cmds, limits, flags)
scripts/lang_install/X.sh   install its toolchain into the runtime image
        |
        v  make build
registry loads + validates -> /readyz probes it -> POST /run executes it
```

---

## Step 1: Declare the Language in YAML

Add a block to `configs/languages.yaml`. Copy the closest existing shape.

**Interpreted** (no build phase, omit the `build:` block entirely):

```yaml
- id: php
  name: PHP
  source_filename: solution.php
  run:
    cmd: /usr/bin/php
    args: ["{{source}}"]
    limits: { wall_time_s: 10, memory_kb: 131072, max_processes: 50 }
```

**Compiled** (build phase produces an artifact the run phase executes):

```yaml
- id: zig
  name: Zig
  source_filename: solution.zig
  artifact_filename: solution
  build:
    cmd: /usr/bin/zig
    args: ["build-exe", "{{source}}", "-femit-bin={{artifact}}", "{{flags}}"]
    limits: { wall_time_s: 30, memory_kb: 524288, max_processes: 100 }
    flag_allowlist: ["-O*"]       # trailing * means prefix match
  run:
    cmd: /solution                # artifact lives at the workspace root
    args: []
    limits: { wall_time_s: 5, memory_kb: 262144, max_processes: 64 }
```

**Things to get right:**

- `cmd` must be an absolute path to the interpreter or compiler inside the image.
- For compiled languages, the run `cmd` is the artifact path inside the jail (e.g. `/solution`), not a host binary.
- `{{flags}}` expands to the validated client flags in-place; list every flag a client may pass in `flag_allowlist` (entries ending in `*` are prefix matches).
- If the toolchain's version flag is not `--version`, set `probe_args` (e.g. `["-v"]` for lua, `["version"]` for go).
- Need extra env vars (offline mode, cache dirs)? Add an `env:` list.
- Need a host path beyond the standard tree (`/bin /usr /lib /etc /dev /var`)? Add it to `bind_mounts`.

---

## Step 2: Install the Toolchain

Add `scripts/lang_install/<id>.sh`. The Dockerfile runtime stage loops over every script in that directory, so no Dockerfile edit is needed.

```bash
#!/usr/bin/env bash
set -euo pipefail
apt-get install -y --no-install-recommends php-cli
php --version          # smoke check: fail the image build if toolchain is broken
```

Keep the `set -euo pipefail` and the version smoke check. A broken toolchain then fails the image build instead of shipping a language that returns `internal_error` at runtime.

---

## Step 3: Build and Verify Readiness

```bash
make build
make run          # in one terminal
curl -s http://localhost:8080/readyz | python3 -m json.tool
# "php": { "ok": true, "version": "PHP 8.2.x ..." }
```

If `/readyz` shows `ok: false` for the new language, the probe could not run `cmd <probe_args>`. Check the binary path and `probe_args`.

---

## Step 4: Execute a Hello-World

```bash
curl -s http://localhost:8080/run -H 'Content-Type: application/json' -d '{
  "language": "php",
  "source": "<?php echo \"hello\\n\";",
  "tests": [{"stdin": "", "expected_stdout": "hello\n"}]
}' | python3 -m json.tool
# "status": "accepted"
```

---

## Step 5: Lock It In

- Add a hello-world fixture under `tests/integration/testdata/hello/` and a test in `tests/integration/run_test.go` so the language is covered by `make integration`.
- The registry validates the block at startup. If `make run` exits with a `language[i] "id": ...` error, fix the reported field.

---

## Timing

| Build state | Time to add a language |
|-------------|------------------------|
| Warm Docker cache (toolchain layer cached) | Well under 10 minutes |
| Cold build (recompiles nsjail and base layers) | Add ~5 minutes |

The demo-day target is under 30 minutes including writing the YAML and verifying a hello-world.

---

## Common Pitfalls

<details>
<summary><strong>bind_mounts EINVAL</strong></summary>

Do not add a path whose parent is already mounted. nsjail refuses to remount a child read-only over its parent. The defaults already cover `/usr`, `/bin`, `/lib`, `/etc`, `/dev`, `/var`.

</details>

<details>
<summary><strong>Compiler can't find headers or libs</strong></summary>

The toolchain's support files must be under one of the bind-mounted trees. Most Debian packages install under `/usr` and work automatically.

</details>

<details>
<summary><strong>Version probe returns junk or exits non-zero</strong></summary>

Set `probe_args`. The probe treats any output as success (some tools, e.g. `javac`, print the version to stderr and exit non-zero), but it needs to produce output.

</details>

<details>
<summary><strong>Toolchain installed outside /usr (e.g. a tarball under /opt)</strong></summary>

Bind-mount that directory via the language's `bind_mounts` field.

</details>
