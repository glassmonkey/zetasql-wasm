# zetasql-wasm ドキュメント

> **⚠️ 開発中**: 現在、WASMビルドのみ完了しています。Go SDK実装はTODOです。

このディレクトリには、zetasql-wasmプロジェクトの設計・運用に関するドキュメントが含まれています。

## ドキュメント一覧

### 設計ドキュメント

- **[architecture.md](./architecture.md)** - アーキテクチャ設計書
  - プロジェクトの全体構成、技術選定の理由、ディレクトリ構成、バージョン管理方針

### ビルド・開発

- **[patches.md](./patches.md)** - パッチ適用ガイド
  - WASMビルド用パッチの詳細説明、各パッチのモチベーションと安全性

### テスト

- **[wasm-sdk-test-plan.md](./wasm-sdk-test-plan.md)** - WASM SDKテスト計画
  - 機能別のテスト計画、Go SDKのテストケース、ICU依存機能の動作状況

### 今後追加予定

- **build.md** - ビルド手順（TODO）
- **migration.md** - 移行ガイド（TODO）

## 関連リンク

- [プロジェクトREADME](../../README.md) - プロジェクト概要とクイックスタート
- [English Documentation](../en/README.md) - 英語ドキュメント

### 外部リソース

- [ZetaSQL公式ドキュメント](https://github.com/google/zetasql/tree/master/docs)
- [wazeroドキュメント](https://wazero.io/)
- [Emscriptenドキュメント](https://emscripten.org/docs/)
- [goccy/go-zetasql](https://github.com/goccy/go-zetasql) - 参考実装（cgoベース）

---

**Last Updated**: 2026-01-08
