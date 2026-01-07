# ZetaSQL WASM SDK テスト計画

## 目的

ZetaSQL WASMビルドのGo SDKとして提供される機能が正しく動作することを検証する。

## 背景

### WASMビルドの特性

- クロスコンパイル環境のため、ICUツール（pkgdata、icupkg等）のビルドを無効化（`--disable-tools`）
- ICUデータは空のstubdata（約600B）となり、ロケールデータは含まれない
- 一部のICU依存機能に制限がある可能性がある

### テスト対象SDK

- **プラットフォーム**: Go SDK (wazeroランタイム使用)
- **公開予定**: github.com/glassmonkey/zetasql-wasm
- **提供機能**: ZetaSQLパーサー、アナライザー、各種SQL関数

## テスト対象機能

ZetaSQL WASM SDKが提供する主要機能を以下のカテゴリに分けて検証します。

### 1. SQL解析機能（Parser）

- **対象API**: `ParseStatement()`, `ParseExpression()`, `ParseType()`
- **実装ファイル**: `zetasql/parser/parser.cc`
- **ICU依存**: なし
- **期待動作**: 完全動作

### 2. 文字列関数（String Functions）

- **対象関数**: `UPPER()`, `LOWER()`, `LENGTH()`, `CONCAT()`, `SUBSTRING()`, `TRIM()`, `NORMALIZE()`
- **実装ファイル**: `zetasql/public/functions/string.cc`, `zetasql/common/unicode_utils.cc`
- **ICU依存**:
  - `UPPER()`, `LOWER()`: ロケール別変換に`icu::CaseMap`を使用
  - `NORMALIZE()`: Unicode正規化に`icu::Normalizer2`を使用
- **期待動作**:
  - ASCII文字: 完全動作
  - 非ASCII文字: デフォルトルールで動作、またはエラー

### 3. 照合順序（Collation）

- **対象機能**: `COLLATE`句を使用した文字列比較
- **実装ファイル**: `zetasql/public/collator.cc`
- **ICU依存**: ロケール別照合ルールに`icu::Collator`, `icu::RuleBasedCollator`を使用
- **期待動作**: エラー、またはデフォルト照合順序にフォールバック

### 4. 正規表現関数（Regexp Functions）

- **対象関数**: `REGEXP_CONTAINS()`, `REGEXP_EXTRACT()`, `REGEXP_REPLACE()`, `REGEXP_MATCH()`
- **実装ファイル**: `zetasql/public/functions/regexp.cc`
- **ICU依存**: Unicodeプロパティ（`\p{L}`, `\p{Nd}`等）に文字プロパティテーブルを使用
- **期待動作**:
  - ASCII範囲のパターン: 完全動作
  - Unicodeプロパティ: エラー、または誤マッチの可能性

### 5. UTF-8処理

- **対象機能**: 基本的な文字列操作、文字数カウント
- **実装ファイル**: `zetasql/common/utf_util.cc`
- **ICU依存**: `U8_*`マクロ（ヘッダーのみ、データ不要）
- **期待動作**: 完全動作

### 6. その他の機能

- **日時関数**: `DATE()`, `TIMESTAMP()`, `FORMAT_DATE()` 等
- **数値関数**: `ABS()`, `ROUND()`, `MOD()` 等
- **集約関数**: `SUM()`, `AVG()`, `COUNT()` 等
- **ICU依存**: 基本的になし
- **期待動作**: 完全動作

## テストケース

### 環境準備

```go
package zetasql_test

import (
    "context"
    "testing"

    "github.com/glassmonkey/zetasql-wasm"
)

func setupParser(t *testing.T) (*zetasql.Parser, context.Context) {
    ctx := context.Background()
    parser, err := zetasql.NewParser(ctx)
    if err != nil {
        t.Fatalf("Failed to create parser: %v", err)
    }
    t.Cleanup(func() {
        parser.Close(ctx)
    })
    return parser, ctx
}
```

### 1. SQL解析機能のテスト

#### 1.1 基本的なクエリ解析

```go
func TestBasicParsing(t *testing.T) {
    parser, ctx := setupParser(t)

    tests := []struct {
        name  string
        query string
    }{
        {"simple select", "SELECT 1"},
        {"where clause", "SELECT id, name FROM users WHERE age > 20"},
        {"join", "SELECT * FROM a JOIN b ON a.id = b.a_id"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            stmt, err := parser.ParseStatement(ctx, tt.query)
            if err != nil {
                t.Errorf("ParseStatement failed: %v", err)
            }
            if stmt == nil {
                t.Error("Expected non-nil statement")
            }
        })
    }
}
```

**期待結果:** ✅ すべて成功

**ICU依存:** なし（パーサーはICUに依存しない）

