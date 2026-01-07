// C++ bridge code - Connects ZetaSQL and WASM
#include <string>
#include <memory>
#include <emscripten.h>

#include "zetasql/public/parse_resume_location.h"
#include "zetasql/public/parse_helpers.h"
#include "zetasql/public/analyzer.h"

extern "C" {

// Parse SQL and return AST string
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

} // extern "C"
