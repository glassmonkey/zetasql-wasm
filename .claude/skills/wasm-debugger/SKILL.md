---
name: wasm-debugger
description: >
  WABT (WebAssembly Binary Toolkit) を使った WASM バイナリ単体でのデバッグスキル。
  wasm-objdump, wasm-interp, wasm2wat, wasm-decompile を使い、
  ホストランタイムなしで WASM 関数のクラッシュ原因を特定する。
  ユーザーが「unreachable」「WASM クラッシュ」「wasm error」「WASM デバッグ」
  「関数が動かない」「wasm-objdump」「wasm-interp」と言及した場合にこのスキルを使うこと。
---

# WASM Debugger (WABT)

WABT (WebAssembly Binary Toolkit) で WASM バイナリを単体操作し、ランタイムエラーの原因を特定する。
ホストランタイム (wazero, wasmtime 等) を介さず、WASM バイナリだけで問題を切り分けるのが目的。

## 前提条件

```bash
brew install wabt  # macOS
```

主要ツール：
- **wasm-objdump** — バイナリの構造を検査（Export/Import/Name セクション等）
- **wasm-interp** — WASM 関数をスタンドアロンで実行
- **wasm2wat** — バイナリをテキスト形式 (WAT) に変換
- **wasm-decompile** — バイナリを擬似コードにデコンパイル

## デバッグフロー

### Step 1: バイナリ構造の把握

まず WASM バイナリがどんな関数をエクスポート/インポートしているか確認する。

```bash
# エクスポート関数の一覧 — 外から呼べる関数
wasm-objdump -x target.wasm -j Export

# インポート関数の一覧 — WASM が外部に要求する関数
wasm-objdump -x target.wasm -j Import

# セクション全体の概要
wasm-objdump -h target.wasm
```

Export の出力例：
```
Export[23]:
 - func[30] <debug_test_analyzer_options> -> "debug_test_analyzer_options"
 - func[36] <debug_test_analyze> -> "debug_test_analyze"
 - func[42] <analyze_statement_proto> -> "analyze_statement_proto"
 - func[41640] <malloc> -> "malloc"
```

ここで **関数インデックスと名前の対応** が取れる。
ランタイムのスタックトレースに `$42` とあれば `analyze_statement_proto` と分かる。

### Step 2: wasm-interp で関数を単体実行

ホストランタイムなしで WASM 関数を直接実行する。ホスト側の問題か WASM 内部の問題かを切り分けられる。

```bash
# --dummy-import-func: 全インポートをダミー実装（0を返す）で埋める
wasm-interp target.wasm --dummy-import-func -r "関数名"

# 引数付きの関数を呼ぶ場合
wasm-interp target.wasm --dummy-import-func -r "add" -a "i32:3" -a "i32:5"
```

成功時：
```
debug_test_basic() => i32:20114248
```

クラッシュ時：
```
called host wasi_snapshot_preview1.clock_time_get(i32:0, i64:1, i32:16776328) => i32:0
called host wasi_snapshot_preview1.fd_write(i32:2, i32:16776160, i32:2, i32:16776156) => i32:0
called host wasi_snapshot_preview1.fd_write(i32:2, i32:16776272, i32:2, i32:16776268) => i32:0
debug_test_analyzer_options() => error: unreachable executed
```

### Step 3: wasm-interp 出力の読み方

`--dummy-import-func` はホスト関数の呼び出しをログに出す。これがクラッシュ原因の手がかりになる。

**fd_write のファイルディスクリプタ**:
- `fd_write(i32:1, ...)` — **stdout** への書き込み。通常の出力
- `fd_write(i32:2, ...)` — **stderr** への書き込み。**エラーログ**

クラッシュ前に `fd_write(fd=2)` が複数回呼ばれていたら、C++ 側で CHECK 失敗や LOG(FATAL) のメッセージを stderr に書いてから abort() している。

