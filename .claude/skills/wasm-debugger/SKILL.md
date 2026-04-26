---
name: wasm-debugger
description: >
  Standalone WASM binary debugging using WABT (WebAssembly Binary Toolkit).
  Uses wasm-objdump, wasm-interp, wasm2wat, and wasm-decompile to identify
  the cause of WASM function crashes without a host runtime.
  Use this skill when the user mentions "unreachable", "WASM crash",
  "wasm error", "WASM debug", "function doesn't work", "wasm-objdump",
  or "wasm-interp".
---

# WASM Debugger (WABT)

Use WABT (WebAssembly Binary Toolkit) to operate on a WASM binary directly
and pinpoint the cause of a runtime error. The goal is to isolate the issue
to the WASM binary itself, without going through a host runtime such as
wazero or wasmtime.

## Prerequisites

```bash
brew install wabt  # macOS
```

Key tools:
- **wasm-objdump** — inspect binary structure (Export/Import/Name sections, etc.)
- **wasm-interp** — execute WASM functions standalone
- **wasm2wat** — convert binary to text format (WAT)
- **wasm-decompile** — decompile binary into pseudo-code

## Debugging flow

### Step 1: Inspect the binary structure

First confirm what the WASM binary exports and imports.

```bash
# List of exported functions — what can be called from outside
wasm-objdump -x target.wasm -j Export

# List of imports — what the WASM expects from the host
wasm-objdump -x target.wasm -j Import

# Section overview
wasm-objdump -h target.wasm
```

Example Export output:
```
Export[23]:
 - func[30] <debug_test_analyzer_options> -> "debug_test_analyzer_options"
 - func[36] <debug_test_analyze> -> "debug_test_analyze"
 - func[42] <analyze_statement_proto> -> "analyze_statement_proto"
 - func[41640] <malloc> -> "malloc"
```

This gives the **function-index ↔ name** mapping. If a runtime stack trace
shows `$42`, you know it is `analyze_statement_proto`.

### Step 2: Run a function standalone with wasm-interp

Execute a WASM function directly without a host runtime. This separates
host-side issues from WASM-internal issues.

```bash
# --dummy-import-func: stub all imports with a no-op (returns 0)
wasm-interp target.wasm --dummy-import-func -r "function_name"

# Calling a function with arguments
wasm-interp target.wasm --dummy-import-func -r "add" -a "i32:3" -a "i32:5"
```

On success:
```
debug_test_basic() => i32:20114248
```

On crash:
```
called host wasi_snapshot_preview1.clock_time_get(i32:0, i64:1, i32:16776328) => i32:0
called host wasi_snapshot_preview1.fd_write(i32:2, i32:16776160, i32:2, i32:16776156) => i32:0
called host wasi_snapshot_preview1.fd_write(i32:2, i32:16776272, i32:2, i32:16776268) => i32:0
debug_test_analyzer_options() => error: unreachable executed
```

### Step 3: Reading the wasm-interp output

`--dummy-import-func` logs every host-function call. These calls are clues
about the cause of the crash.

**fd_write file descriptors**:
- `fd_write(i32:1, ...)` — write to **stdout**, normal output.
- `fd_write(i32:2, ...)` — write to **stderr**, **error log**.

If `fd_write(fd=2)` is called several times before the crash, the C++ side
wrote a `CHECK` failure or `LOG(FATAL)` message to stderr and then aborted.

**clock_time_get → fd_write(fd=2) → unreachable** pattern:
1. `clock_time_get` — fetching a timestamp for the log entry.
2. `fd_write(fd=2)` — writing the error message to stderr.
3. `unreachable` — `abort()` runs.

This pattern is classic abseil `CHECK()` / `LOG(FATAL)` failure.
`--dummy-import-func` does not show the actual content written by
`fd_write`, but the pattern alone identifies the kind of failure.

**Immediate unreachable with no fd_write**:
A crash with no log output usually indicates a C++ exception (`throw`)
that has been converted to `abort()` because Emscripten was built without
exception support.

### Step 4: Bisect by export

When several exports exist, run them from simplest to most complex and
find the first one that fails:

```bash
wasm-interp target.wasm --dummy-import-func -r "func_a"  # OK
wasm-interp target.wasm --dummy-import-func -r "func_b"  # OK
wasm-interp target.wasm --dummy-import-func -r "func_c"  # => error: unreachable
```

If `func_c` is the first to crash, the cause is something specific to
`func_c` that `func_b` does not exercise. Add finer-grained debug
exports on the C++ side and bisect down to the offending step.

### Step 5: Check the Name section

```bash
wasm-objdump -x target.wasm -j Name
```

If debug symbols are present, internal function names are visible. They
are usually empty in optimized builds — fall back to the Export section
(Step 1) and the bisect approach (Step 4).

### Step 6: Drill into the code

When you need to read the function body:

```bash
# Disassembly of a specific function
wasm-objdump -d target.wasm | grep -A 30 "func\[41434\]"

# High-level decompilation (most readable)
wasm-decompile target.wasm -o target.dcmp
grep -A 30 "function 41434" target.dcmp

# WAT text form (precise but verbose)
wasm2wat target.wasm -o target.wat
```

A full dump of a large WASM binary (tens of MB) can be several GB. Always
narrow with `grep`.

### Step 7: Execution trace (last resort)

```bash
# Trace every instruction — extremely slow and the output is huge
wasm-interp target.wasm --dummy-import-func -r "func_name" --trace 2>&1 | tail -100
```

You see the instruction sequence right before the crash. The instructions
just before the final `unreachable` often reveal the cause. Output is
huge for big functions, so `tail` is the only practical way to read it.

## Causes of unreachable

| Pattern | wasm-interp signature | Cause |
|---------|----------------------|------|
| `clock_time_get` → `fd_write(fd=2)` × N → unreachable | CHECK/LOG(FATAL) | abseil CHECK failed; the condition is in the stderr message |
| `fd_write(fd=2)` → unreachable | assertion | C/C++ `assert()` failed |
| immediate unreachable (no fd_write) | C++ exception | `throw` converted to `abort()` by `-fno-exceptions` |
| deep call stack → unreachable | stack overflow | `-sSTACK_SIZE` too small |
| crash after `malloc` → unreachable | out of memory | `-sINITIAL_MEMORY` / `-sMAXIMUM_MEMORY` too small |

## Useful wasm-interp options

```bash
# WASI mode (auto-invokes _start)
wasm-interp target.wasm --wasi -r "func_name"

# verbose (module-load detail)
wasm-interp target.wasm --dummy-import-func -r "func_name" -v

# adjust stack size
wasm-interp target.wasm --dummy-import-func -V 10000 -r "func_name"

# run every export in turn
wasm-interp target.wasm --dummy-import-func --run-all-exports

# pass environment variables (WASI)
wasm-interp target.wasm --wasi -e "KEY=VALUE" -r "func_name"
```