#### 1.2 UTF-8文字列の解析

```go
func TestUTF8StringHandling(t *testing.T) {
    parser, ctx := setupParser(t)

    tests := []struct {
        name  string
        query string
    }{
        {"japanese", "SELECT '日本語テスト'"},
        {"emoji", "SELECT '🔥 test'"},
        {"length", "SELECT LENGTH('こんにちは')"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            stmt, err := parser.ParseStatement(ctx, tt.query)
            if err != nil {
                t.Errorf("ParseStatement failed: %v", err)
            }
            if stmt == nil {
                t.Error("Expected non-nil statement")
            }
        })
    }
}
```

**期待結果:** ✅ すべて成功

**ICU依存:** なし（UTF-8処理は`U8_*`ヘッダーマクロのみ使用）

### 2. 文字列関数のテスト

#### 2.1 UPPER/LOWER関数

```go
func TestCaseConversion(t *testing.T) {
    parser, ctx := setupParser(t)

    tests := []struct {
        name        string
        query       string
        expectError bool
        note        string
    }{
        {
            name:        "ASCII upper",
            query:       "SELECT UPPER('hello')",
            expectError: false,
            note:        "ASCIIはロケールデータ不要",
        },
        {
            name:        "German eszett",
            query:       "SELECT UPPER('straße')", // ß → SS
            expectError: false,
            note:        "ICUデータがない場合、誤った結果の可能性",
        },
        {
            name:        "Turkish I",
            query:       "SELECT LOWER('İstanbul')", // İ → i (トルコ語)
            expectError: false,
            note:        "ICUデータがない場合、誤った結果の可能性",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            stmt, err := parser.ParseStatement(ctx, tt.query)

            if tt.expectError && err == nil {
                t.Error("Expected error but got nil")
            } else if !tt.expectError && err != nil {
                t.Errorf("Unexpected error: %v", err)
            } else {
                t.Logf("Result: stmt=%v, error=%v, note=%s", stmt, err, tt.note)
            }
        })
    }
}
```

**期待結果:**
- ✅ ASCII文字: 正常動作
- ⚠️ 非ASCII文字: デフォルトルールで変換（期待と異なる可能性）

**ICU依存:** あり（ロケール別変換に`icu::CaseMap`を使用）

**調査項目:**
- [ ] ドイツ語のß（エスツェット）の変換結果
- [ ] トルコ語のİ（ドット付きI）の変換結果
- [ ] デフォルトフォールバック動作の詳細

#### 2.2 NORMALIZE関数

```go
func TestNormalization(t *testing.T) {
    parser, ctx := setupParser(t)

    tests := []struct {
        name        string
        query       string
        expectError bool
    }{
        {
            name:        "NFC normalization",
            query:       "SELECT NORMALIZE('é', NFC)", // U+00E9
            expectError: true, // 正規化テーブルが必要
        },
        {
            name:        "NFD normalization",
            query:       "SELECT NORMALIZE('é', NFD)", // U+0065 U+0301
            expectError: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := parser.ParseStatement(ctx, tt.query)

            if tt.expectError {
                if err == nil {
                    t.Error("Expected error but got nil")
                } else {
                    t.Logf("Got expected error: %v", err)
                }
            } else {
                if err != nil {
                    t.Errorf("Unexpected error: %v", err)
                }
            }
        })
    }
}
```

**期待結果:**
- ❌ エラー: "Normalizer not available" または類似のメッセージ
- ⚠️ または入力をそのまま返す（正規化しない）

**ICU依存:** あり（Unicode正規化に`icu::Normalizer2`を使用）

### 3. 照合順序のテスト

#### 3.1 COLLATE句

```go
func TestCollation(t *testing.T) {
    parser, ctx := setupParser(t)

    tests := []struct {
        name        string
        query       string
        expectError bool
        errorMsg    string
    }{
        {
            name:        "en_US collation",
            query:       "SELECT 'a' < 'B' COLLATE 'en_US'",
            expectError: true, // ICUデータがないためエラーが予想される
            errorMsg:    "Collation 'en_US' not found",
        },
        {
            name:        "de_DE collation",
            query:       "SELECT 'ä' = 'a' COLLATE 'de_DE'",
            expectError: true,
            errorMsg:    "Collation 'de_DE' not found",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := parser.ParseStatement(ctx, tt.query)

            if tt.expectError {
                if err == nil {
                    t.Error("Expected error but got nil")
                } else {
                    t.Logf("Got expected error: %v", err)
                }
            } else {
                if err != nil {
                    t.Errorf("Unexpected error: %v", err)
                }
            }
        })
    }
}
```

**期待結果:**
- ❌ エラー: "Collation 'xx_XX' not found" または類似のメッセージ
- ⚠️ またはフォールバック: デフォルト照合順序で動作

