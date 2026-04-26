# xUnit Test Patterns 主要カタログ(Meszaros)

このリポジトリで Smell/Pattern 名を一語で会話するための辞書。出典は全て [xunitpatterns.com](http://xunitpatterns.com/)(Gerard Meszaros, *xUnit Test Patterns: Refactoring Test Code*, 2007)。各エントリは「問題」「処方」「URL」の3行が原則。サブ Smell は親エントリの中にぶら下げる。

このファイルは SKILL.md からの参照対象。レビュー中・テスト設計中に「これは何 Smell?」と迷ったら検索してね。

## 目次

- [Test Smells](#test-smells)
  - [Code Smells](#code-smells)
  - [Behavior Smells](#behavior-smells)
  - [Project Smells](#project-smells)
- [Test Patterns](#test-patterns)
  - [xUnit Basics](#xunit-basics)
  - [Test Strategy](#test-strategy)
  - [Fixture Setup](#fixture-setup)
  - [Fixture Teardown](#fixture-teardown)
  - [Result Verification](#result-verification)
  - [Test Doubles](#test-doubles)
  - [Test Organization](#test-organization)
  - [Value Patterns](#value-patterns)
- [Test Refactorings](#test-refactorings)

---

# Test Smells

テストにまつわる「臭い」のカタログ。3 階層に分かれる。

## Code Smells

テストコードを読んでいるときに気づく臭い。

### Obscure Test
- **問題**: テストを一目で理解できない。読み手が SUT の挙動を読み取れない。
- **処方**: 不要なコードを除去し、Test Utility Method / Creation Method / Custom Assertion で意図を抽象化する。Four Phase Test のフェーズ境界を明確にする。
- **出典**: <http://xunitpatterns.com/Obscure%20Test.html>

Obscure Test には以下のサブ Smell がある(同ページ内の anchor に整理されている):

- **Eager Test**: 1 つの Test Method で複数の機能を検証していて、どこが setup でどこが exercise か不明瞭。→ 機能ごとに Test Method を分ける(Single-Condition Test)。
- **Mystery Guest**: テストの fixture が外部リソース(ファイル、DB レコード等)に依存していて、テスト本体だけ読んでも何が前提か分からない。→ Inline Resource / Setup External Resource で fixture をテスト内に持ち込む。
- **General Fixture**: 共有 fixture が大きくて、各テストが何に依存しているか不明。→ Minimal Fixture / Fresh Fixture を使う。
- **Hard-Coded Test Data**: 期待値が魔法の数値で、なぜその値かの意味が読めない。→ Derived Value / 定数化で意図を表現する。
- **Verbose Test**: テストが長すぎて読み通せない。→ Test Utility Method で抽象化、Parameterized Test で重複削減。
- **Indirect Testing**: 別の SUT を経由して目的の SUT を検証していて、結果が間接的にしか観察できない。→ 目的の SUT を直接 exercise する。
- **Irrelevant Information**: 検証に関係ない値まで Arrange/Assert に書かれていて、何が肝か分からない。→ Anonymous Creation や `_` で意図しない値を非表示化。

### Conditional Test Logic
- **問題**: テスト中に `if`/`switch`/loop で条件分岐があり、ある実行ではアサートされる、別の実行では assert がスキップされる、という状態になっている。
- **処方**: Guard Assertion で前提崩壊を即時失敗にする。条件で実行を変えるなら Test Method を分ける(Single-Condition Test)。
- **出典**: <http://xunitpatterns.com/Conditional%20Test%20Logic.html>

### Hard to Test Code
- **問題**: 本番コードがテストしにくい構造(密結合、隠れた依存、非決定的)になっている。テストを書くこと自体が苦痛。
- **処方**: Extract Testable Component / Replace Dependency with Test Double / Humble Object 等の Refactoring で SUT を切り出す。
- **出典**: <http://xunitpatterns.com/Hard%20to%20Test%20Code.html>

### Test Code Duplication
- **問題**: 同じテストコードが多数のテストで繰り返されている。fixture 構築や検証ロジックが copy-paste で蔓延。
- **処方**: Test Utility Method / Creation Method / Parameterized Test で共通化(ただし helper 自体は trivial に保つ)。
- **出典**: <http://xunitpatterns.com/Test%20Code%20Duplication.html>

### Test Logic in Production
- **問題**: 本番コード側にテストのためだけの分岐や hook(`if testing { ... }` など)が紛れ込んでいる。
- **処方**: Test Double や DI で外側から依存を差し替える設計にする。production にテスト由来の重みを残さない。
- **出典**: <http://xunitpatterns.com/Test%20Logic%20in%20Production.html>

## Behavior Smells

テストを **走らせた** ときに気づく臭い。

### Assertion Roulette
- **問題**: 1 つのテスト内で複数の assert を撃っており、どこで落ちたか即座に判別できない。
- **処方**: Test Method を分ける、または Assertion Message でどの assert か判別可能にする。
- **出典**: <http://xunitpatterns.com/Assertion%20Roulette.html>

### Erratic Test
- **問題**: 同じテストが時々通り、時々落ちる。再現性が低く、デバッグ不能になる。
- **処方**: 原因によって対応が違う(下記サブ Smell 参照)。共通指針: テストを Self-Contained に、依存を顕在化、Shared Fixture を疑う。
- **出典**: <http://xunitpatterns.com/Erratic%20Test.html>

Erratic Test のサブ Smell:

- **Resource Leakage**: テストが資源(ファイル、コネクション、メモリ)を解放しないまま終わり、後続テストに影響。→ Automated Teardown / `t.Cleanup` で必ず解放。
- **Resource Optimism**: 「資源は当然そこにあるはず」と仮定し、無いと予期せず落ちる。→ Guard Assertion で前提を明示、Setup External Resource で資源を確実に用意。
- **Test Run War**: 同時実行されるテスト同士が同じ資源を奪い合う。→ Make Resource Unique でテストごとにユニークな名前にする。
- **Unrepeatable Test**: 同じテストを 2 回連続で動かすと 2 回目が落ちる(残骸が残る)。→ Automated Teardown を確実に。
- **Test Dependency**: あるテストが「先に別のテストが走った」ことを暗黙に前提にしている。→ Independent Test、`t.Cleanup` 徹底。
- **Interacting Tests**: 並列に走らせると干渉する。→ Fresh Fixture / 並列安全な状態管理。
- **Non-Deterministic Test**: 時刻、乱数、外部 API などで結果が揺れる。→ Test Stub で時刻や乱数を固定、Virtual Clock 導入。

### Fragile Test
- **問題**: 本番コードを「振る舞いを変えない」リファクタリングしたのに、テストが壊れる。
- **処方**: 振る舞いをテストする(R12)。実装詳細を assert しない。Sensitive Equality を避け、Custom Assertion で必要部分のみ比較する。
- **出典**: <http://xunitpatterns.com/Fragile%20Test.html>

Fragile Test のサブ Smell:

- **Interface Sensitivity**: SUT の API シグネチャを変えただけで多数のテストが壊れる。→ Creation Method で構築を抽象化、Test Helper で API 呼び出しを集中化。
- **Behavior Sensitivity**: SUT の内部挙動を変えただけで挙動と無関係なテストが落ちる。→ 観察可能な振る舞いだけ assert、内部状態を見ない。
- **Data Sensitivity**: テストが特定のデータ値に過剰に依存していて、データを変えると壊れる。→ Derived Value / Anonymous Creation でデータと無関係に。
- **Context Sensitivity**: テストが実行環境(時刻、ロケール、ファイルシステム等)に依存。→ 環境を Test Double で置換、Virtual Clock。
- **Overspecified Software**: テストが「実装はこうあるべき」を過剰に固定し、リファクタリングを阻害。→ 公開契約のみテスト、内部呼び出し履歴は見ない(Mock の濫用注意)。

### Frequent Debugging
- **問題**: テストが落ちても原因がすぐ分からず、デバッガで追わないと修正できない。
- **処方**: テストを小さくし、Single-Condition Test、Assertion Message で失敗時の情報量を増やす。
- **出典**: <http://xunitpatterns.com/Frequent%20Debugging.html>

### Manual Intervention
- **問題**: テスト実行中に人間の手作業(ファイル準備、ボタン押下、確認)が必要。CI で回せない。
- **処方**: Setup External Resource / Test Utility Method で完全自動化。
- **出典**: <http://xunitpatterns.com/Manual%20Intervention.html>

### Slow Tests
- **問題**: テストが遅すぎて、頻繁には流せない。回されないテストは無価値。
- **処方**: 重い fixture は Shared Fixture 化、I/O は Test Double で抑制、E2E は最小限に絞り単体テストで補う。
- **出典**: <http://xunitpatterns.com/Slow%20Tests.html>

## Project Smells

プロジェクト全体を眺めて気づく臭い。

### Buggy Tests
- **問題**: テスト自体にバグがあり、本番のバグを覆い隠している(常に通る、誤判定する等)。
- **処方**: テストレビュー、triangulation で複数入力を試す、自己一致比較(`assert.Equal(x, x)`)を排除。
- **出典**: <http://xunitpatterns.com/Buggy%20Tests.html>

### Developers Not Writing Tests
- **問題**: 開発者がテストを書かない。書く習慣・文化がない。
- **処方**: TDD 文化醸成、レビュー条件にテストを含める、Test Method/Test Helper のテンプレートを整備。
- **出典**: <http://xunitpatterns.com/Developers%20Not%20Writing%20Tests.html>

### High Test Maintenance Cost
- **問題**: 既存テストの保守に過剰な工数がかかる。production 変更のたびにテストが壊れる。
- **処方**: Fragile Test を退治、helper を集中化、Overspecified Software を解消。
- **出典**: <http://xunitpatterns.com/High%20Test%20Maintenance%20Cost.html>

### Production Bugs
- **問題**: テストが通っているのに本番でバグが見つかる。テスト網が抜けている。
- **処方**: Buggy Tests を検証、未カバーの境界を triangulation、Layer Test で抜けた層を補完。
- **出典**: <http://xunitpatterns.com/Production%20Bugs.html>

---

# Test Patterns

## xUnit Basics

### Test Method
- **問題**: テストコードをどこに書くか?
- **処方**: 1 つのテストを 1 つの Test Method としてクラス上に定義する。
- **出典**: <http://xunitpatterns.com/Test%20Method.html>

### Four Phase Test
- **問題**: 何をテストしているか一目で分かる構造にしたい。
- **処方**: テストを Setup → Exercise → Verify → Teardown の 4 フェーズで記述。Go では AAA(Arrange/Act/Assert)+ `t.Cleanup` がこれに相当。
- **出典**: <http://xunitpatterns.com/Four%20Phase%20Test.html>

### Assertion Method
- **問題**: テストを self-checking にする方法は?
- **処方**: 期待結果を判定するユーティリティメソッド(testify の `assert.Equal` 等)を呼び出す。
- **出典**: <http://xunitpatterns.com/Assertion%20Method.html>

### Assertion Message
- **問題**: 複数の assert がある場合、どれが落ちたか判別する方法は?
- **処方**: 各 Assertion Method に説明文字列を渡す(testify の第 3 引数 `msgAndArgs`)。
- **出典**: <http://xunitpatterns.com/Assertion%20Message.html>

## Test Strategy

### Fresh Fixture
- **問題**: どの fixture 戦略を使うべきか?
- **処方**: 各テストが自分専用の fixture を新規構築する。テスト同士の干渉を最小化。**このリポジトリのデフォルト**。
- **出典**: <http://xunitpatterns.com/Fresh%20Fixture.html>

### Shared Fixture
- **問題**: Slow Tests を避けたい場合の fixture 戦略は?
- **処方**: 同じ fixture を複数テストで共有する。WASM ランタイムのように構築コストの高いものに適用(`newTestAnalyzer` 等)。Erratic Test のリスクと引き換え。
- **出典**: <http://xunitpatterns.com/Shared%20Fixture.html>

### Standard Fixture
- **問題**: 同じ fixture デザインを複数テストで使うか?
- **処方**: 共通の fixture テンプレートを定義し、各テストで使い回す。General Fixture に育つリスクあり。
- **出典**: <http://xunitpatterns.com/Standard%20Fixture.html>

### Minimal Fixture
- **問題**: fixture をどこまで小さくすべきか?
- **処方**: 各テストに必要最小限の fixture を使う。General Fixture / Verbose Test の予防。
- **出典**: <http://xunitpatterns.com/Minimal%20Fixture.html>

### Recorded Test
- **問題**: テストをどう用意するか(対話的アプリの場合)?
- **処方**: アプリとの対話を記録し、再生してテストする(GUI/E2E ツール系)。
- **出典**: <http://xunitpatterns.com/Recorded%20Test.html>

### Scripted Test
- **問題**: テストをどう用意するか?
- **処方**: テストプログラムを手書きする。**このリポジトリのデフォルト**。
- **出典**: <http://xunitpatterns.com/Scripted%20Test.html>

### Data-Driven Test
- **問題**: 同じテストロジックを多数の入力に対して回したい。
- **処方**: テストデータをファイル等に置き、データを駆動軸にしてテストを生成。Go の table-driven test がこれの軽量版。
- **出典**: <http://xunitpatterns.com/Data-Driven%20Test.html>

### Layer Test
- **問題**: 階層化されたアーキテクチャで、層ごとに独立してテストしたい。
- **処方**: 各層に対して別個のテストを書く。下位層を Test Double で置換可能にしておく。
- **出典**: <http://xunitpatterns.com/Layer%20Test.html>

### Back Door Manipulation
- **問題**: ラウンドトリップテストできない場合に、どう独立検証するか?
- **処方**: SUT の API ではなく裏口(DB 直接読み書き等)経由で fixture 設定 / 結果検証する。production に test-only 経路を作るリスクあり。
- **出典**: <http://xunitpatterns.com/Back%20Door%20Manipulation.html>

## Fixture Setup

### Fresh Fixture Setup
- **問題**: Fresh Fixture をどう構築するかの戦略選択。
- **処方**: Inline Setup / Implicit Setup / Delegated Setup から選ぶ。
- **出典**: <http://xunitpatterns.com/Fresh%20Fixture%20Setup.html>

### Inline Setup
- **問題**: Fresh Fixture をどこに書くか?
- **処方**: 各 Test Method 内で fixture を直接構築する。テスト独立性が最大。Test Code Duplication のリスクあり。
- **出典**: <http://xunitpatterns.com/Inline%20Setup.html>

### Implicit Setup
- **問題**: 複数テストで共通する fixture を書くとき、どこに置くか?
- **処方**: `setUp` メソッド(Go では `TestMain` や `t.Run` の前段)に置き、テスト前に自動実行する。Obscure Test を生むリスク。
- **出典**: <http://xunitpatterns.com/Implicit%20Setup.html>

### Delegated Setup
- **問題**: Fresh Fixture を構築する方法は?
- **処方**: Creation Method を呼び出して構築する。Inline と Implicit の中間で、共通化と独立性のバランスが良い。**このリポジトリの主流**(`newTestAnalyzer` 等)。
- **出典**: <http://xunitpatterns.com/Delegated%20Setup.html>

### Lazy Setup
- **問題**: Shared Fixture を最初の利用直前に構築したい。
- **処方**: 初回アクセス時に Lazy Initialization で構築する。
- **出典**: <http://xunitpatterns.com/Lazy%20Setup.html>

### SuiteFixture Setup
- **問題**: Shared Fixture をスイート単位で 1 度だけ構築したい。
- **処方**: テストフレームワークが提供するスイート単位 setup メソッドを使う(JUnit の `@BeforeClass` 等)。Go なら `TestMain`。
- **出典**: <http://xunitpatterns.com/SuiteFixture%20Setup.html>

### Setup Decorator
- **問題**: Shared Fixture をスイート単位で構築したい(別のアプローチ)。
- **処方**: テストスイートをデコレータで包み、デコレータが setup/teardown を実行。
- **出典**: <http://xunitpatterns.com/Setup%20Decorator.html>

### Prebuilt Fixture
- **問題**: Shared Fixture をテスト実行とは別に構築したい。
- **処方**: 事前に DB seed や fixture ファイルを用意しておき、テストはそれを前提に走る。
- **出典**: <http://xunitpatterns.com/Prebuilt%20Fixture.html>

### Creation Method
- **問題**: Fresh Fixture をどう構築するか(具体的方法)?
- **処方**: 構築の詳細を隠したメソッド(`newUsersCatalog()` 等)を用意してそれを呼ぶ。Test Code Duplication 回避の主役。Object Mother / Test Data Builder も同系。
- **出典**: <http://xunitpatterns.com/Creation%20Method.html>

### Chained Tests
- **問題**: Shared Fixture を別のテストに構築させたい。
- **処方**: 別のテストが副作用として fixture を残すことを利用。**Test Dependency Smell の温床**なので避ける。
- **出典**: <http://xunitpatterns.com/Chained%20Tests.html>

### Shared Fixture Construction
- **問題**: Shared Fixture をいつ構築するか?
- **処方**: Lazy Setup / SuiteFixture Setup / Setup Decorator / Prebuilt Fixture から選ぶ。
- **出典**: <http://xunitpatterns.com/Shared%20Fixture%20Construction.html>

## Fixture Teardown

### Garbage-Collected Teardown
- **問題**: Test Fixture をどう破棄するか?
- **処方**: GC に任せる(Go なら通常これで十分)。外部リソースには使えない。
- **出典**: <http://xunitpatterns.com/Garbage-Collected%20Teardown.html>

### Implicit Teardown
- **問題**: Test Fixture をどう破棄するか?
- **処方**: フレームワークが `tearDown` 相当を自動呼び出し。Go では `t.Cleanup` がこれ。
- **出典**: <http://xunitpatterns.com/Implicit%20Teardown.html>

### Inline Teardown
- **問題**: Test Fixture をどう破棄するか?
- **処方**: Test Method の末尾に teardown コードを書く。defer/cleanup の方が安全だが、明示的にしたい場合に使う。
- **出典**: <http://xunitpatterns.com/Inline%20Teardown.html>

### Automated Teardown
- **問題**: Test Fixture をどう破棄するか(忘れない方法)?
- **処方**: テストが作った全リソースを自動追跡し、終了時に必ず解放する仕組みを用意。
- **出典**: <http://xunitpatterns.com/Automated%20Teardown.html>

## Result Verification

### State Verification
- **問題**: state を持つ SUT を self-checking にするには?
- **処方**: SUT を exercise した後の状態を観察し、期待値と比較。**このリポジトリのデフォルト**。
- **出典**: <http://xunitpatterns.com/State%20Verification.html>

### Behavior Verification
- **問題**: 観察可能な state がない SUT を self-checking にするには?
- **処方**: SUT の indirect output(別オブジェクトへの呼び出し)を捕捉し、期待される呼び出しと比較。Mock Object / Test Spy が用いる手法。
- **出典**: <http://xunitpatterns.com/Behavior%20Verification.html>

### Custom Assertion
- **問題**: テスト固有の同等性ロジックを self-checking にするには?
- **処方**: 必要な部分だけ比較する目的特化の Assertion Method を作る(Verification Method)。Sensitive Equality 回避。
- **出典**: <http://xunitpatterns.com/Custom%20Assertion.html>

### Guard Assertion
- **問題**: Conditional Test Logic を避けたい。
- **処方**: テスト内の `if` を「前提が崩れたら即失敗」する assert に置き換える。Go では `require.NoError` 等。
- **出典**: <http://xunitpatterns.com/Guard%20Assertion.html>

### Delta Assertion
- **問題**: fixture の初期状態を制御できない場合に self-checking にするには?
- **処方**: exercise 前後の差分(delta)で assert する。
- **出典**: <http://xunitpatterns.com/Delta%20Assertion.html>

### Unfinished Test Assertion
- **問題**: 未完成のテストが「通過」してしまうのを避けたい。
- **処方**: 必ず失敗する assert(`t.Fatal("not implemented")`)を置く。
- **出典**: <http://xunitpatterns.com/Unfinished%20Test%20Assertion.html>

## Test Doubles

実物の代わりに使う「テスト用ダミー」の分類。Meszaros の正規化された 5 種は **混同しないこと**。

### Test Stub
- **問題**: SUT が他コンポーネントから indirect input を受け取る場合の独立検証は?
- **処方**: 実物を、所望の入力を SUT に流し込むテスト専用オブジェクトで置換。
- **出典**: <http://xunitpatterns.com/Test%20Stub.html>

### Test Spy
- **問題**: Behavior Verification を実装するには?
- **処方**: Test Double に「呼ばれた履歴」を記録させ、テスト本体側で履歴を検証する。
- **出典**: <http://xunitpatterns.com/Test%20Spy.html>

### Mock Object
- **問題**: SUT の indirect output に対する Behavior Verification を実装するには?
- **処方**: 「期待される呼び出し」を事前にプログラムし、それと違ったら自動で失敗するテスト専用オブジェクト。Spy との違いは「期待」を事前に持つかどうか。
- **出典**: <http://xunitpatterns.com/Mock%20Object.html>

### Fake Object
- **問題**: 実物の depended-on object が使えない場合の独立検証は?
- **処方**: 軽量だが本物っぽく動く実装で置換(in-memory DB 等)。
- **出典**: <http://xunitpatterns.com/Fake%20Object.html>

### Dummy Object
- **問題**: 引数として渡すだけで実際には使われない値をどう用意するか?
- **処方**: 何の実装も持たないオブジェクトをパラメータとして渡す。Irrelevant Information の回避にも使う。
- **出典**: <http://xunitpatterns.com/Dummy%20Object.html>

### Configurable Test Double
- **問題**: Test Double に何を返させる/期待させるかをどう指定するか?
- **処方**: 再利用可能な Test Double を、テストごとに値を渡して構成する。
- **出典**: <http://xunitpatterns.com/Configurable%20Test%20Double.html>

### Hard-Coded Test Double
- **問題**: Test Double に何を返させるかをどう指定するか?
- **処方**: 値や期待をハードコードした Test Double クラスを作る。テストごとに専用の Test Double が増える。
- **出典**: <http://xunitpatterns.com/Hard-Coded%20Test%20Double.html>

## Test Organization

### Testcase Class per Class
- **問題**: Test Method をどう Testcase Class に整理するか?
- **処方**: SUT の 1 クラスに対し 1 Testcase Class を作る(`Foo` → `FooTest`)。Go では `foo_test.go` がこれに相当。
- **出典**: <http://xunitpatterns.com/Testcase%20Class%20per%20Class.html>

### Testcase Class per Feature
- **問題**: Test Method をどう Testcase Class に整理するか?
- **処方**: SUT の 1 機能に対し 1 Testcase Class を作る。`TestParser_ParseStatement_AST` のような関数命名がこれに近い。
- **出典**: <http://xunitpatterns.com/Testcase%20Class%20per%20Feature.html>

### Test Helper
- **問題**: 再利用可能な Test Utility Method をどこに置くか?
- **処方**: ヘルパー専用クラス/ファイルを作って集約。Go では `helpers_test.go` 等。
- **出典**: <http://xunitpatterns.com/Test%20Helper.html>

### Test Utility Method
- **問題**: Test Code Duplication を減らすには?
- **処方**: 再利用したいロジックを名前付きメソッドに包む。**ロジックは trivial に保つ**(R9)。
- **出典**: <http://xunitpatterns.com/Test%20Utility%20Method.html>

### Named Test Suite
- **問題**: 任意のテスト群をまとめて走らせるには?
- **処方**: 名前付きスイートを定義し、その中にテストを束ねる。Go なら build tag や `t.Run` のサブテスト命名。
- **出典**: <http://xunitpatterns.com/Named%20Test%20Suite.html>

### Parameterized Test
- **問題**: 同じテストロジックを多数の入力で回したい(Test Code Duplication 削減)。
- **処方**: fixture 値と期待値をパラメータ化し、ループで回す。Go の table-driven test がこれ。R5(triangulation)とも紐づく。
- **出典**: <http://xunitpatterns.com/Parameterized%20Test.html>

### Test Code Reuse
- **問題**: Test Code Duplication を減らす全般戦略。
- **処方**: Test Utility Method / Test Helper / Parameterized Test / Creation Method を組み合わせる。
- **出典**: <http://xunitpatterns.com/Test%20Code%20Reuse.html>

## Value Patterns

### Literal Value
- **問題**: テストで使う値をどう指定するか?
- **処方**: リテラル定数を使う。意味が明白なケースでは最も読みやすい。
- **出典**: <http://xunitpatterns.com/Literal%20Value.html>

### Generated Value
- **問題**: テストで使う値をどう指定するか(衝突回避)?
- **処方**: 実行ごとに値を生成する(タイムスタンプ、UUID 等)。Make Resource Unique と組み合わせ。
- **出典**: <http://xunitpatterns.com/Generated%20Value.html>

### Derived Value
- **問題**: テストで使う値をどう指定するか(意味を持たせる)?
- **処方**: 他の値から計算で導出する(`expectedTotal := price * quantity`)。Hard-Coded Test Data の回避。
- **出典**: <http://xunitpatterns.com/Derived%20Value.html>

---

# Test Refactorings

### Extract Testable Component
- **問題**: テストしたいロジックが密結合のコンポーネントに埋もれている。
- **処方**: テスト可能な独立コンポーネントとして切り出す。Hard to Test Code への対応。
- **出典**: <http://xunitpatterns.com/Extract%20Testable%20Component.html>

### Inline Resource
- **問題**: テストが外部ファイルなどに依存する Mystery Guest になっている。
- **処方**: 外部リソースの内容を fixture setup ロジック内へ取り込む。
- **出典**: <http://xunitpatterns.com/Inline%20Resource.html>

### Make Resource Unique
- **問題**: 複数テストが同じ名前のリソースを取り合っている(Test Run War)。
- **処方**: テスト内で使うリソースの名前をユニークにする。
- **出典**: <http://xunitpatterns.com/Make%20Resource%20Unique.html>

### Minimize Data
- **問題**: fixture が大きすぎてテストが理解しにくい。
- **処方**: fixture から不要な要素を削り、Minimal Fixture にする。
- **出典**: <http://xunitpatterns.com/Minimize%20Data.html>

### Replace Dependency with Test Double
- **問題**: SUT の依存先がテストの邪魔をしている。
- **処方**: 依存先を Test Double で置換し、SUT を独立にテスト可能にする。
- **出典**: <http://xunitpatterns.com/Replace%20Dependency%20with%20Test%20Double.html>

### Setup External Resource
- **問題**: SUT が外部リソース内容に依存しているが、内容が manual に用意されている。
- **処方**: fixture setup ロジックの中で外部リソースを生成する。Manual Intervention の解消。
- **出典**: <http://xunitpatterns.com/Setup%20External%20Resource.html>
