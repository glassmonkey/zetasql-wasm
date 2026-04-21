# zetasql-wasm 開発計画: Analyzer機能の追加実装

## 目的

go-zetasqlite が依存する go-zetasql（CGO必須）を zetasql-wasm（pure Go/wazero）で完全に置き換えるため、Parser以外の機能（Analyzer, Catalog, Type System, Resolved AST）をWASM経由で公開する。

## 設計方針: Protobuf Request/Response パターン

### 核心的な発見

ZetaSQLには既に `local_service.proto` で定義された **AnalyzeRequest/AnalyzeResponse プロトコル**が存在する。このプロトコルでは：

- **リクエスト**: `AnalyzerOptionsProto` + `SimpleCatalogProto` + SQL文字列
- **レスポンス**: `AnyResolvedStatementProto`（シリアライズ済みResolved AST）

これはWASMバウンダリの設計問題（カタログのコールバック問題）を根本的に解決する。Go側でカタログ情報を `SimpleCatalogProto` にシリアライズし、WASM内のC++側でデシリアライズして `SimpleCatalog` を再構築すれば、**WASM↔Goの双方向呼び出しは不要**になる。

```
Go側                          WASM側（C++）
──────                        ──────────────
SimpleCatalog構築               
  ↓                           
SimpleCatalogProto にシリアライズ    
  ↓                           
AnalyzerOptionsProto 構築       
  ↓                           
Proto bytes + SQL → ────────→ デシリアライズ → SimpleCatalog復元
                              AnalyzerOptions復元
                              AnalyzeStatement() 実行
                              Resolved AST → Proto シリアライズ
Proto bytes ← ──────────────← AnyResolvedStatementProto
  ↓
Go側で Resolved AST ノード再構築
```

### 現行Parserとの対比

| | Parser（実装済み） | Analyzer（これから） |
|---|---|---|
| WASM入力 | SQL文字列 | Proto bytes (Catalog + Options + SQL) |
| WASM出力 | `AnyASTStatementProto` bytes | `AnyResolvedStatementProto` bytes |
| コールバック | なし | なし（カタログをProtoで事前注入） |
| Go側の型 | `ast.StatementNode` (497種) | `resolved_ast.Node` (200+種) |

---

## フェーズ構成

### Phase 0: 基盤整備（Resolved AST Proto生成 + 型システム）

**目標**: Resolved ASTのProtobufシリアライズ/デシリアライズ基盤と、Go側の型システムを整備する。

#### 0-1. Resolved AST Protoの Go コード生成確認

既に `wasm/schemas/zetasql/resolved_ast/` にprotoスキーマがあり、`wasm/generated/` にGoコードが生成されている可能性がある。生成済みなら確認のみ、未生成なら `protogen` ツールに追加。

**成果物**:
- `wasm/generated/` に `resolved_ast.pb.go`, `serialization.pb.go` 等が利用可能な状態

#### 0-2. Go側 型システム（`types` パッケージ）の実装

go-zetasqliteが使う型操作APIをpure Goで実装する。Proto定義（`type.proto`）のGoバインディングは既にあるので、それをラップする形で実装。

```go
package types

type TypeKind int32  // type.proto の TypeKind をマッピング

type Type interface {
    Kind() TypeKind
    IsArray() bool
    IsStruct() bool
    AsArray() *ArrayType
    AsStruct() *StructType
    ToProto() *TypeProto  // シリアライズ用
}

// スカラー型（INT64, STRING, BOOL, ...）
func Int64Type() Type
func StringType() Type
// ...23種

// 複合型
func NewArrayType(elemType Type) (*ArrayType, error)
func NewStructType(fields []*StructField) (*StructType, error)
```

**成果物**:
- `types/` パッケージ: `TypeKind`, `Type` interface, 23種のスカラー型, `ArrayType`, `StructType`
- `TypeProto` との相互変換

#### 0-3. Go側 カタログ型の実装

`SimpleCatalogProto` を構築するためのGo APIを実装。

```go
package catalog

type SimpleCatalog struct { /* ... */ }

func NewSimpleCatalog(name string) *SimpleCatalog
func (c *SimpleCatalog) AddTable(t *SimpleTable)
func (c *SimpleCatalog) AddBuiltinFunctions()  // フラグ設定のみ
func (c *SimpleCatalog) ToProto() *SimpleCatalogProto

type SimpleTable struct { /* ... */ }
type SimpleColumn struct { /* ... */ }

func NewSimpleTable(name string, columns []*SimpleColumn) *SimpleTable
func NewSimpleColumn(tableName, name string, typ types.Type) *SimpleColumn
```

