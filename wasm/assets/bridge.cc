// C++ bridge code - Connects ZetaSQL and WASM
#include <string>
#include <memory>
#include <cstring>
#include <emscripten.h>

#include "zetasql/public/parse_resume_location.h"
#include "zetasql/public/parse_helpers.h"
#include "zetasql/public/analyzer.h"
#include "zetasql/public/simple_catalog.h"
#include "zetasql/public/builtin_function_options.h"
#include "zetasql/public/language_options.h"
#include "zetasql/resolved_ast/resolved_node.h"
#include "zetasql/resolved_ast/serialization.pb.h"
#include "zetasql/parser/parse_tree_serializer.h"
#include "zetasql/parser/parse_tree.pb.h"

extern "C" {

// ============ Debug functions ============
// These functions help identify where crashes occur
// Note: No try/catch used because Emscripten has exceptions disabled by default

// Test 1: Just return a string (basic WASM test)
EMSCRIPTEN_KEEPALIVE
char* debug_test_basic() {
    return strdup("debug_test_basic: OK");
}

// Test 2: Test SimpleCatalog creation
EMSCRIPTEN_KEEPALIVE
char* debug_test_catalog() {
    zetasql::SimpleCatalog catalog("test_catalog");
    return strdup("debug_test_catalog: OK - SimpleCatalog created");
}

// Test 3: Test SimpleCatalog with AddZetaSQLFunctions
EMSCRIPTEN_KEEPALIVE
char* debug_test_catalog_with_functions() {
    zetasql::SimpleCatalog catalog("test_catalog");
    catalog.AddZetaSQLFunctions();
    return strdup("debug_test_catalog_with_functions: OK - Functions added");
}

// Test 4: Test AnalyzerOptions creation with minimal settings
EMSCRIPTEN_KEEPALIVE
char* debug_test_analyzer_options() {
    // Create AnalyzerOptions with explicit language options to avoid
    // default initialization issues in WASM environment
    zetasql::AnalyzerOptions options;

    // Set minimal language options
    zetasql::LanguageOptions language_options;
    language_options.SetSupportsAllStatementKinds();
    options.set_language(language_options);

    return strdup("debug_test_analyzer_options: OK - AnalyzerOptions created");
}

// Test 5: Test full AnalyzeStatement with simple SQL
EMSCRIPTEN_KEEPALIVE
char* debug_test_analyze() {
    zetasql::SimpleCatalog catalog("test_catalog");
    catalog.AddZetaSQLFunctions();
    zetasql::AnalyzerOptions options;

    std::unique_ptr<const zetasql::AnalyzerOutput> output;
    absl::Status status = zetasql::AnalyzeStatement(
        "SELECT 1", options, &catalog, catalog.type_factory(), &output);

    if (!status.ok()) {
        std::string error = "debug_test_analyze: ANALYZE FAILED - " + status.ToString();
        return strdup(error.c_str());
    }
    return strdup("debug_test_analyze: OK - AnalyzeStatement succeeded");
}

// ============ End Debug functions ============

// Parse SQL and return AST string (legacy)
EMSCRIPTEN_KEEPALIVE
char* parse_statement(const char* sql) {
    std::string sql_str(sql);
    zetasql::ParserOptions options;

    std::unique_ptr<zetasql::ParserOutput> output;
    absl::Status status = zetasql::ParseStatement(
        sql_str, options, &output);

    if (!status.ok()) {
        std::string error = "Error: " + status.ToString();
        return strdup(error.c_str());
    }

    // Convert AST information to string
    std::string result = output->statement()->DebugString();
    return strdup(result.c_str());
}

// Parse SQL and return serialized Parse Tree AST proto bytes
// Uses ParseStatement + ParseTreeSerializer (does NOT require AnalyzeStatement)
// Returns: pointer to struct { size: uint32, data: uint8[] }
EMSCRIPTEN_KEEPALIVE
void* parse_statement_proto(const char* sql) {
    std::string sql_str(sql);
    zetasql::ParserOptions parser_options;

    std::unique_ptr<zetasql::ParserOutput> parser_output;
    absl::Status status = zetasql::ParseStatement(
        sql_str, parser_options, &parser_output);

    if (!status.ok()) {
        std::string error = "Error: " + status.ToString();
        uint32_t size = error.size();
        void* result = malloc(sizeof(uint32_t) + size + 1);
        memcpy(result, &size, sizeof(uint32_t));
        memcpy((char*)result + sizeof(uint32_t), error.c_str(), size + 1);
        return result;
    }

    // Serialize Parse Tree AST to proto using ParseTreeSerializer
    zetasql::AnyASTStatementProto proto;
    absl::Status serialize_status = zetasql::ParseTreeSerializer::Serialize(
        parser_output->statement(), &proto);

    if (!serialize_status.ok()) {
        std::string error = "Error serializing: " + serialize_status.ToString();
        uint32_t size = error.size();
        void* result = malloc(sizeof(uint32_t) + size + 1);
        memcpy(result, &size, sizeof(uint32_t));
        memcpy((char*)result + sizeof(uint32_t), error.c_str(), size + 1);
        return result;
    }

    // Serialize proto to bytes
    std::string serialized;
    if (!proto.SerializeToString(&serialized)) {
        std::string error = "Error: Failed to serialize proto";
        uint32_t size = error.size();
        void* result = malloc(sizeof(uint32_t) + size + 1);
        memcpy(result, &size, sizeof(uint32_t));
        memcpy((char*)result + sizeof(uint32_t), error.c_str(), size + 1);
        return result;
    }

    // Allocate memory for size + data
    uint32_t size = serialized.size();
    void* result = malloc(sizeof(uint32_t) + size);
    memcpy(result, &size, sizeof(uint32_t));
    memcpy((char*)result + sizeof(uint32_t), serialized.data(), size);

    return result;
}

// SQL analysis (catalog support to be implemented in the future if needed)
EMSCRIPTEN_KEEPALIVE
char* analyze_statement(const char* sql) {
    // TODO: Receive and analyze catalog information
    // Currently only returns parse results
    return parse_statement(sql);
}

// Free memory
EMSCRIPTEN_KEEPALIVE
void free_string(char* ptr) {
    free(ptr);
}

// Free proto buffer memory
EMSCRIPTEN_KEEPALIVE
void free_proto_buffer(void* ptr) {
    free(ptr);
}

} // extern "C"
