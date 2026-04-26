---
name: bazel-build-expert
description: >
  Bazel + Emscripten + C++ WASM ビルドエラーの解析専門スキル。
  ビルドログ(wasm/build.log)を読み取り、エビデンスに基づいた解決策を提案する。
  ユーザーが「ビルドエラー」「build failed」「ビルドが通らない」「make buildが失敗」
  「Bazelエラー」「コンパイルエラー」「リンカーエラー」と言及した場合、
  またはwasm/build.logの解析を求められた場合にこのスキルを使うこと。
---

# Bazel Build Expert

Bazel + Emscripten + C++ ツールチェーンのビルドエラー解析を行う。
このプロジェクトは Docker 内で Bazel + Emscripten を使い ZetaSQL を WASM にコンパイルしている。

## 分析フロー

1. `wasm/build.log` からエラーメッセージを抽出（パスが異なる場合はユーザーに確認）
2. エラーの種類と影響範囲を特定
   - ターゲット用ビルド(Emscripten/WASM) か ホストツール用ビルド(GCC/clang x86_64) か
   - コンパイルエラー / リンカーエラー / Bazel設定エラー / ツールチェーンエラー / パッチエラー
   - `[for tool]` と `[for target]` を区別する
3. 既知の問題を公式リポジトリで検索（可能なら `gh api` を使う）
4. エビデンス付きで解決策を提示、またはエビデンス不足なら正直に伝える

## エビデンスの基準

有効なエビデンスは以下のみ：

- **公式ドキュメント引用**: Bazel / Emscripten / 該当ライブラリの公式ドキュメントからの直接引用（URL付き）
- **GitHub issue/PR**: bazelbuild/bazel, emscripten-core/emscripten 等の公式リポジトリ（issue番号 + URL + 引用）
- **ソースコード**: `.bazelrc`, ツールチェーン定義, ビルド設定の該当行（ファイルパス + 行番号）
- **コマンド実行結果**: `gh api` や CLI ツールの実際の出力

以下はエビデンスとして認めない：
- WebSearch結果のみ（ブログ, Stack Overflow は補助参考のみ）
- 「一般的に」「通常」などの曖昧な表現
- 確認できない報告や推測

## 引用形式

```
**出典**: [タイトル](URL)
> "引用文"
**ステータス**: Open/Closed（issue の場合）
```

## 報告フォーマット

### エビデンスがある場合

```markdown
## 問題の特定
[エラーの説明]

## 根本原因
[原因の説明]

**出典**: [リンク]
> "引用文"

## 解決策
[具体的な修正方法]
```

### エビデンスが不足する場合

```markdown
## 問題の特定
[エラーの説明]

## 調査結果
以下を調査しましたが、確実な解決策は見つかりませんでした：
1. [調査項目] - [結果]

## 提案（エビデンス不足）
以下を**試行**することを提案しますが、確実に動作するエビデンスはありません：
1. [アプローチ] - 根拠: [論理的推論] / リスク: [副作用] / 検証方法: [確認方法]
```

## よくある問題パターン

### パッチファイルエラー
```
Cannot find patch file: /home/builder/workspace/patches/xxx.patch
```
→ MODULE.bazel の patches リストと `wasm/assets/patches/` の実ファイルを照合

### 絶対パスインクルージョン
```
ERROR: absolute path inclusion(s) found in rule
```
→ `[for tool]` vs `[for target]` を確認、`--features=-layering_check` の適用範囲を調査

### コンパイラフラグエラー
```
clang: error: unknown argument: '-fxxx'
```
→ GCC専用 vs clang専用フラグの区別、`--copt` vs `--host_copt` の使い分け

### リンカーエラー
```
undefined reference to `xxx'
```
→ deps の欠落、Emscripten のシステムライブラリ設定を確認

### Bazel 依存解決エラー
```
no such target '@zetasql//zetasql/xxx:yyy'
```
→ ZetaSQL リポジトリの実際の BUILD ファイルでターゲット名を確認

## プロジェクト固有の情報

- ビルドは Docker 内で実行（環境隔離済み）
- `wasm/assets/bridge.cc` — C++ ブリッジコード
- `wasm/assets/BUILD.bazel` — Bazel ビルド定義
- `wasm/assets/MODULE.bazel` — 依存管理（ZetaSQL バージョン、パッチ）
- `wasm/assets/.bazelrc` — Bazel 設定フラグ
- `wasm/assets/patches/` — ZetaSQL 用パッチファイル
- `wasm/Makefile` — `make build` / `make rebuild` エントリポイント
- `wasm/script/build.sh` — Docker ビルドオーケストレーション

## 禁止事項

- 推測を断定として提示しない（「これで解決します」ではなく「試すことを提案します」）
- 未確認の issue/PR 内容を引用しない
- GCC 専用フラグを clang/Emscripten 向けに提案しない
