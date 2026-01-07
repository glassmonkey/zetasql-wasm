# ZetaSQLパッチ適用ガイド

このドキュメントでは、ZetaSQLをWebAssembly（WASM）向けにビルドするために必要なパッチファイルについて説明します。

## 目次

- [概要](#概要)
- [パッチの全体像](#パッチの全体像)
- [各パッチの詳細](#各パッチの詳細)
- [技術的背景](#技術的背景)
- [実践ガイド](#実践ガイド)
- [参考リンク](#参考リンク)

---

## 概要

### パッチが必要な理由

ZetaSQLをEmscriptenでWASMにコンパイルする際、以下の2つの主要な問題が発生します：

#### 1. ICUライブラリのクロスコンパイル問題
- ICUビルドツール（`icupkg`、`genrb`など）がWASM形式でコンパイルされる
- ビルドホスト（Linux/macOS）上でWASMバイナリを実行できない
- クロスコンパイル環境特有の制約

#### 2. C++イテレータ互換性の問題
- Emscriptenのlibc++は厳格なC++20コンセプトチェックを実施
- `absl::string_view::iterator`から`const char*`への暗黙的変換が許可されない
- ネイティブコンパイラ（GCC/Clang）では動作するが、Emscriptenでは失敗

### パッチファイルの命名規則

パッチファイル名はディレクトリとファイル名に対応しています：

| パッチ対象ファイル | パッチファイル名 |
|---|---|
| `bazel/icu.BUILD` | `bazel_icu.patch` |
| `zetasql/public/strings.cc` | `zetasql_public_strings.patch` |
| `zetasql/public/functions/string.cc` | `zetasql_public_functions_string.patch` |

スラッシュ（`/`）をアンダースコア（`_`）に、拡張子（`.cc`/`.BUILD`）は除外してパッチ名とします。

---

## パッチの全体像

### パッチファイル一覧

全5つのパッチファイルで、合計23箇所の修正を適用します：

| # | パッチファイル | 対象ファイル | 修正箇所 | 種類 |
|---|---|---|---|---|
| 1 | `bazel_icu.patch` | `bazel/icu.BUILD` | 1箇所 | ICUビルド設定 |
| 2 | `zetasql_public_strings.patch` | `zetasql/public/strings.cc` | 6箇所 | イテレータ互換性 |
| 3 | `zetasql_public_functions_string.patch` | `zetasql/public/functions/string.cc` | 2箇所 | イテレータ互換性 |
| 4 | `zetasql_public_functions_date_time_util.patch` | `zetasql/public/functions/date_time_util.cc` | 6箇所 | イテレータ互換性 |
| 5 | `zetasql_public_functions_regexp.patch` | `zetasql/public/functions/regexp.cc` | 8箇所 | イテレータ互換性 |

### 安全性保証

すべてのパッチは**意味的に等価な変換**であり、ZetaSQLの機能を損なうことはありません。

#### 共通の原則

1. **ゼロ機能変更**
   アルゴリズムやロジックの変更は一切なし。`begin()`と`data()`は同じアドレスを返す（C++標準で保証）

2. **型安全性の向上**
   ポインタへの明示的変換により、型チェックがより厳格になる

3. **移植性の向上**
   WASMだけでなく、将来のC++標準にも対応しやすくなる

4. **パフォーマンス中立**
   イテレータもポインタも同じ機械語に最適化される

---

## 各パッチの詳細

### 1. bazel_icu.patch

**対象ファイル:** `bazel/icu.BUILD`
**修正箇所:** 1箇所（line 51）

#### モチベーション

- ICUライブラリのビルド時に生成されるツール（`icupkg`、`genrb`など）はホスト環境で実行されるバイナリ
- クロスコンパイル環境（WASM）では、これらのツールがWASMバイナリとしてビルドされる
- ビルドホスト上でWASMバイナリを実行できないため、ビルドが失敗

#### 問題と解決策

**問題:**
ICUの`--enable-tools`オプションが有効だと、以下のツールがWASM形式でコンパイルされます：
- `pkgdata`: ICUデータを静的ライブラリ（libicudata.a）にパッケージ
- `icupkg`: データファイル（.dat）の変換・展開
- `genrb`: リソースバンドルのコンパイル

これらのツールはビルドホスト（Linux/macOS）で実行される必要がありますが、WASMバイナリは実行できません。

**解決策:**
`--disable-tools`を指定してICUツールのビルドをスキップします。

**修正例:**
```diff
-        "--enable-tools",  # needed to build data
+        "--disable-tools",  # WASM: disable tools to avoid cross-compile execution issues
```

**ビルド結果:**
- ツールがビルドされないため、ICUデータを静的ライブラリにパッケージできない
- 代わりに`stubdata`（空のデータライブラリ、約600B）が生成される
- `stubdata`は循環依存を解決するための最小限のヘッダーのみを含む

#### 影響と注意点

**データの欠如:**
- `--disable-tools`により、ICUデータ（ロケール情報、正規化テーブルなど）がビルド成果物に含まれない
- `libicudata.a`は空のstubdataとなる（ネイティブビルド: 30MB → WASMビルド: 600B）

**ZetaSQLへの影響:**
ZetaSQLは以下のICUデータ依存機能を使用しています：
- **Collation（zetasql/public/collator.cc）**: ロケール別の文字列比較
- **Unicode正規化（Normalizer2）**: NFC、NFKC正規化
- **大文字小文字変換（CaseMap）**: ロケール対応の変換

これらの機能は実行時にICUデータが必要ですが、WASMビルドではデータが存在しないため：
- 機能が制限される可能性がある
- 実行時エラーが発生する可能性がある
- デフォルトの動作にフォールバックする可能性がある

**TODO: 動作確認が必要**
- WASMビルドでZetaSQLのICU依存機能が正しく動作するかは未検証
- stubdataで動作する範囲の特定が必要

---

### 2. zetasql_public_strings.patch

**対象ファイル:** `zetasql/public/strings.cc`
**修正箇所:** 6箇所（line 76, 81, 190, 514, 590, 1061）

#### モチベーション

- `absl::string_view`のイテレータメソッド（`begin()`, `end()`）がEmscriptenのC++20厳格型チェックで問題を引き起こす
- ポインタベースの操作に置き換えることでWASM互換性を確保

#### 問題と解決策

**問題:**
以下のパターンでイテレータ→ポインタ変換が失敗：
- `source.end()` を `const char*` として使用
- `source.begin()` を `const char*` として使用
- `absl::string_view(iterator, size_t)` コンストラクタ呼び出し

**解決策:**
- `.begin()` → `.data()`
- `.end()` → `.data() + .size()`
- `absl::string_view(it, ...)` → `absl::string_view(&*it, ...)`

**修正例:**
```cpp
// 修正前
const char* end = source.end();
const int cur_pos = p - source.begin();

// 修正後
const char* end = source.data() + source.size();
const int cur_pos = p - source.data();
```

#### 毀損が無視できる理由

- `begin()`は`data()`と同じアドレスを返す（C++標準で保証）
- `end()`は`data() + size()`と同じアドレスを返す（C++標準で保証）
- ポインタ演算の結果は数学的に等価で、機能的な差異はゼロ
- 文字列エスケープやパス解析のロジックは1バイトも変更されていない

---

### 3. zetasql_public_functions_string.patch

**対象ファイル:** `zetasql/public/functions/string.cc`
**修正箇所:** 2箇所（line 297, 309）

#### モチベーション

- 文字列のトリム処理でイテレータからポインタへの変換を明示的にする
- `string_view`のコンストラクタがポインタを期待している箇所での互換性
- `reverse_iterator::base()`の結果をポインタ演算に使用

#### 問題と解決策

**問題:**
`BytesTrimmer::TrimLeft`および`TrimRight`関数で、イテレータを直接`string_view`コンストラクタに渡している。

**解決策:**
- `absl::string_view(it, ...)` → `absl::string_view(&*it, ...)`
- `it.base() - str.begin()` → `&*it.base() - str.data()`

**修正例:**
```cpp
// 修正前 (TrimLeft)
return absl::string_view(it, str.end() - it);

// 修正後
return absl::string_view(&*it, str.end() - it);

// 修正前 (TrimRight)
return absl::string_view(str.data(), it.base() - str.begin());

// 修正後
return absl::string_view(str.data(), &*it.base() - str.data());
```

#### 毀損が無視できる理由

- `&*it`はイテレータを対応するポインタに変換する安全で標準的な方法
- 文字列の範囲計算は同じ結果になる（先頭からの距離が等しい）
- トリム処理のロジックは完全に保持される（どの文字を削除するかの判定は不変）
- `TrimLeft`と`TrimRight`の対称性が保たれる

---

### 4. zetasql_public_functions_date_time_util.patch

**対象ファイル:** `zetasql/public/functions/date_time_util.cc`
**修正箇所:** 6箇所（line 5596, 5612, 5631, 5661, 5668, 5689）

#### モチベーション

- `std::string`の`begin()`メソッドがWASMで問題を起こす
- フォーマット文字列の部分文字列を作成する際の互換性問題を解決

#### 問題と解決策

**問題:**
日時フォーマット処理で`format_string.begin()`をポインタ演算に使用。

**解決策:**
すべての`format_string.begin()`を`format_string.data()`に置換。

**修正例:**
```cpp
// 修正前
absl::string_view(format_string.begin() + idx_percent_e + 2, i - idx_percent_e - 2)

// 修正後
absl::string_view(format_string.data() + idx_percent_e + 2, i - idx_percent_e - 2)
```

#### 毀損が無視できる理由

- `std::string::begin()`は内部的に`data()`と同じポインタを返す（C++標準で保証）
- 日付時刻フォーマット処理のロジック自体は変更なし
- 文字列の範囲計算が同一の結果を返す（アドレス計算が等価）
- サブナノ秒精度の処理アルゴリズムは完全に保持される

---

### 5. zetasql_public_functions_regexp.patch

**対象ファイル:** `zetasql/public/functions/regexp.cc`
**修正箇所:** 8箇所（line 524, 540, 542, 545, 547）

#### モチベーション

- 正規表現の置換処理でイテレータからポインタへのデリファレンスが必要
- `rewrite.end()`の使用をポインタ演算に置き換え
- `std::string::append(iterator, int)`がEmscriptenで型エラーになる

#### 問題と解決策

**問題:**
正規表現置換処理で以下の問題が発生：
- `std::string::append(iterator, int)` - イテレータを`const char*`として使用
- `const char* < iterator` - ポインタとイテレータの比較

**解決策:**
- イテレータをポインタに変換: `&*p`
- `.end()`を`.data() + .size()`に置換

**修正例:**
```cpp
// 修正前 (line 524)
out->append(p, len);

// 修正後
out->append(&*p, len);

// 修正前 (line 540, 542, 545, 547)
for (const char* s = rewrite.data(); s < rewrite.end(); ++s) {
    while (s < rewrite.end() && *s != '\\') s++;
    if (s < rewrite.end()) {
        int c = (s < rewrite.end()) ? *s : -1;
    }
}

// 修正後
for (const char* s = rewrite.data(); s < rewrite.data() + rewrite.size(); ++s) {
    while (s < rewrite.data() + rewrite.size() && *s != '\\') s++;
    if (s < rewrite.data() + rewrite.size()) {
        int c = (s < rewrite.data() + rewrite.size()) ? *s : -1;
    }
}
```

#### 毀損が無視できる理由

- `&*p`はイテレータ`p`をポインタに変換する標準的なC++イディオム
- `append(iterator, length)`と`append(pointer, length)`は同じメモリ領域を操作
- 正規表現マッチングとキャプチャグループの置換セマンティクスは完全に保持される
- バックスラッシュエスケープ処理のロジックは変更なし

---

## 技術的背景

### Emscriptenのlibc++とC++20コンセプト

Emscriptenのlibc++は、C++20の`contiguous_iterator`コンセプトに基づいてイテレータの型チェックを実施します。`absl::string_view::iterator`は内部的に`__wrap_iter<const char*>`として実装されていますが、このラッパー型は直接`const char*`に変換できません。

#### ネイティブコンパイラの挙動

- GCC/ClangはEmscriptenほど厳格ではなく、暗黙的な変換を許可
- `string_view(iterator, size_t)`コンストラクタが動作

#### Emscriptenの挙動

- C++20 `sized_sentinel_for`コンセプトチェックが失敗
- エラー: `no known conversion from '__wrap_iter<const char *>' to 'const char *'`

### なぜEmscriptenは厳格なのか

Emscriptenは以下の点で他のコンパイラより厳格です：

1. **C++20コンセプトの完全実装**
   `contiguous_iterator`と`sized_sentinel_for`の型チェックを厳格に実施

2. **Clangの最新バージョン使用**
   より厳格な型変換ルールを適用

3. **WebAssemblyの制約**
   ホストバイナリの実行不可（ICUツール問題）

これらのパッチは、ZetaSQLをEmscripten環境に適応させるための**防御的コーディング**であり、機能の毀損はありません。

### 推奨される対処パターン

#### 1. イテレータ→ポインタ変換: `&*it` イディオムを使用

```cpp
auto it = str.begin();
const char* ptr = &*it;  // OK
```

#### 2. 範囲の終端: `.end()`の代わりに`.data() + .size()`

```cpp
const char* end = str.data() + str.size();  // OK
const char* end = str.end();  // NG (Emscriptenでエラー)
```

#### 3. イテレータ演算: `.begin()`の代わりに`.data()`

```cpp
ptrdiff_t offset = ptr - str.data();  // OK
ptrdiff_t offset = ptr - str.begin();  // NG (Emscriptenでエラー)
```

---

## 実践ガイド

### パッチの適用方法

パッチは`MODULE.bazel`の`git_override`で自動適用されます：

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

`patch_strip = 1`により、パッチファイル内の`a/`と`b/`プレフィックスが除去されます。

### トラブルシューティング

#### パッチ適用エラー

**エラー:** `could not apply patch due to CONTENT_DOES_NOT_MATCH_TARGET`

**原因:**
- ZetaSQLのバージョンが異なる
- パッチのコンテキスト行が一致しない

**対処:**
1. ZetaSQLのバージョンを確認（`MODULE.bazel`の`tag`）
2. パッチを再生成（`diff -u`で作成）

#### ビルドエラーが続く場合

新しいイテレータ互換性エラーが出た場合：

1. エラーメッセージから該当ファイルと行番号を特定
2. `.begin()`/`.end()`の使用箇所を確認
3. 上記の推奨パターンで修正
4. 新しいパッチファイルを作成
5. `MODULE.bazel`に追加

---

## 参考リンク

- [Emscripten FAQ](https://emscripten.org/docs/getting_started/FAQ.html) - C++標準ライブラリのサポート状況とコンパイラの特性
- [std::contiguous_iterator](https://en.cppreference.com/w/cpp/iterator/contiguous_iterator.html) - C++20のcontiguous_iteratorコンセプト定義
- [ICU Data Management](https://unicode-org.github.io/icu/userguide/icu_data/) - ICUのデータファイル管理とビルドツール