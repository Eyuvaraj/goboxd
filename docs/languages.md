# Languages

goboxd is configured entirely through `configs/languages.yaml`. No Go code change is needed to add a language.

## Supported languages

### In-scope (required)

| ID | Name | Type |
|----|------|------|
| `py3` | Python 3 | Interpreted |
| `bash` | Bash | Interpreted |
| `js` | JavaScript (Node) | Interpreted |
| `c` | C | Compiled (gcc) |
| `cpp` | C++ | Compiled (g++) |
| `java` | Java | Compiled (javac/java) |
| `verilog` | Verilog | Compiled (iverilog/vvp) |

### Bonus languages

| ID | Name | Type |
|----|------|------|
| `ruby` | Ruby | Interpreted |
| `lua` | Lua 5.4 | Interpreted |
| `rust` | Rust | Compiled (rustc) |
| `kotlin` | Kotlin | Compiled (kotlinc → JVM) |
| `ocaml` | OCaml | Interpreted (ocaml) |
| `go` | Go | Compiled (go build) |

## YAML schema

```yaml
languages:
  - id: py3                        # unique identifier used in API requests
    name: Python 3                 # human-readable name
    source_filename: solution.py   # fixed filename written inside the sandbox

    # source_filename_strategy: from_request  # client supplies the filename (Java)
    # artifact_filename: solution             # compiled output name
    # artifact_filename_strategy: from_request

    build:                         # omit entirely for interpreted languages
      cmd: /usr/bin/gcc
      args: ["{{flags}}", "-o", "{{artifact}}", "{{source}}"]
      limits:
        wall_time_s: 10
        memory_kb: 524288
        max_processes: 100
      flag_allowlist:              # client flags not in this list → 400
        - "-O2"
        - "-std=*"                 # trailing * = prefix match

    run:
      cmd: /usr/bin/python3
      args: ["{{source}}"]         # templates: {{source}}, {{artifact}}, {{flags}}
      limits:
        wall_time_s: 9
        memory_kb: 102400
        max_processes: 100
      flag_allowlist: []           # empty = no client run flags allowed
```

### Template placeholders

| Placeholder | Expands to |
|-------------|-----------|
| `{{source}}` | The source filename (e.g. `solution.py`) |
| `{{artifact}}` | The compiled artifact filename (e.g. `solution`) |
| `{{flags}}` | All client-supplied flags as individual arguments |

Templates are expanded per-element in the args array — never through a shell. A `{{flags}}` element is replaced by zero or more individual flag arguments.

## Adding a language (example: PHP)

1. **Add install script** `scripts/lang_install/php.sh`:
   ```bash
   #!/usr/bin/env bash
   set -euo pipefail
   apt-get install -y --no-install-recommends php-cli
   php --version
   ```

2. **Add to `Dockerfile`** (in the `runtime` stage apt-get block):
   ```dockerfile
   php-cli \
   ```
   And a smoke test line:
   ```dockerfile
   && php --version \
   ```

3. **Add to `configs/languages.yaml`**:
   ```yaml
   - id: php
     name: PHP
     source_filename: solution.php
     run:
       cmd: /usr/bin/php
       args: ["{{source}}"]
       limits:
         wall_time_s: 10
         memory_kb: 131072
         max_processes: 50
   ```

4. **Rebuild the image.** `/readyz` and `/info` reflect the new language automatically.

**Time estimate: under 15 minutes. No Go code change.**

## The `env` field

Some languages require specific environment variables to work inside the sandbox (e.g. Go needs `GO111MODULE=off` to compile without a module file). Add them at the language level:

```yaml
- id: go
  name: Go
  source_filename: solution.go
  artifact_filename: solution
  env:
    - GO111MODULE=off
    - CGO_ENABLED=0
    - GOPATH=/
    - GOCACHE=/.cache/go-build
  build:
    cmd: /usr/bin/go
    args: ["build", "-o", "{{artifact}}", "{{source}}"]
    ...
```

These are injected as `--env KEY=VALUE` arguments to every nsjail invocation for that language. The standard sandbox env (`HOME`, `PATH`, `TMP`) is always present; `env` entries are appended after it.
