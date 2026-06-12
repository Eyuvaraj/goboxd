# Stage 2 - Language Extension Evaluation

> End-to-end verification of the three newly added languages against the live
> `goboxd` execution API. Each scenario posts a payload to `POST /run` and asserts
> that the returned top-level `status` matches the scenario's expected verdict.

| | |
|---|---|
| **Service** | `http://localhost:8080/run` |
| **Branch** | `team/Alpha-stage3` (f6b70a3) |
| **Generated** | 2026-06-12 11:09 UTC |
| **Languages** | C# (`csharp`), Assembly x86 (`assembly`), Prolog (`prolog`) |
| **Scenarios** | 13 |
| **Passed** | 9 |
| **Failed** | 4 |
| **Pass rate** | 69% |

---

## Scoreboard

| Language | Registry id | Passed | Failed | Pass rate |
|---|---|---:|---:|---:|
| C# | `csharp` | 5/5 | 0 | 100% |
| Assembly x86 | `assembly` | 1/5 | 4 | 20% |
| Prolog | `prolog` | 3/3 | 0 | 100% |

---

## C#

Registry id `csharp` | 5/5 scenarios passed

| Scenario | Result | Build | Status | Duration | Memory |
|---|---|---|---|---:|---:|
| `accepted` | **PASS** | `ok` | `accepted` | 35 ms | 9276 kb |
| `build_failed` | **PASS** | `failed` | `build_failed` | 0 ms | 0 kb |
| `runtime_error` | **PASS** | `ok` | `runtime_error` | 33 ms | 8764 kb |
| `time_exceeded` | **PASS** | `ok` | `time_exceeded` | 1010 ms | 5952 kb |
| `wrong_output` | **PASS** | `ok` | `wrong_output` | 40 ms | 11324 kb |

### `accepted`

stdout: `'42\n'`

<details>
<summary>Payload and response</summary>

**Request**

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

**Response**

```json
{
  "status": "accepted",
  "build": {
    "status": "ok",
    "stdout": "",
    "stderr": "",
    "duration_ms": 188
  },
  "tests": [
    {
      "status": "accepted",
      "stdout": "42\n",
      "stderr": "",
      "duration_ms": 35,
      "memory_peak_kb": 9276
    }
  ]
}
```

</details>

### `build_failed`

build stderr: `solution.cs(4,1): error CS1002: ; expected\nsolution.cs(3,246): error CS1525: Unexpected symbol `end-of-file'`

<details>
<summary>Payload and response</summary>

**Request**

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

**Response**

```json
{
  "status": "build_failed",
  "build": {
    "status": "failed",
    "stdout": "Compilation failed: 2 error(s), 0 warnings\n",
    "stderr": "solution.cs(4,1): error CS1002: ; expected\nsolution.cs(3,246): error CS1525: Unexpected symbol `end-of-file'\n",
    "duration_ms": 34
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

### `runtime_error`

stderr: `Unhandled Exception:\nSystem.Exception: boom\n  at Program.Main () [0x00000] in <c04131fb66d9497bbb8b811682c72966>:0   _(+2 more lines)_`

<details>
<summary>Payload and response</summary>

**Request**

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

**Response**

```json
{
  "status": "runtime_error",
  "build": {
    "status": "ok",
    "stdout": "",
    "stderr": "",
    "duration_ms": 167
  },
  "tests": [
    {
      "status": "runtime_error",
      "stdout": "",
      "stderr": "\nUnhandled Exception:\nSystem.Exception: boom\n  at Program.Main () [0x00000] in <c04131fb66d9497bbb8b811682c72966>:0 \n[ERROR] FATAL UNHANDLED EXCEPTION: System.Exception: boom\n  at Program.Main () [0x00000] in <c04131fb66d9497bbb8b811682c72966>:0 \n",
      "duration_ms": 33,
      "memory_peak_kb": 8764
    }
  ]
}
```

</details>

### `time_exceeded`

<details>
<summary>Payload and response</summary>

**Request**

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

**Response**

```json
{
  "status": "time_exceeded",
  "build": {
    "status": "ok",
    "stdout": "",
    "stderr": "",
    "duration_ms": 117
  },
  "tests": [
    {
      "status": "time_exceeded",
      "stdout": "",
      "stderr": "",
      "duration_ms": 1010,
      "memory_peak_kb": 5952
    }
  ]
}
```

</details>

### `wrong_output`

stdout: `'42\n'`

<details>
<summary>Payload and response</summary>

**Request**

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

**Response**

```json
{
  "status": "wrong_output",
  "build": {
    "status": "ok",
    "stdout": "",
    "stderr": "",
    "duration_ms": 158
  },
  "tests": [
    {
      "status": "wrong_output",
      "stdout": "42\n",
      "stderr": "",
      "duration_ms": 40,
      "memory_peak_kb": 11324
    }
  ]
}
```

</details>

---

## Assembly x86

Registry id `assembly` | 1/5 scenarios passed

| Scenario | Result | Build | Status | Duration | Memory |
|---|---|---|---|---:|---:|
| `accepted` | **FAIL** | `failed` | `build_failed` | 0 ms | 0 kb |
| `build_failed` | **PASS** | `failed` | `build_failed` | 0 ms | 0 kb |
| `runtime_error` | **FAIL** | `failed` | `build_failed` | 0 ms | 0 kb |
| `time_exceeded` | **FAIL** | `failed` | `build_failed` | 0 ms | 0 kb |
| `wrong_output` | **FAIL** | `failed` | `build_failed` | 0 ms | 0 kb |