**成果物**:
- `catalog/` パッケージ: `SimpleCatalog`, `SimpleTable`, `SimpleColumn`
- `SimpleCatalogProto` へのシリアライズ

---

### Phase 1: Analyzer WASMブリッジの実装

**目標**: C++側のWASMエクスポート関数を実装し、Go側から呼び出せるようにする。

#### 1-1. C++ bridge関数の実装（`bridge.cc`）

```cpp
// 新規エクスポート関数
// request_ptr: AnalyzeRequest proto bytes へのポインタ
// request_size: バイト数
// 戻り値: [uint32 size][AnalyzeResponse proto bytes] へのポインタ
EMSCRIPTEN_KEEPALIVE
void* analyze_statement_proto(const void* request_ptr, uint32_t request_size);
```

C++側の処理フロー:
1. `request_ptr` から `AnalyzeRequest` proto をデシリアライズ
2. `SimpleCatalogProto` → `SimpleCatalog` 復元（`SimpleCatalog::Deserialize`）
3. `AnalyzerOptionsProto` → `AnalyzerOptions` 復元
4. `zetasql::AnalyzeStatement()` 実行
5. Resolved AST → `AnyResolvedStatementProto` シリアライズ
6. レスポンスbytesを返却

**注意点**:
- `SimpleCatalog::Deserialize` は ZetaSQL に既存のメソッド（要確認）
- ビルトイン関数の登録は `ZetaSQLBuiltinFunctionOptionsProto` 経由
- `AnalyzerOptions` も `AnalyzerOptionsProto` からの復元メソッドがある（要確認）

#### 1-2. Go側 Analyzer APIの実装

```go
// analyzer.go
type Analyzer struct {
    parser *Parser  // 既存のWASMランタイムを共有
}

type AnalyzerOptions struct {
    LanguageOptions LanguageOptions
    Parameters      map[string]types.Type
    // ...
}

type AnalyzeOutput struct {
    statement resolved_ast.StatementNode
}

func (a *Analyzer) AnalyzeStatement(
    ctx context.Context,
    sql string,
    catalog *catalog.SimpleCatalog,
    opts *AnalyzerOptions,
) (*AnalyzeOutput, error)
```

内部処理:
1. `catalog.ToProto()` → `SimpleCatalogProto`
2. `opts.ToProto()` → `AnalyzerOptionsProto`
3. `AnalyzeRequest` Proto構築・シリアライズ
4. WASMメモリにbytesをコピー
5. `analyze_statement_proto()` WASM関数呼び出し
6. レスポンスbytesを読み取り
7. `AnalyzeResponse` デシリアライズ → Go側のResolved ASTノードに変換

**成果物**:
- `bridge.cc` に `analyze_statement_proto` 実装
- `analyzer.go` に `Analyzer` 型と `AnalyzeStatement` メソッド
- WASMバイナリの再ビルド

#### 1-3. 基本テスト

```go
func TestAnalyzeSimpleSelect(t *testing.T) {
    catalog := catalog.NewSimpleCatalog("test")
    table := catalog.NewSimpleTable("users", []*SimpleColumn{
        catalog.NewSimpleColumn("users", "id", types.Int64Type()),
        catalog.NewSimpleColumn("users", "name", types.StringType()),
    })
    catalog.AddTable(table)
    catalog.AddBuiltinFunctions()

    analyzer := NewAnalyzer(parser)
    output, err := analyzer.AnalyzeStatement(ctx, "SELECT id, name FROM users", catalog, defaultOpts)
    // → output.Statement() は ResolvedQueryStmt
}
```

---

### Phase 2: Resolved AST ノードシステム

**目標**: Go側でResolved ASTのノードを型安全に操作できるようにする。

#### 2-1. Resolved AST ノード型の自動生成

`astgen` ツールを拡張し、`resolved_ast.proto` からGoノードラッパーを生成する。parsed AST（497種）と同様のアプローチ。

```go
package resolved_ast

type Node interface {
    Kind() Kind
    // ...
}

type StatementNode interface { Node }
type ExprNode interface { Node }
type ScanNode interface { Node }

// 具象ノード例
type QueryStmt struct { /* proto fields */ }
func (n *QueryStmt) OutputColumnList() []*OutputColumn
func (n *QueryStmt) Query() ScanNode

type TableScan struct { /* proto fields */ }
func (n *TableScan) Table() *TableRefProto
func (n *TableScan) ColumnList() []*ResolvedColumnProto
```

