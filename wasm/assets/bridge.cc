// C++ bridge code - Connects ZetaSQL and WASM
#include <string>
#include <memory>
#include <cstring>
#include <emscripten.h>

#include "zetasql/public/parse_resume_location.h"
#include "zetasql/public/parse_helpers.h"
#include "zetasql/public/analyzer.h"
#include "zetasql/public/simple_catalog.h"
#include "zetasql/resolved_ast/resolved_node.h"
#include "zetasql/resolved_ast/serialization.pb.h"

extern "C" {

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

// Analyze SQL and return serialized Resolved AST proto bytes
// Returns: pointer to struct { size: uint32, data: uint8[] }
EMSCRIPTEN_KEEPALIVE
void* parse_statement_proto(const char* sql) {
    std::string sql_str(sql);

    // Create a simple catalog (no tables defined)
    zetasql::SimpleCatalog catalog("default_catalog");
    zetasql::AnalyzerOptions options;

    std::unique_ptr<const zetasql::AnalyzerOutput> output;
    absl::Status status = zetasql::AnalyzeStatement(
        sql_str, options, &catalog, catalog.type_factory(), &output);

    if (!status.ok()) {
        std::string error = "Error: " + status.ToString();
        // Return error as string with size prefix
        uint32_t size = error.size();
        void* result = malloc(sizeof(uint32_t) + size + 1);
        memcpy(result, &size, sizeof(uint32_t));
        memcpy((char*)result + sizeof(uint32_t), error.c_str(), size + 1);
        return result;
    }

    // Serialize Resolved AST to proto
    zetasql::FileDescriptorSetMap file_descriptor_set_map;
    zetasql::AnyResolvedNodeProto proto;
    absl::Status serialize_status = output->resolved_statement()->SaveTo(
        &file_descriptor_set_map, &proto);

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
