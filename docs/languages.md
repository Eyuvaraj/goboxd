# Languages

goboxd is configured entirely through `configs/languages.yaml`. No Go code change is needed to add a language.

## Supported languages

| ID | Name | Type |
|----|------|------|
| `py3` | Python 3 | Interpreted |
| `bash` | Bash | Interpreted |
| `js` | JavaScript (Node) | Interpreted |
| `c` | C | Compiled (gcc) |
| `cpp` | C++ | Compiled (g++) |
| `java` | Java | Compiled (javac/java) |
| `verilog` | Verilog | Compiled (iverilog/vvp) |

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

## Adding a language (example: Rust)

1. **Add install script** `scripts/lang_install/rust.sh`:
   ```bash
   #!/usr/bin/env bash
   set -euo pipefail
   curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain stable
   export PATH="$HOME/.cargo/bin:$PATH"
   rustc --version
   ```

2. **Add to `Dockerfile`** (in the `runtime` stage `RUN apt-get` block or separately):
   ```dockerfile
   RUN bash /scripts/lang_install/rust.sh
   ```

3. **Add to `configs/languages.yaml`**:
   ```yaml
   - id: rust
     name: Rust
     source_filename: solution.rs
     artifact_filename: solution
     build:
       cmd: /root/.cargo/bin/rustc
       args: ["{{flags}}", "-o", "{{artifact}}", "{{source}}"]
       limits:
         wall_time_s: 30
         memory_kb: 524288
         max_processes: 100
       flag_allowlist:
         - "-O"
         - "--edition=*"
     run:
       cmd: /solution
       args: []
       limits:
         wall_time_s: 5
         memory_kb: 262144
         max_processes: 64
   ```

4. **Rebuild the image.** `/readyz` and `/info` reflect the new language automatically.

**Time estimate: under 30 minutes. No Go code change.**
