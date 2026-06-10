# Issues — Problems We Were Blocked On

Problems that cost real time (roughly an hour or more), what the symptom was,
what the root cause turned out to be, and how it was resolved. These are the
non-obvious failures — the ones worth recording so they are not rediscovered.

---

## nsjail worked for interpreters but failed for compiled languages

**Symptom.** Python, Bash, and JS ran fine in the sandbox. C, C++, Java, and
other compiled languages failed — the compiler or the produced binary could not
find what it needed inside the jail.

**Root cause.** Our first sandbox model was interpreter-shaped: one source file,
one runtime, run it. Compiled languages need their toolchain and shared libraries
reachable during the build phase, and the produced binary needs a working `/dev`
and the right chroot/bind-mount layout at run time. The interpreter path simply
never exercised those requirements.

**Resolution.** Reworked the nsjail invocation to set up chroot, the necessary
bind mounts, and a `/dev` mount that compiled artifacts depend on. This was the
single largest debugging effort of the build and reshaped how we think about the
sandbox (see plan-evolution, 2026-05-28).

---

## `memory_peak_kb` always returned 0

**Symptom.** The field was present in every response but always `0`, so peak
memory reporting was useless and `memory_exceeded` could not be detected
reliably.

**Root cause.** Nothing was actually accounting memory per job — there was no
mechanism reading real usage from the kernel.

**Resolution.** Put each sandbox in a per-job cgroup v2 slice and read
`memory.peak` after `cmd.Wait()`, with swap disabled in the slice so the figure
is a true RSS peak. The same accounting now drives `memory_exceeded` detection
(ADR-8). Verified end to end: the `TestMemoryExceeded` integration case passes
inside the container.

---

## False-positive `memory_exceeded` / time handling under cancellation

**Symptom.** Jobs that were actually cut off for *time* reasons, or cancelled via
context, could be misreported, and pipes were not always closed cleanly on the
cancellation path.

**Root cause.** The status-derivation and the context-cancellation paths did not
fully agree on who "won" when a job ended for more than one reason at once, and
the child stdout pipe was not guaranteed closed when the context was cancelled.

**Resolution.** Made context-cancellation handling explicit in job execution and
enforced pipe closure in `nsjail.go`, so a cancelled or timed-out job produces
the correct single status instead of a misleading memory verdict.

---

## nsjail has no `--version`, so `/readyz` and `/info` reported an error string

**Symptom.** The version probe for nsjail returned the text of nsjail's
"unknown flag" error instead of a version, polluting `/readyz` and `/info`.

**Root cause.** nsjail genuinely exposes no `--version` flag; probing for one can
only fail.

**Resolution.** Report the version from the pinned submodule source (we build
nsjail from a known tag — ADR-12), instead of asking the binary at runtime.

---

## Toolchain warnings leaked into language version strings

**Symptom.** Some language version probes (e.g. JVM-based, and lua/verilog)
returned warning text or junk instead of a clean version string in `/info`.

**Root cause.** The probe captured combined stderr/stdout, and several toolchains
emit warnings or write their version to an unexpected stream.

**Resolution.** Added `probe_args` to the language config and filtered probe
output so the reported version is the version, not the toolchain's incidental
chatter.

---

## SIGXCPU was not mapped to the right status

**Symptom.** A CPU-time-exceeded kill (SIGXCPU) did not map cleanly to
`time_exceeded`.

**Root cause.** The signal-to-status mapping in the status parser did not account
for SIGXCPU specifically.

**Resolution.** Added the SIGXCPU mapping so CPU-time limit kills report
`time_exceeded` like the wall-time path. Covered by `TestTimeExceeded`.

---

## Docker build used a stale submodule

**Symptom.** Builds could pick up an out-of-date or missing nsjail submodule.

**Root cause.** The image build did not guarantee the submodule was initialized
and updated before building.

**Resolution.** The `make build` target now runs
`git submodule update --init --recursive` before the compose build, so the pinned
nsjail is always present and current.

---

## Output capping: stdout was bounded, stderr was not

**Symptom.** A program spewing to stderr could still pressure host memory even
though stdout was capped.

**Root cause.** Only the stdout pipe went through `io.LimitReader`; stderr was
read without the same bound.

**Resolution.** Applied the same cap-and-truncate treatment to stderr, closing
the asymmetry.

---

## Cross-platform tests (disk-free) failed off-Linux

**Symptom.** Disk-free / diskspace-related tests behaved differently on the
non-Linux dev machine vs. the Linux container.

**Root cause.** The syscall surface for free-space differs across platforms, and
the tests assumed Linux semantics.

**Resolution.** Made the disk-free tests cross-platform so the suite is green on
the dev host (macOS) and in the Linux container alike.
