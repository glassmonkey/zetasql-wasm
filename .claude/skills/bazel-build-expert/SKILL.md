---
name: bazel-build-expert
description: >
  Specialist skill for analyzing Bazel + Emscripten + C++ WASM build errors.
  Reads the build log (wasm/build.log) and proposes evidence-backed fixes.
  Use this skill when the user mentions "build error", "build failed",
  "build won't pass", "make build failed", "Bazel error", "compile error",
  or "linker error", or when asked to analyze wasm/build.log.
---

# Bazel Build Expert

Analyze build errors from the Bazel + Emscripten + C++ toolchain.
This project compiles ZetaSQL into WASM using Bazel + Emscripten inside Docker.

## Analysis flow

1. Extract the error message from `wasm/build.log` (ask the user if the path differs).
2. Identify the kind and scope of the error:
   - Target build (Emscripten/WASM) vs. host-tool build (GCC/clang x86_64)
   - Compile error / linker error / Bazel config error / toolchain error / patch error
   - Distinguish `[for tool]` from `[for target]`.
3. Search the upstream repositories for known issues (use `gh api` when possible).
4. Present an evidence-backed fix, or honestly report when evidence is insufficient.

## Evidence standards

The only acceptable evidence:

- **Official documentation**: direct quotes from the Bazel / Emscripten / library
  documentation, with URLs.
- **GitHub issue/PR**: from official repos like bazelbuild/bazel,
  emscripten-core/emscripten (issue number + URL + quote).
- **Source code**: the actual line of `.bazelrc`, toolchain definition, or
  build config (file path + line number).
- **Command output**: real output from `gh api` or other CLI tools.

The following are **not** acceptable as evidence:

- WebSearch results alone (blog posts, Stack Overflow are reference only).
- Vague hedges like "in general", "usually".
- Unverifiable reports or speculation.

## Citation format

```
**Source**: [title](URL)
> "quoted text"
**Status**: Open/Closed (for issues)
```

## Report format

### When evidence exists

```markdown
## Problem
[error description]

## Root cause
[explanation]

**Source**: [link]
> "quote"

## Fix
[concrete change]
```

### When evidence is insufficient

```markdown
## Problem
[error description]

## Investigation
The following were checked but no definitive fix was found:
1. [checked item] - [result]

## Suggestion (no firm evidence)
The following is suggested as something to **try**, but there is no firm
evidence that it will work:
1. [approach] - rationale: [reasoning] / risk: [side effects] / verification: [how to confirm]
```

## Common patterns

### Patch file error
```
Cannot find patch file: /home/builder/workspace/patches/xxx.patch
```
→ Cross-check the `patches` list in `MODULE.bazel` against the actual files in `wasm/assets/patches/`.

### Absolute-path inclusion
```
ERROR: absolute path inclusion(s) found in rule
```
→ Confirm `[for tool]` vs `[for target]`; check the scope of `--features=-layering_check`.

### Unknown compiler flag
```
clang: error: unknown argument: '-fxxx'
```
→ Distinguish GCC-only flags from clang-only flags; check `--copt` vs `--host_copt`.

### Linker error
```
undefined reference to `xxx'
```
→ Look for missing `deps`; check Emscripten system-library settings.

### Bazel target resolution error
```
no such target '@zetasql//zetasql/xxx:yyy'
```
→ Verify the actual target name in the upstream ZetaSQL repository's BUILD file.

## Project-specific paths

- Builds run inside Docker (environment is isolated).
- `wasm/assets/bridge.cc` — C++ bridge code.
- `wasm/assets/BUILD.bazel` — Bazel build definitions.
- `wasm/assets/MODULE.bazel` — dependencies (ZetaSQL version, patches).
- `wasm/assets/.bazelrc` — Bazel configuration flags.
- `wasm/assets/patches/` — patches applied to ZetaSQL.
- `wasm/Makefile` — `make build` / `make rebuild` entry points.
- `wasm/script/build.sh` — Docker build orchestration.

## Prohibitions

- Do not present speculation as fact ("this will fix it" → "I suggest trying").
- Do not cite unverified issue/PR contents.
- Do not propose GCC-only flags for clang/Emscripten.
