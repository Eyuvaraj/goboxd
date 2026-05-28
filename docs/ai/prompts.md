# AI Usage Log

This document tracks AI interactions during the development of goboxd, as required by the hackathon specification.

## 2026-05-23 [Running jobs and limiting server load]

**Prompt:** "I need to run nsjail from my Go code, but I want to make sure it doesn't crash my server. Also, how do I make sure my server only runs a few jobs at a time so it doesn't freeze under heavy load?"

**Response summary:**
- The AI suggested using something called `io.LimitReader` to cut off the text output if it gets too long.
- To limit jobs, it showed me how to use a Go channel (like a waiting line) to only allow a few jobs to run at the same time. 

**What we used / didn't use:**
- I used the `io.LimitReader` and the channel waiting line exactly as shown.
- They were simple to understand and didn't require me to install any extra packages. 

## 2026-05-23 [Adding Swagger API Docs]

**Prompt:** "I want to add Swagger documentation to my Go API so users can see the endpoints and test them. What's the easiest way to do this without writing everything by hand?"

**Response summary:**
- The AI suggested using `swaggo/swag` which reads comments above my Go functions to generate the Swagger files automatically.
- It also gave me some example comments to add above my routes to explain the inputs and outputs.

**What we used / didn't use:**
- I used `swaggo/swag` because it keeps the documentation right next to the code.
- I didn't use its suggestion to serve a fancy Swagger UI web page because the hackathon rules say no web UIs, so I just kept the generated YAML file.

## 2026-05-23 [Fixing file permissions]

**Prompt:** "When I create temporary folders for the sandbox, I want to make sure other users on the server can't read the files inside. How do I set strict folder permissions in Go?"

**Response summary:**
- The AI explained that `os.MkdirTemp` uses safe permissions by default (`0700`), meaning only the owner can read it.
- It also showed me how to use `os.Chmod` to lock down permissions on files that I copy into the folder.

**What we used / didn't use:**
- I used its advice to double-check my folder permissions.
- I ended up just relying on the safe defaults of `MkdirTemp` instead of manually running `Chmod` everywhere.

## 2026-05-24 [Securing the sandbox]

**Prompt:** "I need to make my nsjail sandbox secure. How do I block dangerous things? Also, how can I tell if a program got killed because it used too much memory?"

**Response summary:**
- The AI gave me a list of system calls to block (using a Kafel policy string).
- It also explained that when a program runs out of memory in cgroup v2, the system sends it a "SIGKILL" signal (signal 9), so I can check for that to know if it ran out of memory.

**What we used / didn't use:**
- I used the block list it gave me for the `--seccomp_string` flag.
- I also added the check for signal 9 to correctly report memory errors to the user instead of generic runtime errors.

## 2026-05-24 [Fixing bugs with logs and language order]

**Prompt:** "I have two weird bugs. My code is matching the word 'nsjail' anywhere in the logs and marking everything as an internal error. Also, my API returns the list of languages in a random order every time I refresh. How do I fix these?"

**Response summary:**
- The AI explained that Go maps always loop in a random order by design, and I should use a separate list (slice) to keep them in order.
- For the log bug, it pointed out that real nsjail errors start with `[E][`, so I should look for that instead of just the word "nsjail".

**What we used / didn't use:**
- I added a list to keep my languages ordered, and I changed my code to search for `[E][`.
- I didn't use the AI's suggestion to download a third-party sorted map package because I wanted to keep things simple.

## 2026-05-24 [Upgrading Go in Docker]

**Prompt:** "My code linter is failing inside my Docker build. It says 'golangci-lint v2 requires Go >= 1.25' but I think my Docker image has an older version. How do I upgrade it?"

**Response summary:**
- The AI told me that I just need to change the first line in my Dockerfile from `FROM golang:1.24` to `FROM golang:1.26-bookworm` to get the latest version.
- It also mentioned that the `bookworm` tag is better because it's based on a newer Linux version.

**What we used / didn't use:**
- I used the `golang:1.26-bookworm` tag it suggested, and the linter started passing immediately.
- I didn't bother pinning the exact minor version (like 1.26.3) because letting it grab the latest patch is easier to maintain.

## 2026-05-24 [Capping user limits]

**Prompt:** "My API lets users send custom time and memory limits for their code. But right now, someone could send a time limit of 99999 seconds and freeze up a spot in the queue forever. How do I stop this?"

**Response summary:**
- The AI suggested adding a validation check that compares the user's requested limit against the default limit for that language.
- It recommended rejecting any request where the user asks for more than double the default limit.

**What we used / didn't use:**
- I used the "double the default" rule because it felt like a fair balance between flexibility and safety.
- I added a clean error message to reject these bad requests with a 400 Bad Request status.

## 2026-05-24 [Tracking the queue size]

**Prompt:** "I am using a channel to limit how many jobs run at once. How can I safely check how many jobs are currently waiting in that channel so I can show it on my `/info` endpoint?"

**Response summary:**
- The AI told me I could use the `len()` function on my buffered channel to see how many spots are currently taken.
- It also showed me how to use `sync/atomic` counters to safely track total jobs across different threads.

**What we used / didn't use:**
- I used the atomic counters exactly as shown because they are perfectly safe to use when lots of requests come in at once.
- I used `len(channel)` to calculate the active queue size, which made my `/info` stats much more useful.

## 2026-05-26 [Adding Go support]

**Prompt:** "I'm trying to let users run Go code in my sandbox, but it keeps failing because it's trying to download modules and we have no internet inside the jail. What environment variables do I need to set to make `go build` work completely offline?"

**Response summary:**
- It told me to set `GO111MODULE=off` so Go doesn't look for modules.
- It suggested `CGO_ENABLED=0` to keep it simple.
- It recommended moving the cache to a folder where the jail is allowed to write (`GOCACHE=/.cache/go-build`).

**What we used / didn't use:**
- I added those exact variables to the environment list in my language config file.
- Go compiled perfectly on the first try.

## 2026-05-28 [Java crashing on Device]

**Prompt:** "My Java programs are crashing in the sandbox with an error about compressed class space on my Device. My memory limit is set to 512 MB. Why is this happening and how do I fix it?"

**Response summary:**
- The AI explained that Java on ARM64 chips reserves about 1 GB of virtual memory just to start up, even if it doesn't use it.
- It said I should raise my virtual memory limit (`RLIMIT_AS`) to something huge like 4096 MB, and just rely on cgroups to limit the actual physical memory.

**What we used / didn't use:**
- I raised the virtual memory limit to 4096 MB like it said, and Java started working immediately.
- The cgroup limit still properly stopped programs that tried to use too much real memory.

## 2026-05-28 [Writing tests and checking code]

**Prompt:** "can you build and test all the apis and its output and verify them and do the needful updates... and audit the code and write tests"

**Response summary:**
- The AI looked through my code and noticed I didn't have any tests for my API endpoints.
- It wrote a bunch of tests to check things like bad JSON, missing files, and files being too big.
- It also caught a tiny bug where I was deleting leading spaces from the output, but the rules say I should only delete trailing spaces.

**What we used / didn't use:**
- I used all the tests it generated because they tested a lot of edge cases I hadn't thought of.
- I also used its fix to use `TrimRight` instead of `TrimSpace` so my output checker follows the rules perfectly.
