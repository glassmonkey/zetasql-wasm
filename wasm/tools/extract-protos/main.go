package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"
)

func main() {
	cmd := &cli.Command{
		Name:  "extract-protos",
		Usage: "Extract proto schema files from Bazel build output",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "output-base",
				Usage:    "Bazel output base directory (from 'bazel info output_base')",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "schemas-dir",
				Usage:    "Output directory for extracted proto schemas",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "path-prefix",
				Usage: "Path prefix to filter proto files (e.g., '/zetasql+/zetasql/'). Empty extracts all protos.",
				Value: "/zetasql+/zetasql/",
			},
			&cli.BoolFlag{
				Name:  "verbose",
				Usage: "Enable verbose output",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return extract(
				cmd.String("output-base"),
				cmd.String("schemas-dir"),
				cmd.String("path-prefix"),
				cmd.Bool("verbose"),
			)
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

func extract(outputBase, schemasDir, pathPrefix string, verbose bool) error {
	if _, err := os.Stat(outputBase); os.IsNotExist(err) {
		return fmt.Errorf("output-base directory not found: %s", outputBase)
	}

	// Clean up previous extraction output
	if _, err := os.Stat(schemasDir); err == nil {
		if err := os.RemoveAll(schemasDir); err != nil {
			return fmt.Errorf("failed to clean up schemas directory %s: %w", schemasDir, err)
		}
	}

	fmt.Println("📦 Extracting proto schemas from Bazel build output...")
	fmt.Printf("   Source: %s\n", outputBase)
	fmt.Printf("   Destination: %s\n", schemasDir)
	if pathPrefix != "" {
		fmt.Printf("   Pattern: %s\n", pathPrefix)
	}
	fmt.Println()

	totalFiles := 0

	// Recursively find and copy proto files
	err := filepath.Walk(outputBase, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process .proto files
		if !strings.HasSuffix(path, ".proto") {
			return nil
		}

		// Calculate relative path from output-base
		relPath, err := filepath.Rel(outputBase, path)
		if err != nil {
			return err
		}

		var destPath string

		// If pathPrefix is empty, extract all protos with their original paths
		if pathPrefix == "" {
			destPath = filepath.Join(schemasDir, relPath)
		} else {
			// Extract only zetasql repository proto files matching the pattern
			// This filters out all other dependencies (protobuf+, googleapis+, etc.)
			idx := strings.Index(relPath, pathPrefix)
			if idx == -1 {
				return nil
			}

			// Extract path after pattern -> zetasql/XXX.proto
			zetasqlPath := "zetasql/" + relPath[idx+len(pathPrefix):]
			destPath = filepath.Join(schemasDir, zetasqlPath)
		}

		// Skip if file already exists (avoid overwriting)
		if _, err := os.Stat(destPath); err == nil {
			if verbose {
				relDest, _ := filepath.Rel(schemasDir, destPath)
				fmt.Printf("  ⏭️  Already exists: %s\n", relDest)
			}
			return nil
		}

		// Create destination directory
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(destPath), err)
		}

		// Copy file
		if err := copyFile(path, destPath); err != nil {
			return fmt.Errorf("failed to copy %s: %w", path, err)
		}

		if verbose {
			fmt.Printf("  → %s\n", relPath)
		}

		totalFiles++
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to extract proto files: %w", err)
	}

	fmt.Println()
	fmt.Printf("✅ Successfully extracted %d proto files to %s\n", totalFiles, schemasDir)

	return nil
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = sourceFile.Close() }()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = destFile.Close() }()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	return destFile.Sync()
}
