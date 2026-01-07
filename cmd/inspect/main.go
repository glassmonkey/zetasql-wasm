package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	zetasql "github.com/glassmonkey/zetasql-wasm"
)

func main() {
	ctx := context.Background()
	var sql string

	// Get SQL from command line arguments or stdin
	if len(os.Args) > 1 {
		// Use command line argument
		sql = strings.Join(os.Args[1:], " ")
	} else {
		// Read from stdin
		fmt.Println("Enter SQL statement:")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			sql = scanner.Text()
		}
		if sql == "" {
			fmt.Println("Error: No SQL statement provided")
			os.Exit(1)
		}
	}

	// Create parser with WASM backend
	parser, err := zetasql.NewParser(ctx)
	if err != nil {
		fmt.Printf("Error: Failed to initialize ZetaSQL WASM parser: %v\n", err)
		os.Exit(1)
	}
	defer parser.Close(ctx)

	// Parse the SQL statement using WASM
	stmt, err := parser.ParseStatement(ctx, sql)
	if err != nil {
		fmt.Printf("Error: Failed to parse SQL: %v\n", err)
		os.Exit(1)
	}

	// Display the result
	displayResult(stmt)
}

func displayResult(stmt *zetasql.Statement) {
	fmt.Println("\n=== ZetaSQL Parse Result ===")
	fmt.Printf("SQL: %s\n", stmt.SQL)
	fmt.Printf("Parsed: %v\n", stmt.Parsed)

	if stmt.Parsed {
		fmt.Println("\n=== Abstract Syntax Tree ===")
		fmt.Println(stmt.AST)
		fmt.Println("\nStatus: ✓ Successfully parsed")
	} else {
		fmt.Println("\n=== Parse Error ===")
		fmt.Println(stmt.Error)
		fmt.Println("\nStatus: ✗ Parse failed")
		os.Exit(1)
	}
}