**clock_time_get → fd_write(fd=2) → unreachable** のパターン：
1. `clock_time_get` — ログのタイムスタンプ取得
2. `fd_write(fd=2)` — エラーメッセージを stderr に出力
3. `unreachable` — abort() 実行

このパターンは abseil の `CHECK()` / `LOG(FATAL)` 失敗の典型。`--dummy-import-func` では fd_write の内容（実際のエラー文字列）は見えないが、パターンから原因の種類を推測できる。

**fd_write なしの即 unreachable**：
ログ出力なしにいきなりクラッシュする場合は、C++ 例外 (`throw`) が Emscripten の例外無効設定により `abort()` に変換されている可能性が高い。

### Step 4: 段階的な切り分け

複数のエクスポート関数がある場合、簡単な関数から順に実行して最初に失敗する関数を見つける：

```bash
wasm-interp target.wasm --dummy-import-func -r "func_a"  # OK
wasm-interp target.wasm --dummy-import-func -r "func_b"  # OK
wasm-interp target.wasm --dummy-import-func -r "func_c"  # => error: unreachable
```

func_c で初めてクラッシュするなら、func_b にはない func_c 固有の処理が原因。
C++ 側に粒度の細かいデバッグ関数を追加し、どの処理ステップでクラッシュするか二分探索で絞り込む。

### Step 5: Name セクションの確認

```bash
wasm-objdump -x target.wasm -j Name
```

デバッグシンボルがあれば内部関数名が見える。最適化ビルドでは空のことが多い。
空の場合は Step 1 の Export セクションと Step 4 の段階的テストで補う。

### Step 6: コードレベルの調査

関数の中身を見る必要がある場合：

```bash
# 特定関数のディスアセンブリ
wasm-objdump -d target.wasm | grep -A 30 "func\[41434\]"

# 高レベルデコンパイル（最も読みやすい）
wasm-decompile target.wasm -o target.dcmp
grep -A 30 "function 41434" target.dcmp

# テキスト形式 (WAT) — 正確だが冗長
wasm2wat target.wasm -o target.wat
```

大きい WASM（数十MB以上）のフルダンプは数GB になりうる。`grep` でピンポイントに絞ること。

### Step 7: 実行トレース（最終手段）

```bash
# 全命令のトレース — 非常に遅く出力も膨大
wasm-interp target.wasm --dummy-import-func -r "func_name" --trace 2>&1 | tail -100
```

クラッシュ直前の命令列が見える。最後の `unreachable` の直前に何をしていたかで原因が分かることがある。大きい関数では出力が膨大になるため、`tail` で末尾だけ見るのが現実的。

## unreachable の原因パターン

| パターン | wasm-interp での見え方 | 原因 |
|----------|----------------------|------|
| `clock_time_get` → `fd_write(fd=2)` × N → unreachable | CHECK/LOG(FATAL) | abseil CHECK 失敗。stderr にエラー条件が書かれている |
| `fd_write(fd=2)` → unreachable | アサーション | C/C++ assert() 失敗 |
| 即 unreachable（fd_write なし） | C++ 例外 | `throw` が `-fno-exceptions` で abort() に変換 |
| 深いコールスタック → unreachable | スタックオーバーフロー | `-sSTACK_SIZE` 不足 |
| malloc 呼び出し後 → unreachable | メモリ不足 | `-sINITIAL_MEMORY` / `-sMAXIMUM_MEMORY` 不足 |

## wasm-interp の便利オプション

```bash
# WASI モード（_start を自動呼出し）
wasm-interp target.wasm --wasi -r "func_name"

# verbose（モジュールロード詳細）
wasm-interp target.wasm --dummy-import-func -r "func_name" -v

# スタックサイズ変更
wasm-interp target.wasm --dummy-import-func -V 10000 -r "func_name"

# 全エクスポート関数を順に実行
wasm-interp target.wasm --dummy-import-func --run-all-exports

# 環境変数を渡す (WASI)
wasm-interp target.wasm --wasi -e "KEY=VALUE" -r "func_name"
```
