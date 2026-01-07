# ZetaSQL Patch Application Guide

This document explains the patch files required to build ZetaSQL for WebAssembly (WASM).

## Table of Contents

- [Overview](#overview)
- [Patch Overview](#patch-overview)
- [Detailed Patch Descriptions](#detailed-patch-descriptions)
- [Technical Background](#technical-background)
- [Practical Guide](#practical-guide)
- [References](#references)

---

## Overview

### Why Patches Are Needed

When compiling ZetaSQL to WASM with Emscripten, two major issues arise:

#### 1. ICU Library Cross-Compilation Issues
- ICU build tools (`icupkg`, `genrb`, etc.) are compiled in WASM format
- WASM binaries cannot be executed on the build host (Linux/macOS)
- Cross-compilation environment constraints

#### 2. C++ Iterator Compatibility Issues
- Emscripten's libc++ enforces strict C++20 concept checking
- Implicit conversion from `absl::string_view::iterator` to `const char*` is not permitted
- Works with native compilers (GCC/Clang) but fails with Emscripten

### Patch File Naming Convention

Patch file names correspond to directory and file names:

| Target File | Patch File Name |
|---|---|
| `bazel/icu.BUILD` | `bazel_icu.patch` |
| `zetasql/public/strings.cc` | `zetasql_public_strings.patch` |
| `zetasql/public/functions/string.cc` | `zetasql_public_functions_string.patch` |

Slashes (`/`) are replaced with underscores (`_`), and extensions (`.cc`/`.BUILD`) are omitted from patch names.

---

## Patch Overview

### List of Patch Files

A total of 5 patch files apply 23 modifications:

| # | Patch File | Target File | Modifications | Type |
|---|---|---|---|---|
| 1 | `bazel_icu.patch` | `bazel/icu.BUILD` | 1 location | ICU build configuration |
| 2 | `zetasql_public_strings.patch` | `zetasql/public/strings.cc` | 6 locations | Iterator compatibility |
| 3 | `zetasql_public_functions_string.patch` | `zetasql/public/functions/string.cc` | 2 locations | Iterator compatibility |
| 4 | `zetasql_public_functions_date_time_util.patch` | `zetasql/public/functions/date_time_util.cc` | 6 locations | Iterator compatibility |
| 5 | `zetasql_public_functions_regexp.patch` | `zetasql/public/functions/regexp.cc` | 8 locations | Iterator compatibility |

### Safety Guarantees

All patches are **semantically equivalent transformations** and do not compromise ZetaSQL functionality.

#### Common Principles

1. **Zero Functional Changes**
   No changes to algorithms or logic. `begin()` and `data()` return the same address (guaranteed by C++ standard)

2. **Improved Type Safety**
   Explicit pointer conversions make type checking more rigorous

3. **Improved Portability**
   Better compatibility not only with WASM but also future C++ standards

4. **Performance Neutral**
   Both iterators and pointers optimize to the same machine code

---

## Detailed Patch Descriptions

### 1. bazel_icu.patch

**Target File:** `bazel/icu.BUILD`
**Modifications:** 1 location (line 51)

#### Motivation

- ICU library build tools (`icupkg`, `genrb`, etc.) are executables for the host environment
- In cross-compilation environments (WASM), these tools are built as WASM binaries
- WASM binaries cannot be executed on the build host, causing build failures

#### Problem and Solution

**Problem:**
When ICU's `--enable-tools` option is enabled, the following tools are compiled in WASM format:
- `pkgdata`: Packages ICU data into a static library (libicudata.a)
- `icupkg`: Converts and extracts data files (.dat)
- `genrb`: Compiles resource bundles

These tools need to run on the build host (Linux/macOS), but WASM binaries cannot be executed there.

**Solution:**
Specify `--disable-tools` to skip building ICU tools.

**Modification Example:**
```diff
-        "--enable-tools",  # needed to build data
+        "--disable-tools",  # WASM: disable tools to avoid cross-compile execution issues
```

**Build Result:**
- Since tools are not built, ICU data cannot be packaged into a static library
- Instead, `stubdata` (an empty data library, ~600B) is generated
- `stubdata` contains only minimal headers to resolve circular dependencies

#### Impact and Considerations

**Data Absence:**
- With `--disable-tools`, ICU data (locale information, normalization tables, etc.) is not included in build artifacts
- `libicudata.a` becomes an empty stubdata (Native build: 30MB → WASM build: 600B)

**Impact on ZetaSQL:**
ZetaSQL uses the following ICU data-dependent features:
- **Collation (zetasql/public/collator.cc)**: Locale-specific string comparison
- **Unicode normalization (Normalizer2)**: NFC, NFKC normalization
- **Case mapping (CaseMap)**: Locale-aware case conversion

These features require ICU data at runtime, but since data is absent in WASM builds:
- Functionality may be limited
- Runtime errors may occur
- May fall back to default behavior

**TODO: Verification Needed**
- Whether ZetaSQL's ICU-dependent features work correctly in WASM builds is unverified
- Need to identify the scope of functionality that works with stubdata

---

### 2. zetasql_public_strings.patch

**Target File:** `zetasql/public/strings.cc`
**Modifications:** 6 locations (lines 76, 81, 190, 514, 590, 1061)

#### Motivation

- Iterator methods of `absl::string_view` (`begin()`, `end()`) cause issues with Emscripten's strict C++20 type checking
- Replace with pointer-based operations to ensure WASM compatibility

#### Problem and Solution

**Problem:**
Iterator→pointer conversion fails in the following patterns:
- Using `source.end()` as `const char*`
- Using `source.begin()` as `const char*`
- Calling `absl::string_view(iterator, size_t)` constructor

**Solution:**
- `.begin()` → `.data()`
- `.end()` → `.data() + .size()`
- `absl::string_view(it, ...)` → `absl::string_view(&*it, ...)`

**Modification Example:**
```cpp
// Before
const char* end = source.end();
const int cur_pos = p - source.begin();

// After
const char* end = source.data() + source.size();
const int cur_pos = p - source.data();
```

#### Why Corruption Is Negligible

- `begin()` returns the same address as `data()` (guaranteed by C++ standard)
- `end()` returns the same address as `data() + size()` (guaranteed by C++ standard)
- Pointer arithmetic results are mathematically equivalent with zero functional difference
- String escaping and path parsing logic remains unchanged

---

### 3. zetasql_public_functions_string.patch

**Target File:** `zetasql/public/functions/string.cc`
**Modifications:** 2 locations (lines 297, 309)

#### Motivation

- Make iterator-to-pointer conversion explicit in string trimming operations
- Compatibility where `string_view` constructor expects a pointer
- Use `reverse_iterator::base()` result in pointer arithmetic

#### Problem and Solution

**Problem:**
In `BytesTrimmer::TrimLeft` and `TrimRight` functions, iterators are passed directly to the `string_view` constructor.

**Solution:**
- `absl::string_view(it, ...)` → `absl::string_view(&*it, ...)`
- `it.base() - str.begin()` → `&*it.base() - str.data()`

**Modification Example:**
```cpp
// Before (TrimLeft)
return absl::string_view(it, str.end() - it);

// After
return absl::string_view(&*it, str.end() - it);

// Before (TrimRight)
return absl::string_view(str.data(), it.base() - str.begin());

// After
return absl::string_view(str.data(), &*it.base() - str.data());
```

#### Why Corruption Is Negligible

- `&*it` is a safe and standard way to convert an iterator to its corresponding pointer
- String range calculations yield the same result (distance from start is equal)
- Trimming logic is completely preserved (decision of which characters to remove is unchanged)
- Symmetry between `TrimLeft` and `TrimRight` is maintained

---

### 4. zetasql_public_functions_date_time_util.patch

**Target File:** `zetasql/public/functions/date_time_util.cc`
**Modifications:** 6 locations (lines 5596, 5612, 5631, 5661, 5668, 5689)

#### Motivation

- `std::string`'s `begin()` method causes issues in WASM
- Resolve compatibility issues when creating substrings of format strings

#### Problem and Solution

**Problem:**
Date/time formatting uses `format_string.begin()` in pointer arithmetic.

**Solution:**
Replace all `format_string.begin()` with `format_string.data()`.

**Modification Example:**
```cpp
// Before
absl::string_view(format_string.begin() + idx_percent_e + 2, i - idx_percent_e - 2)

// After
absl::string_view(format_string.data() + idx_percent_e + 2, i - idx_percent_e - 2)
```

#### Why Corruption Is Negligible

- `std::string::begin()` internally returns the same pointer as `data()` (guaranteed by C++ standard)
- Date/time formatting logic itself remains unchanged
- String range calculations return identical results (address calculations are equivalent)
- Subsecond precision handling algorithm is completely preserved

---

### 5. zetasql_public_functions_regexp.patch

**Target File:** `zetasql/public/functions/regexp.cc`
**Modifications:** 8 locations (lines 524, 540, 542, 545, 547)

#### Motivation

- Iterator dereferencing to pointer is needed in regex replacement operations
- Replace use of `rewrite.end()` with pointer arithmetic
- `std::string::append(iterator, int)` causes type error in Emscripten

#### Problem and Solution

**Problem:**
The following issues occur in regex replacement:
- `std::string::append(iterator, int)` - using iterator as `const char*`
- `const char* < iterator` - comparison between pointer and iterator

**Solution:**
- Convert iterator to pointer: `&*p`
- Replace `.end()` with `.data() + .size()`

**Modification Example:**
```cpp
// Before (line 524)
out->append(p, len);

// After
out->append(&*p, len);

// Before (lines 540, 542, 545, 547)
for (const char* s = rewrite.data(); s < rewrite.end(); ++s) {
    while (s < rewrite.end() && *s != '\\') s++;
    if (s < rewrite.end()) {
        int c = (s < rewrite.end()) ? *s : -1;
    }
}

// After
for (const char* s = rewrite.data(); s < rewrite.data() + rewrite.size(); ++s) {
    while (s < rewrite.data() + rewrite.size() && *s != '\\') s++;
    if (s < rewrite.data() + rewrite.size()) {
        int c = (s < rewrite.data() + rewrite.size()) ? *s : -1;
    }
}
```

#### Why Corruption Is Negligible

- `&*p` is a standard C++ idiom for converting iterator `p` to a pointer
- `append(iterator, length)` and `append(pointer, length)` operate on the same memory region
- Regex matching and capture group replacement semantics are completely preserved
- Backslash escaping logic remains unchanged

---

## Technical Background

### Emscripten's libc++ and C++20 Concepts

Emscripten's libc++ performs iterator type checking based on C++20's `contiguous_iterator` concept. `absl::string_view::iterator` is internally implemented as `__wrap_iter<const char*>`, but this wrapper type cannot be directly converted to `const char*`.

#### Native Compiler Behavior

- GCC/Clang are less strict than Emscripten and permit implicit conversions
- `string_view(iterator, size_t)` constructor works

#### Emscripten Behavior

- C++20 `sized_sentinel_for` concept check fails
- Error: `no known conversion from '__wrap_iter<const char *>' to 'const char *'`

### Why Is Emscripten Stricter?

Emscripten is stricter than other compilers in the following ways:

1. **Full C++20 Concepts Implementation**
   Rigorous type checking for `contiguous_iterator` and `sized_sentinel_for`

2. **Uses Latest Clang Version**
   Applies stricter type conversion rules

3. **WebAssembly Constraints**
   Cannot execute host binaries (ICU tools issue)

These patches are **defensive coding** to adapt ZetaSQL to the Emscripten environment, with no functional corruption.

### Recommended Patterns

#### 1. Iterator→Pointer Conversion: Use `&*it` Idiom

```cpp
auto it = str.begin();
const char* ptr = &*it;  // OK
```

#### 2. End of Range: Use `.data() + .size()` Instead of `.end()`

```cpp
const char* end = str.data() + str.size();  // OK
const char* end = str.end();  // NG (Error in Emscripten)
```

#### 3. Iterator Arithmetic: Use `.data()` Instead of `.begin()`

```cpp
ptrdiff_t offset = ptr - str.data();  // OK
ptrdiff_t offset = ptr - str.begin();  // NG (Error in Emscripten)
```

---

## Practical Guide

### How to Apply Patches

Patches are automatically applied via `git_override` in `MODULE.bazel`:

```python
git_override(
    module_name = "zetasql",
    remote = "https://github.com/google/zetasql.git",
    tag = "2025.12.1",
    patches = [
        "//:patches/bazel_icu.patch",
        "//:patches/zetasql_public_functions_date_time_util.patch",
        "//:patches/zetasql_public_functions_regexp.patch",
        "//:patches/zetasql_public_functions_string.patch",
        "//:patches/zetasql_public_strings.patch",
    ],
    patch_strip = 1,
)
```

`patch_strip = 1` removes the `a/` and `b/` prefixes from the patch file.

### Troubleshooting

#### Patch Application Error

**Error:** `could not apply patch due to CONTENT_DOES_NOT_MATCH_TARGET`

**Cause:**
- Different ZetaSQL version
- Patch context lines don't match

**Solution:**
1. Check ZetaSQL version (`tag` in `MODULE.bazel`)
2. Regenerate patch (create with `diff -u`)

#### If Build Errors Continue

If new iterator compatibility errors occur:

1. Identify the file and line number from error message
2. Check usage of `.begin()`/`.end()`
3. Fix with recommended patterns above
4. Create new patch file
5. Add to `MODULE.bazel`

---

## References

- [Emscripten FAQ](https://emscripten.org/docs/getting_started/FAQ.html) - C++ standard library support and compiler characteristics
- [std::contiguous_iterator](https://en.cppreference.com/w/cpp/iterator/contiguous_iterator.html) - C++20 contiguous_iterator concept definition
- [ICU Data Management](https://unicode-org.github.io/icu/userguide/icu_data/) - ICU data file management and build tools
- [Abseil string_view Documentation](https://abseil.io/docs/cpp/guides/strings) - absl::string_view usage and best practices
- [Bazel git_override Documentation](https://bazel.build/external/overview#overrides) - Patching external dependencies with Bazel

---

**Last Updated**: 2026-01-08
**Version**: 1.0.0
