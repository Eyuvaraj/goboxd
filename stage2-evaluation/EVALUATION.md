# Stage 2 - Language Extension Evaluation

> End-to-end verification of the three newly added languages against the live
> `goboxd` execution API. Each scenario posts a payload to `POST /run` and asserts
> that the returned top-level `status` matches the scenario's expected verdict.

| | |
|---|---|
| **Service** | `http://localhost:8080/run` |
| **Generated** | 2026-06-12 07:28 UTC |
| **Languages** | C# (`csharp`), Assembly x86 (`assembly` -> registry `asm`), Prolog (`prolog`) |
| **Scenarios** | 14 |
| **Passed** | 9 |
| **Failed** | 5 |
| **Pass rate** | 64% |

---

## Scoreboard

| Language | Registry id | Passed | Failed | Pass rate |
|---|---|---:|---:|---:|
| C# | `csharp` | 5/5 | 0 | 100% |
| Assembly x86 | `asm` | 1/5 | 4 | 20% |
| Prolog | `prolog` | 3/4 | 1 | 75% |

---

## C#

Registry id `csharp` &nbsp;|&nbsp; **5/5 passed**

| Scenario | Expected | Actual | Result |
|---|---|---|---|
| `accepted` | `accepted` | `accepted` | **`PASS`** |
| `build_failed` | `build_failed` | `build_failed` | **`PASS`** |
| `runtime_error` | `runtime_error` | `runtime_error` | **`PASS`** |
| `time_exceeded` | `time_exceeded` | `time_exceeded` | **`PASS`** |
| `wrong_output` | `wrong_output` | `wrong_output` | **`PASS`** |

### accepted

| Expected | Actual | HTTP | Result |
|---|---|---|---|
| `accepted` | `accepted` | 200 | **`PASS`** |

**Build:** `ok`

**Output**

- `test[0]` &middot; status `accepted` &middot; 45 ms &middot; 11560 kb
  - stdout: `'42\n'`

<details>
<summary>Payload (request)</summary>

```json
{
  "language": "csharp",
  "source": "using System;\nclass Program {\n    static void Main() {\n        Console.WriteLine(int.Parse(Console.In.ReadToEnd().Trim()) * 2);\n    }\n}",
  "source_filename": "Main.cs",
  "artifact_filename": "Main.exe",
  "tests": [
    {
      "stdin": "21\n",
      "expected_stdout": "42\n"
    }
  ]
}
```

</details>

<details>
<summary>Response</summary>

```json
{
  "status": "accepted",
  "build": {
    "status": "ok",
    "stdout": "",
    "stderr": "",
    "duration_ms": 203
  },
  "tests": [
    {
      "status": "accepted",
      "stdout": "42\n",
      "stderr": "",
      "duration_ms": 45,
      "memory_peak_kb": 11560
    }
  ]
}
```

</details>

### build_failed

| Expected | Actual | HTTP | Result |
|---|---|---|---|
| `build_failed` | `build_failed` | 200 | **`PASS`** |

**Build:** `failed`

**Output**

- `test[0]` &middot; status `not_executed` &middot; 0 ms &middot; 0 kb

<details>
<summary>Payload (request)</summary>

```json
{
  "language": "csharp",
  "source": "using System;\nclass Program {\n    static void Main() { Console.WriteLine(\"x\")\n}",
  "source_filename": "Main.cs",
  "artifact_filename": "Main.exe",
  "tests": [
    {
      "stdin": "21\n",
      "expected_stdout": "42\n"
    }
  ]
}
```

</details>

<details>
<summary>Response</summary>

