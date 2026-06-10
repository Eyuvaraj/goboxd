# Postmortem

Written after PR submission. Honest accounting of what was hard, where AI advice needed correction, and what we would do differently if starting over.

---

## What Was Hard

**The sandbox model had to be rebuilt once.** The first version was interpreter-shaped: one source file, one runtime binary, done. When compiled languages were added, the entire mental model broke. Compilers need their toolchain and shared libraries reachable at build time; the produced artifact needs a working `/dev` and the right bind-mount layout at run time. Nothing about the interpreter path hinted at this. The rework consumed more time than any other single task and changed how we reasoned about the sandbox going forward.

**cgroup v2 memory accounting was non-obvious.** `memory_peak_kb` returned `0` for the entire early phase because nothing actually hooked into the kernel. Getting it right required per-job cgroup slices, reading `memory.peak` after `cmd.Wait()` (not before, not during), and disabling swap in the slice so the figure is true RSS. The order of operations around `cmd.Wait()` and pipe closure also triggered a separate bug: cancellation and time-exceeded signals could race and produce the wrong status.

**nsjail's interface is not documented for programmatic use.** The flag names, the `[E]` log format, the signal encoding in diagnostic output, the absence of `--version`: none of this is in a man page or README section written for a calling process. We read the source.

---

## Where AI Got It Wrong

**nsjail flags.** AI suggested flag names and argument shapes (for rlimit settings, cgroup path construction, and the seccomp string syntax) that were plausible but wrong for nsjail 3.4. Every flag had to be verified against the source or the actual `--help` output before use. The suggestions were good starting-point guesses, not reliable answers.

**Concurrency design.** Early AI suggestions leaned toward a worker-pool goroutine model with a channel of pre-spawned workers. The semaphore pattern (`chan struct{}`) is simpler, has fewer failure modes, and is semantically identical for this workload. We discarded the worker-pool idea without implementing it.

**Status parsing logic.** The initial AI-suggested approach to mapping nsjail diagnostic output to status strings was based on string contains checks on combined output. The actual log format uses structured `[E]`/`[W]` prefixes and the signal information appears in a specific field. The parser had to be written from scratch after reading real nsjail output.

---

## What We Would Do Differently

**Start with a compiled language in the sandbox, not an interpreter.** Starting with Python gave false confidence that the sandbox worked. C or Java as the first language would have surfaced the chroot/bind-mount/dev requirements immediately and shaped the architecture from the beginning.

**Write the integration test container path on day one.** Unit tests run fine on macOS. Everything that matters, nsjail namespaces, cgroup accounting, seccomp, runs only inside Docker. We had unit tests early and integration tests late. The right order is the reverse: get one language working end-to-end in the container, then build the unit test layer around what you have proven.

**Allocate one day for the demo rehearsal explicitly.** The "add a language in 30 minutes" criterion is 25% of the score and the only rubric item that is purely a time trial. Knowing that the architecture supports it is not the same as having done it under a timer.

---

## What Held Up

The things that worked without incident: the language registry YAML design (no Go changes for new languages), the chi middleware chain (BodyLimit and StructuredLogger composed cleanly), the validate package (all validation centralised before any execution), and load shedding (the bounded queue with 503 was straightforward once the semaphore pattern was settled).

The judging rubric drove every priority decision. When a task was not clearly on the rubric, we did not build it.