**生成戦略**:
- `resolved_ast.proto` の `AnyResolvedNodeProto` の oneof 構造をパースして全ノード型を列挙
- 各 `message` の fields からアクセサメソッドを生成
- `serialization.proto` の `ResolvedColumnProto` 等の補助型もラップ

**成果物**:
- `resolved_ast/` パッケージ: 200+ノード型の自動生成コード
- `Walk()` 関数（ノード走査用）
- `AnyResolvedStatementProto` → Go ノードへの変換関数

#### 2-2. AnalyzeOutputの型安全化

Phase 1の `AnalyzeOutput` を拡張し、型安全なResolved ASTノードを返すようにする。

---

### Phase 3: go-zetasqlite互換API

**目標**: go-zetasqliteからの移行を最小限の変更で可能にする。

#### 3-1. 関数・シグネチャAPI

```go
package types

type Function struct { /* ... */ }
type FunctionSignature struct { /* ... */ }
type FunctionArgumentType struct { /* ... */ }

func NewFunction(name string, group string, mode Mode, sigs []*FunctionSignature) *Function
func NewFunctionSignature(ret *FunctionArgumentType, args []*FunctionArgumentType, contextID int64) *FunctionSignature
func NewFunctionArgumentType(typ Type, options *FunctionArgumentTypeOptions) *FunctionArgumentType
func NewTemplatedFunctionArgumentType(kind SignatureArgumentKind, options *FunctionArgumentTypeOptions) *FunctionArgumentType
```

#### 3-2. AnalyzerOptions / LanguageOptions の完全実装

go-zetasqliteが使う39の言語フラグ、16のステートメント種別を全てサポート。

```go
type LanguageOptions struct { /* ... */ }

func (o *LanguageOptions) EnableLanguageFeature(f LanguageFeature)
func (o *LanguageOptions) SetSupportedStatementKinds(kinds []StatementKind)
func (o *LanguageOptions) EnableReservableKeyword(keyword string, enable bool)
func (o *LanguageOptions) ToProto() *LanguageOptionsProto
```

#### 3-3. マルチステートメントパース

```go
type ParseResumeLocation struct { /* ... */ }

func NewParseResumeLocation(input string) *ParseResumeLocation
func (p *Parser) ParseNextScriptStatement(ctx context.Context, loc *ParseResumeLocation, opts *ParserOptions) (ast.StatementNode, bool, error)
```

C++側に `parse_next_script_statement_proto` を追加するか、既存の `ParseResumeLocationProto` を活用。

#### 3-4. NodeMap（parsed↔resolved ASTマッピング）

```go
type NodeMap struct { /* ... */ }

func NewNodeMap(resolvedNode resolved_ast.Node, parsedNode ast.StatementNode) *NodeMap
func (m *NodeMap) FindNodeFromResolvedNode(n resolved_ast.Node) (ast.Node, bool)
```

**実装方針**: `ParseLocationRecordType` を `FULL_NODE_SCOPE` に設定してAnalyzerを実行し、Resolved ASTの各ノードに含まれる `parse_location_range` を使ってparsed ASTノードとマッチングする。WASM側で処理する必要はなく、Go側のみで実装可能。

---

## マイルストーン

### M0: 基盤整備（Phase 0）
- [ ] Resolved AST proto生成の確認
- [ ] `types` パッケージの基本実装（TypeKind, Type interface, スカラー型, Array, Struct）
- [ ] `catalog` パッケージの基本実装（SimpleCatalog, SimpleTable, SimpleColumn）
- [ ] Proto相互変換のユニットテスト

### M1: 最初のAnalyze成功（Phase 1）
- [ ] C++ bridge: `analyze_statement_proto` の実装
- [ ] WASMバイナリの再ビルド
- [ ] Go側: `Analyzer.AnalyzeStatement` の実装
- [ ] `SELECT 1` の意味解析が通ること
- [ ] `SELECT col FROM table` がカタログ付きで解析できること

### M2: Resolved AST操作（Phase 2）
- [ ] `resolved_ast` パッケージのコード生成ツール拡張
- [ ] 200+ノード型の自動生成
- [ ] Walk関数の実装
- [ ] go-zetasqliteで使われる主要ノード（QueryStmt, TableScan, JoinScan, FunctionCall等）のテスト