**ICU依存:** あり（ロケール別照合ルールに`icu::Collator`, `icu::RuleBasedCollator`を使用）

**調査項目:**
- [ ] エラーメッセージの内容
- [ ] フォールバック動作の有無
- [ ] パフォーマンスへの影響

### 4. 正規表現関数のテスト

#### 4.1 Unicodeプロパティ

```go
func TestRegexpUnicodeProperties(t *testing.T) {
    parser, ctx := setupParser(t)

    tests := []struct {
        name        string
        query       string
        expectError bool
        note        string
    }{
        {
            name:        "ASCII regex",
            query:       `SELECT REGEXP_CONTAINS('test123', r'[a-z]+')`,
            expectError: false,
            note:        "ASCII範囲のみ、ICUデータ不要",
        },
        {
            name:        "Unicode letter property",
            query:       `SELECT REGEXP_CONTAINS('café', r'\p{L}+')`,
            expectError: false, // パース自体は成功する可能性
            note:        "\\p{L}（Unicode Letter）はICUデータが必要",
        },
        {
            name:        "Unicode digit property",
            query:       `SELECT REGEXP_CONTAINS('test123', r'\p{Nd}+')`,
            expectError: false,
            note:        "\\p{Nd}（Unicode Decimal Number）はICUデータが必要",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            stmt, err := parser.ParseStatement(ctx, tt.query)

            if tt.expectError && err == nil {
                t.Error("Expected error but got nil")
            } else if !tt.expectError && err != nil {
                t.Errorf("Unexpected error: %v", err)
            } else {
                t.Logf("Result: stmt=%v, error=%v, note=%s", stmt, err, tt.note)
            }
        })
    }
}
```

**期待結果:**
- ✅ ASCII範囲のパターン: 正常動作
- ❓ Unicodeプロパティ（`\p{}`）: パースは成功するが、実行時にエラーまたは誤マッチの可能性

**ICU依存:** あり（Unicodeプロパティに文字プロパティテーブルを使用）

### 5. エッジケーステスト

#### 5.1 空文字列

```go
func TestEmptyStrings(t *testing.T) {
    parser, ctx := setupParser(t)

    tests := []string{
        "SELECT UPPER('')",
        "SELECT LOWER('')",
        // COLLATE, NORMALIZEは実装次第でテスト追加
    }

    for _, query := range tests {
        t.Run(query, func(t *testing.T) {
            _, err := parser.ParseStatement(ctx, query)
            if err != nil {
                t.Errorf("Should handle empty string without error: %v", err)
            }
        })
    }
}
```

**期待結果:** ✅ エラーにならず、空文字列を返す

**ICU依存:** 関数により異なる

## テスト実行方法

### 前提条件
- WASMビルドが完了している（`wasm/zetasql.wasm`が存在）
- Go 1.21以上がインストールされている

### 実行手順

```bash
# 1. リポジトリのルートディレクトリに移動
cd /path/to/zetasql-wasm

# 2. テスト実行
go test -v ./... -run TestICU

# 3. カバレッジ付きで実行
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# 4. ベンチマーク実行
go test -bench=. -benchmem ./...
```

### ログ収集

```go
// テストファイル内でログ出力を追加
func TestWithLogging(t *testing.T) {
    // 詳細なログを有効化
    t.Logf("Testing ICU-dependent feature...")

    // 各テストで結果を記録
    results := make(map[string]interface{})

    // テスト終了時に結果をJSON出力
    t.Cleanup(func() {
        if data, err := json.MarshalIndent(results, "", "  "); err == nil {
            t.Logf("Test results: %s", string(data))
        }
    })
}
```

## 成功基準

### Phase 1: 基本機能の検証
- [ ] SQL解析機能が正常に動作する
- [ ] ASCII範囲の文字列処理が正常に動作する
- [ ] UTF-8エンコードされた文字列を正しく処理できる
- [ ] ICU依存機能が適切にエラーハンドリングされる（クラッシュしない）
- [ ] エラーメッセージが明確で理解しやすい

### Phase 2: ドキュメント化と最適化
- [ ] 各機能の動作状況をREADME.mdにドキュメント化
- [ ] ICU依存機能の制限事項と代替手段を明記
- [ ] パフォーマンスベンチマーク結果を公開
- [ ] 必要に応じてICUデータ提供方法を確立

## 機能別の動作状況

### WASM環境での動作予測

