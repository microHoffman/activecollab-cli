package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckDocumentationDetectsDrift(t *testing.T) {
	generated := t.TempDir()
	committed := t.TempDir()
	writeTestFile(t, generated, "activecollab.md", "current")
	writeTestFile(t, committed, "activecollab.md", "stale")

	err := checkDocumentation(generated, committed)
	if err == nil || !strings.Contains(err.Error(), "outdated activecollab.md") {
		t.Fatalf("unexpected drift result: %v", err)
	}

	writeTestFile(t, committed, "activecollab.md", "current")
	if err := checkDocumentation(generated, committed); err != nil {
		t.Fatalf("matching documentation reported drift: %v", err)
	}
}

func TestUpdateDocumentationWritesCurrentFilesAndRemovesStaleGeneratedFiles(t *testing.T) {
	generated := t.TempDir()
	committed := t.TempDir()
	writeTestFile(t, generated, "activecollab.md", "current")
	writeTestFile(t, committed, "activecollab_old.md", "stale")

	if err := updateDocumentation(generated, committed); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(committed, "activecollab.md"))
	if err != nil || string(content) != "current" {
		t.Fatalf("generated content = %q, err = %v", content, err)
	}
	if _, err := os.Stat(filepath.Join(committed, "activecollab_old.md")); !os.IsNotExist(err) {
		t.Fatalf("stale generated file still exists or returned an unexpected error: %v", err)
	}
}

func writeTestFile(t *testing.T, directory, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
