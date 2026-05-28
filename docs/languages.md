# Language Configuration

`goboxd` uses a fully data-driven configuration model for languages. **No Go code changes are required** to add support for a new language. The entire language catalog is managed through `configs/languages.yaml`.

---

## Supported Languages

The following languages are currently supported out of the box:

### Core Languages

| ID | Language | Type | Toolchain |
|----|----------|------|-----------|
| `py3` | Python 3 | Interpreted | `/usr/bin/python3` |
| `bash` | Bash | Interpreted | `/bin/bash` |
| `js` | JavaScript (Node.js) | Interpreted | `/usr/bin/node` |
| `c` | C | Compiled | `gcc` |
| `cpp` | C++ | Compiled | `g++` |
| `java` | Java | Compiled | `javac` / `java` |
| `verilog` | Verilog | Compiled | `iverilog` / `vvp` |

### Additional Supported Languages

| ID | Language | Type | Toolchain |
|----|----------|------|-----------|
| `ruby` | Ruby | Interpreted | `ruby` |
| `lua` | Lua 5.4 | Interpreted | `lua5.4` |
| `rust` | Rust | Compiled | `rustc` |
| `kotlin` | Kotlin | Compiled | `kotlinc` / `java` |
| `ocaml` | OCaml | Interpreted | `ocaml` |
| `go` | Go | Compiled | `go build` |

---

## YAML Configuration Schema

The `languages.yaml` file defines how the sandbox interacts with each language's toolchain.

```yaml
languages:
  - id: py3                        # Unique identifier used in API requests
    name: Python 3                 # Human-readable display name
    source_filename: solution.py   # Static filename used inside the sandbox
    
    # Advanced Filename Strategies (e.g., for Java):
    # source_filename_strategy: from_request
    # artifact_filename: solution
    # artifact_filename_strategy: from_request

    # Build Phase (Omit for interpreted languages)
    build:                         
      cmd: /usr/bin/gcc
      args: ["{{flags}}", "-o", "{{artifact}}", "{{source}}"]
      limits:
        wall_time_s: 10
        memory_kb: 524288
        max_processes: 100
      flag_allowlist:              # Strict allowlist. Unlisted client flags result in 400 Bad Request
        - "-O2"
        - "-std=*"                 # The '*' wildcard allows prefix matching

    # Run Phase
    run:
      cmd: /usr/bin/python3
      args: ["{{source}}"]         # Available templates: {{source}}, {{artifact}}, {{flags}}
      limits:
        wall_time_s: 9
        memory_kb: 102400
        max_processes: 100
      flag_allowlist: []           # An empty list explicitly forbids any client-provided run flags
```

### Template Expansion

To avoid shell injection vulnerabilities, `goboxd` expands templates directly into argument array elements. **A shell is never used.**

| Placeholder | Resolves To | Example |
|-------------|-------------|---------|
| `{{source}}` | The source code filename | `solution.py` |
| `{{artifact}}` | The compiled output filename | `solution` |
| `{{flags}}` | Array of validated client flags | `["-O2", "-Wall"]` |

> **Note:** If `{{flags}}` is used in the `args` array, it expands in-place into zero or more separate elements.

---

## Step-by-Step: Adding a New Language

Adding a new language (e.g., PHP) is straightforward and typically takes under 15 minutes.

1. **Create an Installation Script:**
   Add `scripts/lang_install/php.sh` to install the required packages.
   ```bash
   #!/usr/bin/env bash
   set -euo pipefail
   apt-get install -y --no-install-recommends php-cli
   php --version
   ```

2. **Update the Dockerfile:**
   Add the toolchain to the `runtime` stage's `apt-get` installation block.
   ```dockerfile
   php-cli \
   ```
   Add a smoke test to verify the installation:
   ```dockerfile
   && php --version \
   ```

3. **Register the Language in `languages.yaml`:**
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

4. **Rebuild and Deploy:**
   Rebuild the Docker image. The new language will automatically appear in `/readyz` and `/info` endpoints.

---

## Environment Variables (`env`)

Certain toolchains require specific environment variables to function properly in an isolated sandbox (e.g., Go requires `GO111MODULE=off` when compiling outside of a standard module directory). 

You can define these at the language level:

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
```

These variables are injected into the `nsjail` invocation via `--env KEY=VALUE` arguments. They supplement the baseline sandbox environment variables (`HOME`, `PATH`, `TMP`), which are always provided.
