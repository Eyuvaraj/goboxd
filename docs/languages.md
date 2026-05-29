# Language Configuration

`goboxd` is fully data-driven. **No Go code changes are needed** to add a language. The entire catalog lives in `configs/languages.yaml`.

---

## Supported Languages

### In-scope (required by spec)

| ID | Language | Type | Toolchain |
|----|----------|------|-----------|
| `py3` | Python 3 | Interpreted | `/usr/bin/python3` |
| `bash` | Bash | Interpreted | `/bin/bash` |
| `js` | JavaScript (Node.js) | Interpreted | `/usr/bin/node` |
| `c` | C | Compiled | `gcc` |
| `cpp` | C++ | Compiled | `g++` |
| `java` | Java | Compiled | `javac` / `java` |
| `verilog` | Verilog | Compiled | `iverilog` / `vvp` |

### Bonus

| ID | Language | Type | Toolchain |
|----|----------|------|-----------|
| `ruby` | Ruby | Interpreted | `/usr/bin/ruby` |
| `lua` | Lua 5.4 | Interpreted | `/usr/bin/lua5.4` |
| `rust` | Rust | Compiled | `/usr/bin/rustc` |
| `kotlin` | Kotlin | Compiled | `/usr/bin/kotlinc` + `java` |
| `ocaml` | OCaml | Interpreted | `/usr/bin/ocaml` |
| `go` | Go | Compiled | `/usr/bin/go build` |

---

## YAML Schema

```yaml
languages:
  - id: cpp                          # unique identifier used in API requests
    name: C++                        # human-readable display name
    source_filename: solution.cpp    # fixed filename written into the workspace

    # Java uses from_request so the filename matches the public class name:
    # source_filename_strategy: from_request
    # artifact_filename_strategy: from_request

    build:                           # omit entirely for interpreted languages
      cmd: /usr/bin/g++
      args: ["{{flags}}", "-o", "{{artifact}}", "{{source}}"]
      limits:
        wall_time_s: 10
        memory_kb: 524288
        max_processes: 100
      flag_allowlist:                # unlisted flags return 400 invalid_flag
        - "-O2"
        - "-std=*"                   # trailing * = prefix match

    run:
      cmd: /solution                 # compiled artifact lives at the workspace root
      args: []
      limits:
        wall_time_s: 5
        memory_kb: 262144
        max_processes: 64

    env:                             # optional extra env vars injected via --env KEY=VALUE
      - GO111MODULE=off              # example from the Go language definition
```

### Template variables

| Placeholder | Expands to |
|-------------|-----------|
| `{{source}}` | Source filename inside the workspace |
| `{{artifact}}` | Compiled output filename |
| `{{flags}}` | Zero or more validated client-supplied flags (in-place expansion) |

Expansion is done element-by-element in `sandbox.ExpandArgs` — never through a shell.

---

## Adding a Language

With a warm Docker cache (layers already built), adding a language takes under 10 minutes. Cold build adds ~5 minutes for the base image layer.

### Steps

1. **Add a YAML block** to `configs/languages.yaml` — copy the shape closest to the new language (interpreted or compiled).

2. **Add the toolchain** to the Dockerfile runtime stage:
   ```dockerfile
   php-cli \
   ```

3. **Rebuild:**
   ```
   make build
   make run
   ```

4. **Verify:**
   ```
   curl http://localhost:8080/readyz | jq .languages.php
   # → {"ok": true, "version": "PHP 8.2.x"}
   ```

5. **Test:**
   ```
   curl -s http://localhost:8080/run -d '{
     "language": "php",
     "source": "<?php echo \"hello\\n\";",
     "tests": [{"stdin": "", "expected_stdout": "hello\n"}]
   }' | jq .status
   # → "accepted"
   ```

No Go code change at any step.

### Probe command

By default `/readyz` runs `<cmd> --version` against each language's run command. If a toolchain uses a different flag (e.g. `version` without dashes), set `probe_cmd` in the YAML — though none of the current 13 languages need this.