### `accepted`

build stderr: `solution.asm:1: error: parser: instruction expected\nsolution.asm:2: error: parser: instruction expected\nsolution.asm:3: error: parser: instruction expected  _(+8 more lines)_`

<details>
<summary>Payload and response</summary>

**Request**

```json
{
  "language": "assembly",
  "source": ".global _start\n.section .data\nmsg: .ascii \"42\\n\"\n.section .text\n_start:\n    mov $1, %rax\n    mov $1, %rdi\n    lea msg(%rip), %rsi\n    mov $3, %rdx\n    syscall\n    mov $60, %rax\n    xor %rdi, %rdi\n    syscall",
  "tests": [
    {
      "stdin": "",
      "expected_stdout": "42\n"
    }
  ]
}
```

**Response**

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

### `build_failed`

build stderr: `solution.asm:1: error: parser: instruction expected\nsolution.asm:2: error: parser: instruction expected\nsolution.asm:4: error: parser: instruction expected`

<details>
<summary>Payload and response</summary>

**Request**

```json
{
  "language": "assembly",
  "source": ".global _start\n.section .text\n_start:\n    not_a_real_instruction %rax",
  "tests": [
    {
      "stdin": "",
      "expected_stdout": "42\n"
    }
  ]
}
```

**Response**

```json
{
  "status": "build_failed",
  "build": {
    "status": "failed",
    "stdout": "",
    "stderr": "solution.asm:1: error: parser: instruction expected\nsolution.asm:2: error: parser: instruction expected\nsolution.asm:4: error: parser: instruction expected\n",
    "duration_ms": 11
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

### `runtime_error`

build stderr: `solution.asm:1: error: parser: instruction expected\nsolution.asm:2: error: parser: instruction expected\nsolution.asm:4: error: expression syntax error  _(+1 more lines)_`

<details>
<summary>Payload and response</summary>

**Request**

```json
{
  "language": "assembly",
  "source": ".global _start\n.section .text\n_start:\n    mov $60, %rax\n    mov $3, %rdi\n    syscall",
  "tests": [
    {
      "stdin": "",
      "expected_stdout": "42\n"
    }
  ]
}
```

**Response**

```json
{
  "status": "build_failed",
  "build": {
    "status": "failed",
    "stdout": "",
    "stderr": "solution.asm:1: error: parser: instruction expected\nsolution.asm:2: error: parser: instruction expected\nsolution.asm:4: error: expression syntax error\nsolution.asm:5: error: expression syntax error\n",
    "duration_ms": 9
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

### `time_exceeded`

build stderr: `solution.asm:1: error: parser: instruction expected\nsolution.asm:2: error: parser: instruction expected`

<details>
<summary>Payload and response</summary>

**Request**

```json
{
  "language": "assembly",
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

**Response**

```json
{
  "status": "build_failed",
  "build": {
    "status": "failed",
    "stdout": "",
    "stderr": "solution.asm:1: error: parser: instruction expected\nsolution.asm:2: error: parser: instruction expected\n",
    "duration_ms": 9
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

### `wrong_output`

build stderr: `solution.asm:1: error: parser: instruction expected\nsolution.asm:2: error: parser: instruction expected\nsolution.asm:3: error: parser: instruction expected  _(+8 more lines)_`

<details>
<summary>Payload and response</summary>

**Request**

```json
{
  "language": "assembly",
  "source": ".global _start\n.section .data\nmsg: .ascii \"42\\n\"\n.section .text\n_start:\n    mov $1, %rax\n    mov $1, %rdi\n    lea msg(%rip), %rsi\n    mov $3, %rdx\n    syscall\n    mov $60, %rax\n    xor %rdi, %rdi\n    syscall",
  "tests": [
    {
      "stdin": "",
      "expected_stdout": "43\n"
    }
  ]
}
```

**Response**

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

Registry id `prolog` | 3/3 scenarios passed

| Scenario | Result | Build | Status | Duration | Memory |
|---|---|---|---|---:|---:|
| `accepted` | **PASS** | `ok` | `accepted` | 43 ms | 9432 kb |
| `time_exceeded` | **PASS** | `ok` | `time_exceeded` | 1006 ms | 8564 kb |
| `wrong_output` | **PASS** | `ok` | `wrong_output` | 58 ms | 9640 kb |

### `accepted`

stdout: `'42\n'`

<details>
<summary>Payload and response</summary>

**Request**

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

**Response**

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
      "duration_ms": 43,
      "memory_peak_kb": 9432
    }
  ]
}
```

</details>

### `time_exceeded`

<details>
<summary>Payload and response</summary>

**Request**

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

**Response**

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
      "duration_ms": 1006,
      "memory_peak_kb": 8564
    }
  ]
}
```

</details>

### `wrong_output`

stdout: `'42\n'`

<details>
<summary>Payload and response</summary>

**Request**

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

**Response**

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
      "duration_ms": 58,
      "memory_peak_kb": 9640
    }
  ]
}
```

</details>

---

## Notes on failures

- **Assembly** -- the challenge fixtures are GAS/AT&T syntax, while the `assembly`
  toolchain uses **NASM** (Intel syntax). NASM cannot parse AT&T directives, so
  they return `build_failed`. `build_failed.json` matches only incidentally (an
  invalid program fails to build under either assembler). NASM-syntax fixtures pass.
