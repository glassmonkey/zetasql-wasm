# zetasql-wasm Documentation

> **⚠️ Work in Progress**: Currently, only the WASM build is completed. Go SDK implementation is TODO.

This directory contains design and operational documentation for the zetasql-wasm project.

## Documentation List

### Design Documents

- **[architecture.md](./architecture.md)** - Architecture Design Document
  - Project structure, technology selection, directory structure, version management

### Build & Development

- **[patches.md](./patches.md)** - Patch Application Guide
  - WASM build patches explained, motivation and safety of each patch

### Testing

- **[wasm-sdk-test-plan.md](./wasm-sdk-test-plan.md)** - WASM SDK Test Plan
  - Feature-based test plan, Go SDK test cases, ICU dependency status

### Planned for Future

- **build.md** - Build Instructions (TODO)
- **migration.md** - Migration Guide (TODO)

## Related Links

- [Project README](../../README.md) - Project overview and quick start
- [Japanese Documentation](../ja/README.md) - 日本語ドキュメント

### External Resources

- [ZetaSQL Official Documentation](https://github.com/google/zetasql/tree/master/docs)
- [wazero Documentation](https://wazero.io/)
- [Emscripten Documentation](https://emscripten.org/docs/)
- [goccy/go-zetasql](https://github.com/goccy/go-zetasql) - Reference implementation (CGO-based)

---

**Last Updated**: 2026-01-08