| カテゴリ | 機能 | 動作状況 | 制限事項 |
|---------|------|---------|---------|
| SQL解析 | Parser全般 | ✅ 完全動作 | なし |
| UTF-8処理 | 基本的な文字列操作 | ✅ 完全動作 | なし |
| 文字列関数 | UPPER/LOWER (ASCII) | ✅ 完全動作 | なし |
| 文字列関数 | UPPER/LOWER (非ASCII) | ⚠️ 部分動作 | ロケール別変換は不可、デフォルトルールで動作 |
| 文字列関数 | NORMALIZE() | ❌ 制限あり | ICUデータがないため、エラーまたは未サポート |
| 照合順序 | COLLATE句 | ❌ 制限あり | ロケール別照合は不可、バイナリ比較で代替 |
| 正規表現 | ASCII範囲 | ✅ 完全動作 | なし |
| 正規表現 | Unicodeプロパティ | ⚠️ 部分動作 | `\p{}`パターンは誤マッチの可能性 |

### WASMビルドの技術的制約

1. **ICU stubdata**
   - WASMビルドではICUデータが空のstubdata（約600B）
   - ネイティブビルドでは完全なICUデータ（約30MB）を使用
   - ロケール情報や正規化テーブルが含まれない

2. **クロスコンパイル制約**
   - `--disable-tools`によりICUツール（pkgdata、icupkg等）がビルドされない
   - ツールなしではICUデータを静的ライブラリにパッケージできない

## 今後の対応オプション

### オプション1: 現状維持（stubdata）
**採用条件:** ZetaSQLの使用がParser/Analyzerのみで、ICU依存機能を使わない場合

- ✅ メリット: シンプル、WASMサイズ小
- ❌ デメリット: 機能制限

**対応:**
- README.mdに制限事項を明記
- ICU依存機能のエラーハンドリングを強化

### オプション2: ICUデータの別途提供
**採用条件:** ICU依存機能が必要な場合

- ✅ メリット: 完全な機能
- ❌ デメリット: WASMサイズが約30MB増加

**実装方法:**
```go
// ICUデータを別途バンドル
//go:embed icudt75l.dat
var icuData []byte

// 実行時にロード
func init() {
    // wazeroのメモリにICUデータを書き込み
    // udata_setCommonData() を呼び出し
}
```

### オプション3: ICU依存を削減
**採用条件:** 長期的な解決策

- ✅ メリット: 根本的な解決
- ❌ デメリット: ZetaSQLのフォークが必要

**検討事項:**
- ZetaSQLのICU使用箇所を特定
- 代替実装の可能性を調査
- アップストリームへの提案

## テスト結果の記録

テスト実行後、以下の形式で結果を記録してください：

```markdown
## ZetaSQL WASM SDK テスト結果

**実行日:** YYYY-MM-DD

**環境:**
- OS: macOS/Linux/Windows
- Go version: 1.21.x
- WASM size: XX MB
- ICU stubdata size: 600B

**結果サマリー:**
- ✅ 成功: XX件
- ⚠️ 部分動作: XX件
- ❌ 失敗: XX件
- ⏭️ スキップ: XX件

**機能別の結果:**

### SQL解析機能
- ✅ 基本的なクエリ解析: 正常動作
- ✅ UTF-8文字列の解析: 正常動作

### 文字列関数
- ✅ UPPER/LOWER (ASCII): 正常動作
- ⚠️ UPPER/LOWER (非ASCII): デフォルトルールで動作
- ❌ NORMALIZE(): エラー "Normalizer not available"

### 照合順序
- ❌ COLLATE句: エラー "Collation 'en_US' not found"

### 正規表現関数
- ✅ ASCII範囲: 正常動作
- ⚠️ Unicodeプロパティ: パースは成功、実行時の動作は要検証

**備考:**
- ICU依存機能の制限はドキュメント化済み
- 代替手段の提案: ...
```

## 参考情報

### ドキュメント
- [patches.md](./patches.md) - WASMビルド用パッチの詳細説明
- [architecture.md](./architecture.md) - プロジェクト全体のアーキテクチャ設計

### 実装詳細
- [ICU stubdata implementation](../tmp/icu-source/icu4c/source/stubdata/stubdata.cpp) - ICU stubdataの実装
- [ZetaSQL Collator](../tmp/zetasql/zetasql/public/collator.cc) - ZetaSQLの照合順序実装
- [ZetaSQL String Functions](../tmp/zetasql/zetasql/public/functions/string.cc) - 文字列関数の実装
- [ZetaSQL Regexp Functions](../tmp/zetasql/zetasql/public/functions/regexp.cc) - 正規表現関数の実装

### 外部リソース
- [wazero documentation](https://wazero.io/) - Go用WASMランタイム
- [ZetaSQL Documentation](https://github.com/google/zetasql) - ZetaSQL公式ドキュメント
- [ICU Data Management](https://unicode-org.github.io/icu/userguide/icu_data/) - ICUデータ管理ガイド