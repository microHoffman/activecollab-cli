package cli

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestStoredCredentialsValidation(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*storedConfiguration)
		wantError string
	}{
		{name: "valid"},
		{name: "unsupported schema", configure: func(configuration *storedConfiguration) { configuration.Version = 2 }, wantError: "unsupported"},
		{name: "missing URL", configure: func(configuration *storedConfiguration) { configuration.URL = "" }, wantError: "no URL"},
		{name: "missing token", configure: func(configuration *storedConfiguration) { configuration.Token = "" }, wantError: "no token"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := authTestOptions(t)
			configuration := storedConfiguration{
				Version: credentialsSchemaVersion,
				URL:     "https://activecollab.example.com/api/v1",
				Account: "person@example.com",
				Token:   "secret-token",
			}
			if test.configure != nil {
				test.configure(&configuration)
			}
			if err := options.saveConfiguration(configuration); err != nil {
				t.Fatal(err)
			}
			loaded, err := options.loadConfiguration()
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("loadConfiguration() error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if loaded != configuration {
				t.Fatalf("loadConfiguration() = %#v, want %#v", loaded, configuration)
			}
		})
	}
}

func TestStoredCredentialsRejectMalformedAndOversizedFiles(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		wantError string
	}{
		{name: "malformed", data: []byte("not JSON"), wantError: "decode"},
		{name: "oversized", data: make([]byte, maxConfigSize+1), wantError: "unexpectedly large"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := authTestOptions(t)
			if err := options.saveLoginCredentials(storedConfiguration{URL: "https://activecollab.example.com/api/v1", Token: "secret-token"}); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(options.configPath, test.data, 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := options.loadConfiguration()
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("loadConfiguration() error = %v", err)
			}
		})
	}
}

func TestStoredCredentialsRejectSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks may require elevated Windows privileges")
	}
	options := authTestOptions(t)
	target := filepath.Join(t.TempDir(), "target.json")
	if err := os.WriteFile(target, []byte(`{"version":1,"url":"https://example.com/api/v1","token":"secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(options.configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, options.configPath); err != nil {
		t.Fatal(err)
	}
	_, err := options.loadConfiguration()
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("loadConfiguration() error = %v", err)
	}
}

func TestStoredCredentialsRejectUnsafeUnixPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows credentials use DACLs instead of POSIX modes")
	}
	options := authTestOptions(t)
	if err := options.saveLoginCredentials(storedConfiguration{URL: "https://activecollab.example.com/api/v1", Token: "secret-token"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(options.configPath, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := options.loadConfiguration()
	if err == nil || !strings.Contains(err.Error(), "permissions") {
		t.Fatalf("loadConfiguration() error = %v", err)
	}
}

func TestLogoutWithoutStoredCredentialsIsIdempotent(t *testing.T) {
	options := authTestOptions(t)
	output, err := executeWithOptionsForTest(t, options, "auth", "logout", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, `"stored_credentials_removed":false`) {
		t.Fatalf("unexpected logout output: %s", output)
	}
	if _, err := os.Stat(options.configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("credentials file unexpectedly exists: %v", err)
	}
}
