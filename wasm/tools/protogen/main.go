package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"
)

func main() {
	cmd := &cli.Command{
		Name:  "protogen",
		Usage: "Generate Go code from Protocol Buffer files",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "proto-path",
				Aliases: []string{"p"},
				Usage:   "Path to proto schema directory",
				Value:   "schemas",
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Output directory for generated code",
				Value:   "generated",
			},
			&cli.StringFlag{
				Name:  "go-module",
				Usage: "Go module path for generated code",
				Value: "github.com/glassmonkey/zetasql-wasm/wasm/generated",
			},
			&cli.BoolFlag{
				Name:  "verbose",
				Usage: "Enable verbose output",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return generate(
				cmd.String("proto-path"),
				cmd.String("output"),
				cmd.String("go-module"),
				cmd.Bool("verbose"),
			)
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

func generate(protoPath, outputDir, goModule string, verbose bool) error {
	// Get working directory
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	fmt.Println("🔧 Generating Go code from Protocol Buffer files...")
	fmt.Println()

	// Collect proto files
	protoFiles, err := collectProtoFiles(protoPath)
	if err != nil {
		return fmt.Errorf("failed to collect proto files: %w", err)
	}

	if len(protoFiles) == 0 {
		return fmt.Errorf("no proto files found in %s", protoPath)
	}

	fmt.Printf("📊 Proto files to process: %d\n", len(protoFiles))
	fmt.Printf("📁 Output directory: %s\n", outputDir)
	fmt.Println()

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate M options
	mOpts := generateMOptions(protoFiles, protoPath, goModule)

	// Build protoc command
	args := []string{
		"--go_out=" + outputDir,
		"--go_opt=paths=source_relative",
	}

	// Add well-known types mapping to standard protobuf packages
	wellKnownTypes := map[string]string{
		"google/protobuf/descriptor.proto":   "google.golang.org/protobuf/types/descriptorpb",
		"google/protobuf/timestamp.proto":    "google.golang.org/protobuf/types/known/timestamppb",
		"google/protobuf/duration.proto":     "google.golang.org/protobuf/types/known/durationpb",
		"google/protobuf/empty.proto":        "google.golang.org/protobuf/types/known/emptypb",
		"google/protobuf/struct.proto":       "google.golang.org/protobuf/types/known/structpb",
		"google/protobuf/wrappers.proto":     "google.golang.org/protobuf/types/known/wrapperspb",
		"google/protobuf/any.proto":          "google.golang.org/protobuf/types/known/anypb",
		"google/protobuf/field_mask.proto":   "google.golang.org/protobuf/types/known/fieldmaskpb",
		"google/protobuf/source_context.proto": "google.golang.org/protobuf/types/known/sourcecontextpb",
		"google/protobuf/type.proto":         "google.golang.org/protobuf/types/known/typepb",
		"google/protobuf/api.proto":          "google.golang.org/protobuf/types/known/apipb",
	}
	for proto, goPkg := range wellKnownTypes {
		args = append(args, fmt.Sprintf("--go_opt=M%s=%s", proto, goPkg))
	}

	args = append(args, mOpts...)
	args = append(args, "--proto_path="+protoPath)

	// Add system protobuf include path for well-known types if it exists
	if _, err := os.Stat("/usr/include"); err == nil {
		args = append(args, "--proto_path=/usr/include")
	}

	args = append(args, protoFiles...)

	if verbose {
		fmt.Println("🔍 Executing protoc command:")
		fmt.Printf("   protoc %s\n", strings.Join(args, " \\\n      "))
		fmt.Println()
	}

	// Execute protoc
	protocCmd := exec.Command("protoc", args...)
	protocCmd.Dir = workDir
	output, err := protocCmd.CombinedOutput()
	if err != nil {
		// Filter out warnings
		outputStr := string(output)
		lines := strings.Split(outputStr, "\n")
		var errors []string
		for _, line := range lines {
			if line != "" && !strings.Contains(line, "warning: Import") && !strings.Contains(line, "is unused") {
				errors = append(errors, line)
			}
		}
		if len(errors) > 0 {
			return fmt.Errorf("protoc failed:\n%s", strings.Join(errors, "\n"))
		}
	}

	fmt.Println()
	fmt.Println("✅ Code generation complete!")
	fmt.Println()

	// Print summary
	if err := printSummary(outputDir, verbose); err != nil {
		return err
	}

	fmt.Println("🎉 Done! Generated code is ready in:", outputDir+"/")
	return nil
}

func collectProtoFiles(protoPath string) ([]string, error) {
	var protoFiles []string

	// Recursively collect all .proto files
	err := filepath.Walk(protoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			// Skip testdata and information_schema directories
			if info.Name() == "testdata" || info.Name() == "information_schema" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only include .proto files
		if filepath.Ext(path) == ".proto" {
			protoFiles = append(protoFiles, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return protoFiles, nil
}


func generateMOptions(protoFiles []string, protoPath, goModule string) []string {
	var opts []string

	for _, protoFile := range protoFiles {
		// Remove protoPath prefix
		relPath := strings.TrimPrefix(protoFile, protoPath+"/")

		// Get directory path
		dirPath := filepath.Dir(relPath)

		// Generate Go package path
		goPkg := filepath.Join(goModule, dirPath)

		// Add M option
		opts = append(opts, fmt.Sprintf("--go_opt=M%s=%s", relPath, goPkg))
	}

	return opts
}

func printSummary(outputDir string, verbose bool) error {
	// Count generated files and calculate total size
	var count int
	var totalSize int64
	var directories []string

	err := filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			directories = append(directories, path)
		} else {
			totalSize += info.Size()
			if strings.HasSuffix(path, ".pb.go") {
				count++
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	fmt.Println("📈 Summary:")
	fmt.Printf("   - Generated files: %d\n", count)
	fmt.Printf("   - Total size: %s\n", formatSize(totalSize))
	fmt.Println()

	// Print directory structure
	if verbose {
		fmt.Println("📂 Generated directory structure:")
		for _, dir := range directories {
			fmt.Printf("   %s\n", dir)
		}
		fmt.Println()
	}

	return nil
}

// formatSize formats bytes to human-readable size (e.g., 3.1M, 152K)
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"K", "M", "G", "T"}
	return fmt.Sprintf("%.1f%s", float64(bytes)/float64(div), units[exp])
}