### M3: go-zetasqlite互換（Phase 3）
- [ ] Function / FunctionSignature API
- [ ] LanguageOptions 全フラグ対応
- [ ] マルチステートメントパース
- [ ] NodeMap実装
- [ ] go-zetasqliteのテストスイートが通ること

### M4: go-zetasqlite統合
- [ ] go-zetasqliteの `go.mod` を zetasql-wasm に切り替え
- [ ] インポートパスの変更
- [ ] go-zetasqliteの全テスト通過
- [ ] パフォーマンスベンチマーク（CGO版との比較）

---

## リスクと対策

### R1: SimpleCatalog のデシリアライズ

**リスク**: C++側で `SimpleCatalogProto` → `SimpleCatalog` の復元がZetaSQLの内部APIに依存する可能性。

**対策**: ZetaSQLの `local_service` 実装（`local_service_impl.cc`）を参考にする。このサービスはまさにProto経由でカタログを受け取りAnalyzerに渡しているため、同じパターンが使える。

### R2: WASMバイナリサイズの増大

**リスク**: Analyzer関連のコードをリンクすることで、現在21.4MBのWASMバイナリがさらに大きくなる可能性。

**対策**: 
- ビルトイン関数の登録はオンデマンドにする（`ZetaSQLBuiltinFunctionOptionsProto` で制御）
- Emscriptenの最適化フラグ（`-Oz`）の確認
- 現状のdebug関数（`debug_test_analyze`等）で既にAnalyzerがリンクされているため、追加の増大は限定的と推測

### R3: Resolved ASTのProtoシリアライズの完全性

**リスク**: ZetaSQLのResolved ASTシリアライズが全ノードタイプをカバーしているか不確実。

**対策**: ZetaSQLの `local_service` が本番で使っている機能なので、主要ノードは確実にカバーされている。未対応のエッジケースは発見次第パッチを追加。

### R4: パフォーマンス

**リスク**: Proto serialize/deserialize のオーバーヘッドが大きい可能性。特にカタログが大きい場合。

**対策**:
- カタログのキャッシュ: 同じカタログで複数回分析する場合、WASM側にカタログを保持する仕組みを後から追加可能（`RegisterCatalog` パターン）
- 初期実装ではシンプルなリクエスト/レスポンスで進め、ボトルネックが見えたら最適化

---

## 技術的な補足

### ZetaSQLが持つシリアライズ基盤

調査の結果、ZetaSQLは以下のシリアライズ/デシリアライズ機能を内蔵している：

| 対象 | Proto型 | シリアライズ | デシリアライズ |
|------|---------|-------------|---------------|
| Parse Tree AST | `AnyASTStatementProto` | `ParseTreeSerializer::Serialize` | `ParseTreeSerializer::Deserialize` |
| Resolved AST | `AnyResolvedStatementProto` | AnalyzerOutput内部 | local_service内部 |
| Catalog | `SimpleCatalogProto` | `SimpleCatalog::Serialize` | local_service内部 |
| AnalyzerOptions | `AnalyzerOptionsProto` | 各フィールドの手動設定 | local_service内部 |
| Type | `TypeProto` | `Type::SerializeToSelfContainedProto` | `TypeDeserializer` |

これらは全て `local_service` の実装で実際に使われているため、WASMブリッジでの利用は十分に実績がある。

### bridge.cc の実装参考

ZetaSQLの `zetasql/local_service/local_service_impl.cc` がまさに同じことをRPC経由でやっている。この実装を参考にすれば：

1. `AnalyzeRequest` → `SimpleCatalog` 復元 → `AnalyzeStatement` → `AnalyzeResponse` の全フローのC++コードが得られる
2. `DescriptorPool` のハンドリングも含めた正しいデシリアライズ手順がわかる
3. ビルトイン関数の登録タイミングも明確

### Go側のコード生成戦略

| 対象 | 既存ツール | 拡張方針 |
|------|-----------|---------|
| Parse Tree AST（497種） | `astgen` | そのまま利用 |
| Resolved AST（200+種） | なし | `astgen` を拡張するか、新ツール `resolved_astgen` を作成 |
| 型システム | なし | `type.proto` の `TypeKind` から手動実装（種類が限定的） |
| カタログ | なし | `simple_catalog.proto` から手動実装 |
