package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

func authTestOptions(t *testing.T) *rootOptions {
	t.Helper()
	t.Setenv("ACTIVECOLLAB_URL", "")
	t.Setenv("ACTIVECOLLAB_TOKEN", "")
	return &rootOptions{
		timeout:    0,
		version:    "test",
		configPath: filepath.Join(t.TempDir(), "activecollab", "credentials.json"),
	}
}

func TestAuthLoginIssuesValidatesAndStoresToken(t *testing.T) {
	const (
		password = "sensitive-password"
		token    = "sensitive-token"
	)
	var issueRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.Method + " " + request.URL.Path {
		case "POST /api/v1/issue-token":
			issueRequests.Add(1)
			if got := request.Header.Get("X-Angie-AuthApiToken"); got != "" {
				t.Errorf("login request unexpectedly contained token header %q", got)
			}
			var payload map[string]string
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Errorf("decode login payload: %v", err)
			}
			if payload["username"] != "person@example.com" || payload["password"] != password {
				t.Errorf("unexpected login payload: %#v", payload)
			}
			if payload["client_name"] != "activecollab-cli" || payload["client_vendor"] != "microHoffman" {
				t.Errorf("unexpected client metadata: %#v", payload)
			}
			_, _ = io.WriteString(w, fmt.Sprintf(`{"is_ok":true,"token":%q}`, token))
		case "GET /api/v1/info":
			if got := request.Header.Get("X-Angie-AuthApiToken"); got != token {
				t.Errorf("token header = %q", got)
			}
			_, _ = io.WriteString(w, `{"application":"ActiveCollab","version":"7.4.765"}`)
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()

	options := authTestOptions(t)
	options.promptSecret = func(prompt string) (string, error) {
		if prompt != "ActiveCollab password: " {
			t.Fatalf("unexpected secret prompt %q", prompt)
		}
		return password, nil
	}
	output, err := executeWithOptionsForTest(
		t,
		options,
		"auth", "login",
		"--url", server.URL+"/api/v1",
		"--email", "person@example.com",
		"--allow-insecure-http",
		"--json",
	)
	if err != nil {
		t.Fatal(err)
	}
	if issueRequests.Load() != 1 {
		t.Fatalf("issue-token requests = %d", issueRequests.Load())
	}
	if strings.Contains(output, token) || strings.Contains(output, password) {
		t.Fatalf("login output exposed a secret: %s", output)
	}
	configurationData, err := os.ReadFile(options.configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(configurationData), password) {
		t.Fatalf("credentials file exposed the password: %s", configurationData)
	}
	var configuration storedConfiguration
	if err := json.Unmarshal(configurationData, &configuration); err != nil {
		t.Fatal(err)
	}
	if configuration.Version != credentialsSchemaVersion || configuration.URL != server.URL+"/api/v1" || configuration.Account != "person@example.com" || configuration.Token != token {
		t.Fatalf("unexpected stored credentials: %#v", configuration)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(options.configPath)
		if err != nil {
			t.Fatal(err)
		}
		if permissions := info.Mode().Perm(); permissions != 0o600 {
			t.Fatalf("configuration permissions = %o", permissions)
		}
		directoryInfo, err := os.Stat(filepath.Dir(options.configPath))
		if err != nil {
			t.Fatal(err)
		}
		if permissions := directoryInfo.Mode().Perm(); permissions != 0o700 {
			t.Fatalf("credentials directory permissions = %o", permissions)
		}
	}

	infoOutput, err := executeWithOptionsForTest(t, options, "info", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(infoOutput, `"version":"7.4.765"`) || strings.Contains(infoOutput, token) {
		t.Fatalf("unexpected info output: %s", infoOutput)
	}
}

func TestAuthLoginAcceptsTokenFromStdin(t *testing.T) {
	const token = "existing-secret-token"
	var issueRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/api/v1/issue-token" {
			issueRequests.Add(1)
		}
		if request.URL.Path != "/api/v1/info" {
			http.NotFound(w, request)
			return
		}
		if got := request.Header.Get("X-Angie-AuthApiToken"); got != token {
			t.Errorf("token header = %q", got)
		}
		_, _ = io.WriteString(w, `{"application":"ActiveCollab","version":"7.4.765"}`)
	}))
	defer server.Close()

	options := authTestOptions(t)
	options.stdin = strings.NewReader(token + "\n")
	output, err := executeWithOptionsForTest(
		t,
		options,
		"auth", "login",
		"--url", server.URL+"/api/v1",
		"--token-stdin",
		"--allow-insecure-http",
	)
	if err != nil {
		t.Fatal(err)
	}
	if issueRequests.Load() != 0 {
		t.Fatalf("token-stdin made %d issue-token requests", issueRequests.Load())
	}
	if strings.Contains(output, token) {
		t.Fatalf("login output exposed token: %s", output)
	}
	configuration, err := options.loadConfiguration()
	if err != nil {
		t.Fatal(err)
	}
	if configuration.Token != token {
		t.Fatalf("stored token = %q", configuration.Token)
	}
}

