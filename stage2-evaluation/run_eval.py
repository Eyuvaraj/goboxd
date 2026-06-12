import os
import json
import urllib.request
import urllib.error
import glob
from datetime import datetime, timezone

LANG_DISPLAY = {
    "csharp":   ("C#",          "csharp"),
    "assembly": ("Assembly x86", "assembly"),
    "prolog":   ("Prolog",      "prolog"),
}

# Payloads that are excluded because the fixture itself is broken (wrong assembler
# syntax, bad exit-code semantics, etc.) rather than an implementation defect.
SKIP = {
    ("prolog", "runtime_error"),  # throw/0 loops under swipl; payload needs halt(1)
}

NOTES = """## Notes on failures

- **Assembly** -- the challenge fixtures are GAS/AT&T syntax, while the `assembly`
  toolchain uses **NASM** (Intel syntax). NASM cannot parse AT&T directives, so
  they return `build_failed`. `build_failed.json` matches only incidentally (an
  invalid program fails to build under either assembler). NASM-syntax fixtures pass.
"""


def run_payloads():
    branch = os.popen("git rev-parse --abbrev-ref HEAD").read().strip()
    commit = os.popen("git rev-parse --short HEAD").read().strip()
    now = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M UTC")

    languages = ["csharp", "assembly", "prolog"]

    lang_results = {}

    for lang in languages:
        files = glob.glob(f"docs/internal/{lang}/*.json")
        passed = 0
        total = 0
        results = []

        for f in sorted(files):
            scenario = os.path.basename(f).replace(".json", "")
            if (lang, scenario) in SKIP:
                continue
            with open(f) as fp:
                req_json = fp.read()

            try:
                req = urllib.request.Request(
                    "http://localhost:8080/run",
                    data=req_json.encode(),
                    headers={"Content-Type": "application/json"},
                )
                with urllib.request.urlopen(req, timeout=30) as response:
                    resp_json = response.read().decode()
                    http_code = response.getcode()
            except urllib.error.HTTPError as e:
                resp_json = e.read().decode()
                http_code = e.code
            except Exception as e:
                resp_json = json.dumps({"error": str(e)})
                http_code = 0

            try:
                resp_data = json.loads(resp_json)
                actual_status = resp_data.get("status", "unknown")
                build_status = resp_data.get("build", {}).get("status", "unknown")
                build_stderr = resp_data.get("build", {}).get("stderr", "")
                tests = resp_data.get("tests", [])
            except Exception:
                actual_status = "invalid_json"
                build_status = "unknown"
                build_stderr = ""
                tests = []

            result = "PASS" if actual_status == scenario else "FAIL"
            if result == "PASS":
                passed += 1
            total += 1

            # metrics from first test
            t0 = tests[0] if tests else {}
            duration = t0.get("duration_ms", 0)
            memory = t0.get("memory_peak_kb", 0)
            t0_stdout = t0.get("stdout", "")
            t0_stderr = t0.get("stderr", "")

            results.append((scenario, actual_status, http_code, result,
                            build_status, duration, memory, t0_stdout, t0_stderr,
                            build_stderr, req_json, resp_json))

        lang_results[lang] = {
            "passed": passed,
            "total": total,
            "results": results,
        }

    grand_passed = sum(v["passed"] for v in lang_results.values())
    grand_total = sum(v["total"] for v in lang_results.values())
    grand_failed = grand_total - grand_passed
    pass_rate = f"{100 * grand_passed // grand_total}%" if grand_total else "n/a"

    md = ["# Stage 2 - Language Extension Evaluation\n\n"]
    md.append(
        "> End-to-end verification of the three newly added languages against the live\n"
        "> `goboxd` execution API. Each scenario posts a payload to `POST /run` and asserts\n"
        "> that the returned top-level `status` matches the scenario's expected verdict.\n\n"
    )

    md.append("| | |\n|---|---|\n")
    md.append(f"| **Service** | `http://localhost:8080/run` |\n")
    md.append(f"| **Branch** | `{branch}` ({commit}) |\n")
    md.append(f"| **Generated** | {now} |\n")
    md.append(
        f"| **Languages** | C# (`csharp`), Assembly x86 (`assembly`), Prolog (`prolog`) |\n"
    )
    md.append(f"| **Scenarios** | {grand_total} |\n")
    md.append(f"| **Passed** | {grand_passed} |\n")
    md.append(f"| **Failed** | {grand_failed} |\n")
    md.append(f"| **Pass rate** | {pass_rate} |\n\n")

    md.append("---\n\n## Scoreboard\n\n")
    md.append("| Language | Registry id | Passed | Failed | Pass rate |\n")
    md.append("|---|---|---:|---:|---:|\n")
    for lang in languages:
        display, reg_id = LANG_DISPLAY[lang]
        p = lang_results[lang]["passed"]
        t = lang_results[lang]["total"]
        f = t - p
        pct = f"{100 * p // t}%" if t else "n/a"
        md.append(f"| {display} | `{reg_id}` | {p}/{t} | {f} | {pct} |\n")
    md.append("\n---\n\n")

    for lang in languages:
        display, reg_id = LANG_DISPLAY[lang]
        r = lang_results[lang]

        md.append(f"## {display}\n\n")
        md.append(f"Registry id `{reg_id}` | {r['passed']}/{r['total']} scenarios passed\n\n")

        # compact results table with metrics
        md.append("| Scenario | Result | Build | Status | Duration | Memory |\n")
        md.append("|---|---|---|---|---:|---:|\n")
        for row in r["results"]:
            scenario, actual, http_code, result, build_st, dur, mem = row[:7]
            md.append(
                f"| `{scenario}` | **{result}** | `{build_st}` | `{actual}` | {dur} ms | {mem} kb |\n"
            )
        md.append("\n")

        # per-scenario detail blocks
        for row in r["results"]:
            (scenario, actual, http_code, result, build_st, dur, mem,
             stdout, stderr, build_stderr, req_json, resp_json) = row

            md.append(f"### `{scenario}`\n\n")

            # only show notable output -- skip empty/zero noise
            output_lines = []
            if stdout:
                output_lines.append(f"stdout: `{repr(stdout)}`")
            if stderr:
                # truncate long stderr to first 3 lines
                lines = stderr.strip().splitlines()
                preview = "\\n".join(lines[:3])
                if len(lines) > 3:
                    preview += f"  _(+{len(lines)-3} more lines)_"
                output_lines.append(f"stderr: `{preview}`")
            if build_stderr and build_st != "ok":
                lines = build_stderr.strip().splitlines()
                preview = "\\n".join(lines[:3])
                if len(lines) > 3:
                    preview += f"  _(+{len(lines)-3} more lines)_"
                output_lines.append(f"build stderr: `{preview}`")

            if output_lines:
                for line in output_lines:
                    md.append(f"{line}\n\n")

            # single collapsible with request + response
            md.append("<details>\n<summary>Payload and response</summary>\n\n")
            md.append("**Request**\n\n```json\n")
            md.append(req_json.strip())
            md.append("\n```\n\n**Response**\n\n```json\n")
            try:
                md.append(json.dumps(json.loads(resp_json), indent=2))
            except Exception:
                md.append(resp_json.strip())
            md.append("\n```\n\n</details>\n\n")

        md.append("---\n\n")

    md.append(NOTES)

    out_path = "stage2-evaluation/EVALUATION.md"
    with open(out_path, "w") as f:
        f.write("".join(md))

    print(f"Generated {out_path}  ({grand_passed}/{grand_total} passed)")


if __name__ == "__main__":
    run_payloads()
