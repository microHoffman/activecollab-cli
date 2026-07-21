// Command gen-docs generates the committed CLI command reference from the
// same Cobra command tree used by the activecollab binary.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/microHoffman/activecollab-cli/internal/cli"
	"github.com/spf13/cobra/doc"
)

const (
	modulePath      = "module github.com/microHoffman/activecollab-cli"
	targetDirectory = "docs/commands"
)

func main() {
	check := flag.Bool("check", false, "verify that committed command documentation is current")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "gen-docs does not accept positional arguments")
		os.Exit(2)
	}
	if err := run(*check); err != nil {
		fmt.Fprintln(os.Stderr, "gen-docs:", err)
		os.Exit(1)
	}
}

func run(check bool) error {
	if err := verifyRepositoryRoot(); err != nil {
		return err
	}
	temporaryDirectory, err := os.MkdirTemp("", "activecollab-command-docs-*")
	if err != nil {
		return fmt.Errorf("create temporary documentation directory: %w", err)
	}
	defer os.RemoveAll(temporaryDirectory)

	if err := generate(temporaryDirectory); err != nil {
		return err
	}
	if check {
		return checkDocumentation(temporaryDirectory, targetDirectory)
	}
	return updateDocumentation(temporaryDirectory, targetDirectory)
}

func verifyRepositoryRoot() error {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return fmt.Errorf("run from the activecollab-cli repository root: %w", err)
	}
	if !strings.HasPrefix(string(data), modulePath+"\n") {
		return fmt.Errorf("run from the activecollab-cli repository root")
	}
	return nil
}

func generate(outputDirectory string) error {
	command := cli.NewCommand("dev")
	command.InitDefaultCompletionCmd()
	command.DisableAutoGenTag = true
	if err := doc.GenMarkdownTree(command, outputDirectory); err != nil {
		return fmt.Errorf("generate command documentation: %w", err)
	}
	return nil
}

func checkDocumentation(generatedDirectory, committedDirectory string) error {
	generated, err := readMarkdownFiles(generatedDirectory)
	if err != nil {
		return err
	}
	committed, err := readMarkdownFiles(committedDirectory)
	if err != nil {
		return err
	}

	var differences []string
	for _, name := range sortedUnion(generated, committed) {
		generatedContent, generatedExists := generated[name]
		committedContent, committedExists := committed[name]
		switch {
		case !committedExists:
			differences = append(differences, "missing "+name)
		case !generatedExists:
			differences = append(differences, "unexpected "+name)
		case !bytes.Equal(generatedContent, committedContent):
			differences = append(differences, "outdated "+name)
		}
	}
	if len(differences) != 0 {
		return fmt.Errorf("command documentation is not current (%s); run `mise run docs`", strings.Join(differences, ", "))
	}
	return nil
}

func updateDocumentation(generatedDirectory, committedDirectory string) error {
	generated, err := readMarkdownFiles(generatedDirectory)
	if err != nil {
		return err
	}
	committed, err := readMarkdownFiles(committedDirectory)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(committedDirectory, 0o755); err != nil {
		return fmt.Errorf("create command documentation directory: %w", err)
	}
	for name, content := range generated {
		path := filepath.Join(committedDirectory, name)
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	for name := range committed {
		if _, exists := generated[name]; exists {
			continue
		}
		path := filepath.Join(committedDirectory, name)
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove stale generated file %s: %w", path, err)
		}
	}
	return nil
}

func readMarkdownFiles(directory string) (map[string][]byte, error) {
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return map[string][]byte{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", directory, err)
	}
	files := make(map[string][]byte)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		files[entry.Name()] = content
	}
	return files, nil
}

func sortedUnion(left, right map[string][]byte) []string {
	names := make(map[string]struct{}, len(left)+len(right))
	for name := range left {
		names[name] = struct{}{}
	}
	for name := range right {
		names[name] = struct{}{}
	}
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}