func TestAuthLoginChecksCredentialFileBeforeIssuingToken(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()

	options := authTestOptions(t)
	blockedParent := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blockedParent, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	options.configPath = filepath.Join(blockedParent, "credentials.json")
	_, err := executeWithOptionsForTest(
		t,
		options,
		"auth", "login",
		"--url", server.URL+"/api/v1",
		"--email", "person@example.com",
		"--allow-insecure-http",
	)
	if err == nil || !strings.Contains(err.Error(), "prepare protected ActiveCollab credential storage") {
		t.Fatalf("unexpected error: %v", err)
	}
	if requests.Load() != 0 {
		t.Fatalf("login made %d requests before credential-file preflight", requests.Load())
	}
}

func TestAuthLoginFailureDoesNotExposeCredentialsOrResponseBody(t *testing.T) {
	const password = "password-that-must-not-leak"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"message":"password-that-must-not-leak"}`)
	}))
	defer server.Close()

	options := authTestOptions(t)
	options.promptSecret = func(string) (string, error) { return password, nil }
	output, err := executeWithOptionsForTest(
		t,
		options,
		"auth", "login",
		"--url", server.URL+"/api/v1",
		"--email", "person@example.com",
		"--allow-insecure-http",
	)
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(err.Error(), password) || strings.Contains(output, password) {
		t.Fatalf("login failure exposed credentials: output=%q error=%v", output, err)
	}
}

func TestIssueTokenRefusesCrossOriginRedirect(t *testing.T) {
	var targetRequests atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		targetRequests.Add(1)
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", target.URL+"/capture")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer source.Close()

	options := &rootOptions{timeout: 0, version: "test"}
	_, err := options.issueToken(context.Background(), source.URL+"/api/v1", "person@example.com", "secret-password")
	if err == nil || !strings.Contains(err.Error(), "different origin") {
		t.Fatalf("unexpected redirect error: %v", err)
	}
	if targetRequests.Load() != 0 {
		t.Fatalf("redirect target received %d requests", targetRequests.Load())
	}
}

func TestAuthStatusAndLogoutNeverExposeToken(t *testing.T) {
	const (
		baseURL = "https://activecollab.example.com/api/v1"
		token   = "status-secret-token"
	)
	options := authTestOptions(t)
	if err := options.saveLoginCredentials(storedConfiguration{URL: baseURL, Account: "person@example.com", Token: token}); err != nil {
		t.Fatal(err)
	}

	statusOutput, err := executeWithOptionsForTest(t, options, "auth", "status", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(statusOutput, token) || !strings.Contains(statusOutput, `"source":"credential_file"`) {
		t.Fatalf("unexpected status output: %s", statusOutput)
	}

	logoutOutput, err := executeWithOptionsForTest(t, options, "auth", "logout", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(logoutOutput, token) || !strings.Contains(logoutOutput, `"remote_token_revoked":false`) {
		t.Fatalf("unexpected logout output: %s", logoutOutput)
	}
	if _, err := os.Stat(options.configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("configuration still exists after logout: %v", err)
	}
}

func TestCompleteEnvironmentCredentialsOverrideInvalidStoredConfiguration(t *testing.T) {
	options := authTestOptions(t)
	if err := os.MkdirAll(filepath.Dir(options.configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(options.configPath, []byte("not JSON"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ACTIVECOLLAB_URL", "https://environment.example.com/api/v1")
	t.Setenv("ACTIVECOLLAB_TOKEN", "environment-secret")

	output, err := executeWithOptionsForTest(t, options, "auth", "status", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, `"source":"environment"`) || strings.Contains(output, "environment-secret") {
		t.Fatalf("unexpected status output: %s", output)
	}
}

func TestNormalizeLoginURLRequiresCompleteSecureAPIURL(t *testing.T) {
	tests := []struct {
		name          string
		value         string
		allowInsecure bool
		want          string
		wantError     string
	}{
		{name: "https", value: "https://activecollab.example.com/api/v1/", want: "https://activecollab.example.com/api/v1"},
		{name: "self-hosted subpath", value: "https://example.com/tools/activecollab/api/v1", want: "https://example.com/tools/activecollab/api/v1"},
		{name: "missing API path", value: "https://activecollab.example.com", wantError: "complete /api/v1"},
		{name: "HTTP", value: "http://activecollab.example.com/api/v1", wantError: "refusing to send credentials over HTTP"},
		{name: "explicit HTTP", value: "http://localhost:8080/api/v1", allowInsecure: true, want: "http://localhost:8080/api/v1"},
		{name: "userinfo", value: "https://person:secret@example.com/api/v1", wantError: "must not contain credentials"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeLoginURL(test.value, test.allowInsecure)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("normalizeLoginURL() error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("normalizeLoginURL() = %q, want %q", got, test.want)
			}
		})
	}
}