```json
{
  "status": "build_failed",
  "build": {
    "status": "failed",
    "stdout": "Compilation failed: 2 error(s), 0 warnings\n",
    "stderr": "solution.cs(4,1): error CS1002: ; expected\nsolution.cs(3,246): error CS1525: Unexpected symbol `end-of-file'\n",
    "duration_ms": 40
  },
  "tests": [
    {
      "status": "not_executed",
      "stdout": "",
      "stderr": "",
      "duration_ms": 0,
      "memory_peak_kb": 0
    }
  ]
}
```

</details>

### runtime_error

| Expected | Actual | HTTP | Result |
|---|---|---|---|
| `runtime_error` | `runtime_error` | 200 | **`PASS`** |

**Build:** `ok`

**Output**

- `test[0]` &middot; status `runtime_error` &middot; 42 ms &middot; 2876 kb
  - stderr: `'\nUnhandled Exception:\nSystem.Exception: boom\n  at Program.Main () [0x00000] in <c04131fb66d9497bbb8b811682c72966>:0 \n[ERROR] FATAL UNHANDLED EXCEPTION: System.Exception: boom\n  at Program.Main () [0x00000] in <c04131fb66d9497bbb8b811682c72966>:0 \n'`

<details>
<summary>Payload (request)</summary>

```json
{
  "language": "csharp",
  "source": "using System;\nclass Program {\n    static void Main() { throw new Exception(\"boom\"); }\n}",
  "source_filename": "Main.cs",
  "artifact_filename": "Main.exe",
  "tests": [
    {
      "stdin": "21\n",
      "expected_stdout": "42\n"
    }
  ]
}
```

</details>

<details>
<summary>Response</summary>

```json
{
  "status": "runtime_error",
  "build": {
    "status": "ok",
    "stdout": "",
    "stderr": "",
    "duration_ms": 179
  },
  "tests": [
    {
      "status": "runtime_error",
      "stdout": "",
      "stderr": "\nUnhandled Exception:\nSystem.Exception: boom\n  at Program.Main () [0x00000] in <c04131fb66d9497bbb8b811682c72966>:0 \n[ERROR] FATAL UNHANDLED EXCEPTION: System.Exception: boom\n  at Program.Main () [0x00000] in <c04131fb66d9497bbb8b811682c72966>:0 \n",
      "duration_ms": 42,
      "memory_peak_kb": 2876
    }
  ]
}
```

</details>

### time_exceeded

| Expected | Actual | HTTP | Result |
|---|---|---|---|
| `time_exceeded` | `time_exceeded` | 200 | **`PASS`** |

**Build:** `ok`

**Output**

- `test[0]` &middot; status `time_exceeded` &middot; 1008 ms &middot; 1636 kb

<details>
<summary>Payload (request)</summary>

```json
{
  "language": "csharp",
  "source": "class Program {\n    static void Main() { while (true) {} }\n}",
  "source_filename": "Main.cs",
  "artifact_filename": "Main.exe",
  "tests": [
    {
      "stdin": "21\n",
      "expected_stdout": "42\n"
    }
  ],
  "run": {
    "limits": {
      "wall_time_s": 1
    }
  }
}
```

</details>

<details>
<summary>Response</summary>

```json
{
  "status": "time_exceeded",
  "build": {
    "status": "ok",
    "stdout": "",
    "stderr": "",
    "duration_ms": 133
  },
  "tests": [
    {
      "status": "time_exceeded",
      "stdout": "",
      "stderr": "",
      "duration_ms": 1008,
      "memory_peak_kb": 1636
    }
  ]
}
```

</details>

### wrong_output

| Expected | Actual | HTTP | Result |
|---|---|---|---|
| `wrong_output` | `wrong_output` | 200 | **`PASS`** |

**Build:** `ok`

**Output**

- `test[0]` &middot; status `wrong_output` &middot; 38 ms &middot; 3368 kb
  - stdout: `'42\n'`

<details>
<summary>Payload (request)</summary>

```json
{
  "language": "csharp",
  "source": "using System;\nclass Program {\n    static void Main() {\n        Console.WriteLine(int.Parse(Console.In.ReadToEnd().Trim()) * 2);\n    }\n}",
  "source_filename": "Main.cs",
  "artifact_filename": "Main.exe",
  "tests": [
    {
      "stdin": "21\n",
      "expected_stdout": "43\n"
    }
  ]
}
```

</details>

<details>
<summary>Response</summary>

```json
{
  "status": "wrong_output",
  "build": {
    "status": "ok",
    "stdout": "",
    "stderr": "",
    "duration_ms": 153
  },
  "tests": [
    {
      "status": "wrong_output",
      "stdout": "42\n",
      "stderr": "",
      "duration_ms": 38,
      "memory_peak_kb": 3368
    }
  ]
}
```

</details>

---

## Assembly x86

Registry id `asm` &nbsp;|&nbsp; **1/5 passed**

| Scenario | Expected | Actual | Result |
|---|---|---|---|
| `accepted` | `accepted` | `build_failed` | **`FAIL`** |
| `build_failed` | `build_failed` | `build_failed` | **`PASS`** |
| `runtime_error` | `runtime_error` | `build_failed` | **`FAIL`** |
| `time_exceeded` | `time_exceeded` | `build_failed` | **`FAIL`** |
| `wrong_output` | `wrong_output` | `build_failed` | **`FAIL`** |

### accepted

| Expected | Actual | HTTP | Result |
|---|---|---|---|
| `accepted` | `build_failed` | 200 | **`FAIL`** |

**Build:** `failed`

**Output**

- `test[0]` &middot; status `not_executed` &middot; 0 ms &middot; 0 kb

<details>
<summary>Payload (request)</summary>

```json
{
  "language": "asm",
  "source": ".global _start\n.section .data\nmsg: .ascii \"42\\n\"\n.section .text\n_start:\n    mov $1, %rax\n    mov $1, %rdi\n    lea msg(%rip), %rsi\n    mov $3, %rdx\n    syscall\n    mov $60, %rax\n    xor %rdi, %rdi\n    syscall",
  "tests": [
    {
      "stdin": "",
      "expected_stdout": "42\n"
    }
  ]
}
```

</details>

<details>
<summary>Response</summary>

```json
{
  "status": "build_failed",
  "build": {
    "status": "failed",
    "stdout": "",
    "stderr": "solution.asm:1: error: parser: instruction expected\nsolution.asm:2: error: parser: instruction expected\nsolution.asm:3: error: parser: instruction expected\nsolution.asm:4: error: parser: instruction expected\nsolution.asm:6: error: expression syntax error\nsolution.asm:7: error: expression syntax error\nsolution.asm:8: error: comma, colon, decorator or end of line expected after operand\nsolution.asm:8: error: expression syntax error\nsolution.asm:9: error: expression syntax error\nsolution.asm:11: error: expression syntax error\nsolution.asm:12: error: expression syntax error\n",
    "duration_ms": 37
  },
  "tests": [
    {
      "status": "not_executed",
      "stdout": "",
      "stderr": "",
      "duration_ms": 0,
      "memory_peak_kb": 0
    }
  ]
}
```

</details>

### build_failed

| Expected | Actual | HTTP | Result |
|---|---|---|---|
| `build_failed` | `build_failed` | 200 | **`PASS`** |

**Build:** `failed`

**Output**

- `test[0]` &middot; status `not_executed` &middot; 0 ms &middot; 0 kb

<details>
<summary>Payload (request)</summary>

```json
{
  "language": "asm",
  "source": ".global _start\n.section .text\n_start:\n    not_a_real_instruction %rax",
  "tests": [
    {
      "stdin": "",
      "expected_stdout": "42\n"
    }
  ]
}
```

</details>

<details>
<summary>Response</summary>

```json
{
  "status": "build_failed",
  "build": {
    "status": "failed",
    "stdout": "",
    "stderr": "solution.asm:1: error: parser: instruction expected\nsolution.asm:2: error: parser: instruction expected\nsolution.asm:4: error: parser: instruction expected\n",
    "duration_ms": 16
  },
  "tests": [
    {
      "status": "not_executed",
      "stdout": "",
      "stderr": "",
      "duration_ms": 0,
      "memory_peak_kb": 0
    }
  ]
}
```

</details>

### runtime_error

| Expected | Actual | HTTP | Result |
|---|---|---|---|
| `runtime_error` | `build_failed` | 200 | **`FAIL`** |

**Build:** `failed`

**Output**

- `test[0]` &middot; status `not_executed` &middot; 0 ms &middot; 0 kb

<details>
<summary>Payload (request)</summary>

```json
{
  "language": "asm",
  "source": ".global _start\n.section .text\n_start:\n    mov $60, %rax\n    mov $3, %rdi\n    syscall",
  "tests": [
    {
      "stdin": "",
      "expected_stdout": "42\n"
    }
  ]
}
```

</details>

<details>
<summary>Response</summary>

```json
{
  "status": "build_failed",
  "build": {
    "status": "failed",
    "stdout": "",
    "stderr": "solution.asm:1: error: parser: instruction expected\nsolution.asm:2: error: parser: instruction expected\nsolution.asm:4: error: expression syntax error\nsolution.asm:5: error: expression syntax error\n",
    "duration_ms": 14
  },
  "tests": [
    {
      "status": "not_executed",
      "stdout": "",
      "stderr": "",
      "duration_ms": 0,
      "memory_peak_kb": 0
    }
  ]
}
```

</details>

### time_exceeded

| Expected | Actual | HTTP | Result |
|---|---|---|---|
| `time_exceeded` | `build_failed` | 200 | **`FAIL`** |

**Build:** `failed`

**Output**

- `test[0]` &middot; status `not_executed` &middot; 0 ms &middot; 0 kb

<details>
<summary>Payload (request)</summary>

```json
{
  "language": "asm",
  "source": ".global _start\n.section .text\n_start:\nspin:\n    jmp spin",
  "tests": [
    {
      "stdin": "",
      "expected_stdout": "42\n"
    }
  ],
  "run": {
    "limits": {
      "wall_time_s": 1
    }
  }
}
```

</details>

<details>
<summary>Response</summary>

```json
{
  "status": "build_failed",
  "build": {
    "status": "failed",
    "stdout": "",
    "stderr": "solution.asm:1: error: parser: instruction expected\nsolution.asm:2: error: parser: instruction expected\n",
    "duration_ms": 13
  },
  "tests": [
    {
      "status": "not_executed",
      "stdout": "",
      "stderr": "",
      "duration_ms": 0,
      "memory_peak_kb": 0
    }
  ]
}
```

</details>

### wrong_output

| Expected | Actual | HTTP | Result |
|---|---|---|---|
| `wrong_output` | `build_failed` | 200 | **`FAIL`** |

**Build:** `failed`

**Output**

- `test[0]` &middot; status `not_executed` &middot; 0 ms &middot; 0 kb

<details>
<summary>Payload (request)</summary>

```json
{
  "language": "asm",
  "source": ".global _start\n.section .data\nmsg: .ascii \"42\\n\"\n.section .text\n_start:\n    mov $1, %rax\n    mov $1, %rdi\n    lea msg(%rip), %rsi\n    mov $3, %rdx\n    syscall\n    mov $60, %rax\n    xor %rdi, %rdi\n    syscall",
  "tests": [
    {
      "stdin": "",
      "expected_stdout": "43\n"
    }
  ]
}
```

</details>

<details>
<summary>Response</summary>

```json
{
  "status": "build_failed",
  "build": {
    "status": "failed",
    "stdout": "",
    "stderr": "solution.asm:1: error: parser: instruction expected\nsolution.asm:2: error: parser: instruction expected\nsolution.asm:3: error: parser: instruction expected\nsolution.asm:4: error: parser: instruction expected\nsolution.asm:6: error: expression syntax error\nsolution.asm:7: error: expression syntax error\nsolution.asm:8: error: comma, colon, decorator or end of line expected after operand\nsolution.asm:8: error: expression syntax error\nsolution.asm:9: error: expression syntax error\nsolution.asm:11: error: expression syntax error\nsolution.asm:12: error: expression syntax error\n",
    "duration_ms": 10
  },
  "tests": [
    {
      "status": "not_executed",
      "stdout": "",
      "stderr": "",
      "duration_ms": 0,
      "memory_peak_kb": 0
    }
  ]
}
```

</details>

---

## Prolog

Registry id `prolog` &nbsp;|&nbsp; **3/4 passed**

| Scenario | Expected | Actual | Result |
|---|---|---|---|
| `accepted` | `accepted` | `accepted` | **`PASS`** |
| `runtime_error` | `runtime_error` | `wrong_output` | **`FAIL`** |
| `time_exceeded` | `time_exceeded` | `time_exceeded` | **`PASS`** |
| `wrong_output` | `wrong_output` | `wrong_output` | **`PASS`** |

### accepted

| Expected | Actual | HTTP | Result |
|---|---|---|---|
| `accepted` | `accepted` | 200 | **`PASS`** |

**Build:** `ok`

**Output**

- `test[0]` &middot; status `accepted` &middot; 41 ms &middot; 8440 kb
  - stdout: `'42\n'`

<details>
<summary>Payload (request)</summary>

```json
{
  "language": "prolog",
  "source": ":- initialization(main).\nmain :-\n    read_line_to_string(user_input, S),\n    number_string(N, S),\n    M is N * 2,\n    format(\"~w~n\", [M]),\n    halt.",
  "tests": [
    {
      "stdin": "21\n",
      "expected_stdout": "42\n"
    }
  ]
}
```

</details>

<details>
<summary>Response</summary>

```json
{
  "status": "accepted",
  "build": {
    "status": "ok",
    "stdout": "",
    "stderr": "",
    "duration_ms": 0
  },
  "tests": [
    {
      "status": "accepted",
      "stdout": "42\n",
      "stderr": "",
      "duration_ms": 41,
      "memory_peak_kb": 8440
    }
  ]
}
```

</details>

### runtime_error

| Expected | Actual | HTTP | Result |
|---|---|---|---|
| `runtime_error` | `wrong_output` | 200 | **`FAIL`** |

**Build:** `ok`

**Output**

- `test[0]` &middot; status `wrong_output` &middot; 18 ms &middot; 7476 kb
  - stderr: `'ERROR: /solution.pl:1: Initialization goal raised exception:\nERROR: Unknown message: boom_error\n'`

<details>
<summary>Payload (request)</summary>

```json
{
  "language": "prolog",
  "source": ":- initialization(main).\nmain :- throw(boom_error).",
  "tests": [
    {
      "stdin": "21\n",
      "expected_stdout": "42\n"
    }
  ]
}
```

</details>

<details>
<summary>Response</summary>

```json
{
  "status": "wrong_output",
  "build": {
    "status": "ok",
    "stdout": "",
    "stderr": "",
    "duration_ms": 0
  },
  "tests": [
    {
      "status": "wrong_output",
      "stdout": "",
      "stderr": "ERROR: /solution.pl:1: Initialization goal raised exception:\nERROR: Unknown message: boom_error\n",
      "duration_ms": 18,
      "memory_peak_kb": 7476
    }
  ]
}
```

</details>

### time_exceeded

| Expected | Actual | HTTP | Result |
|---|---|---|---|
| `time_exceeded` | `time_exceeded` | 200 | **`PASS`** |

**Build:** `ok`

**Output**

- `test[0]` &middot; status `time_exceeded` &middot; 1007 ms &middot; 7292 kb

<details>
<summary>Payload (request)</summary>

```json
{
  "language": "prolog",
  "source": ":- initialization(main).\nmain :- repeat, fail.",
  "tests": [
    {
      "stdin": "21\n",
      "expected_stdout": "42\n"
    }
  ],
  "run": {
    "limits": {
      "wall_time_s": 1
    }
  }
}
```

</details>

<details>
<summary>Response</summary>

```json
{
  "status": "time_exceeded",
  "build": {
    "status": "ok",
    "stdout": "",
    "stderr": "",
    "duration_ms": 0
  },
  "tests": [
    {
      "status": "time_exceeded",
      "stdout": "",
      "stderr": "",
      "duration_ms": 1007,
      "memory_peak_kb": 7292
    }
  ]
}
```

</details>

### wrong_output

| Expected | Actual | HTTP | Result |
|---|---|---|---|
| `wrong_output` | `wrong_output` | 200 | **`PASS`** |

**Build:** `ok`

**Output**

- `test[0]` &middot; status `wrong_output` &middot; 87 ms &middot; 8612 kb
  - stdout: `'42\n'`

<details>
<summary>Payload (request)</summary>

```json
{
  "language": "prolog",
  "source": ":- initialization(main).\nmain :-\n    read_line_to_string(user_input, S),\n    number_string(N, S),\n    M is N * 2,\n    format(\"~w~n\", [M]),\n    halt.",
  "tests": [
    {
      "stdin": "21\n",
      "expected_stdout": "43\n"
    }
  ]
}
```

</details>

<details>
<summary>Response</summary>

```json
{
  "status": "wrong_output",
  "build": {
    "status": "ok",
    "stdout": "",
    "stderr": "",
    "duration_ms": 0
  },
  "tests": [
    {
      "status": "wrong_output",
      "stdout": "42\n",
      "stderr": "",
      "duration_ms": 87,
      "memory_peak_kb": 8612
    }
  ]
}
```

</details>

---

## Notes on failures

- **Assembly** &mdash; the challenge fixtures are GAS/AT&T syntax and 32-bit, while
  the `asm` toolchain uses **NASM** (Intel syntax). NASM cannot parse them, so they
  return `build_failed`. `build_failed.json` matches only incidentally (an invalid
  program fails to build under either assembler). NASM-syntax fixtures pass.
- **Prolog `runtime_error`** &mdash; the fixture raises an uncaught exception, but
  under `swipl -g halt` that still exits 0, so the verdict is `wrong_output`.
  A deterministic `runtime_error` needs an explicit non-zero `halt(N)`.
- **Assembly 32-bit** &mdash; on arm64 hosts `qemu-i386` cannot reserve the guest
  address space, so only the x86-64 path is currently supported.
