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

type memorySecretStore struct {
	values   map[string]string
	probeErr error
}

func newMemorySecretStore() *memorySecretStore {
	return &memorySecretStore{values: make(map[string]string)}
}

func (store *memorySecretStore) Get(key string) (string, error) {
	value, ok := store.values[key]
	if !ok {
		return "", errSecretNotFound
	}
	return value, nil
}

func (store *memorySecretStore) Set(key, value string) error {
	store.values[key] = value
	return nil
}

func (store *memorySecretStore) Delete(key string) error {
	if _, ok := store.values[key]; !ok {
		return errSecretNotFound
	}
	delete(store.values, key)
	return nil
}

func (store *memorySecretStore) Probe() error {
	return store.probeErr
}

func authTestOptions(t *testing.T, store secretStore) *rootOptions {
	t.Helper()
	t.Setenv("ACTIVECOLLAB_URL", "")
	t.Setenv("ACTIVECOLLAB_TOKEN", "")
	return &rootOptions{
		timeout:     0,
		version:     "test",
		secretStore: store,
		configPath:  filepath.Join(t.TempDir(), "activecollab", "config.json"),
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

	store := newMemorySecretStore()
	options := authTestOptions(t, store)
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
	if got := store.values[server.URL+"/api/v1"]; got != token {
		t.Fatalf("stored token = %q", got)
	}

	configurationData, err := os.ReadFile(options.configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(configurationData), token) || strings.Contains(string(configurationData), password) {
		t.Fatalf("configuration exposed a secret: %s", configurationData)
	}
	if !strings.Contains(string(configurationData), `"account": "person@example.com"`) {
		t.Fatalf("configuration is missing account: %s", configurationData)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(options.configPath)
		if err != nil {
			t.Fatal(err)
		}
		if permissions := info.Mode().Perm(); permissions != 0o600 {
			t.Fatalf("configuration permissions = %o", permissions)
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

	store := newMemorySecretStore()
	options := authTestOptions(t, store)
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
	if got := store.values[server.URL+"/api/v1"]; got != token {
		t.Fatalf("stored token = %q", got)
	}
}

func TestAuthLoginChecksCredentialStoreBeforeIssuingToken(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()

	store := newMemorySecretStore()
	store.probeErr = errors.New("keyring unavailable")
	options := authTestOptions(t, store)
	_, err := executeWithOptionsForTest(
		t,
		options,
		"auth", "login",
		"--url", server.URL+"/api/v1",
		"--email", "person@example.com",
		"--allow-insecure-http",
	)
	if err == nil || !strings.Contains(err.Error(), "OS credential store is unavailable") {
		t.Fatalf("unexpected error: %v", err)
	}
	if requests.Load() != 0 {
		t.Fatalf("login made %d requests before credential-store preflight", requests.Load())
	}
}

func TestAuthLoginFailureDoesNotExposeCredentialsOrResponseBody(t *testing.T) {
	const password = "password-that-must-not-leak"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"message":"password-that-must-not-leak"}`)
	}))
	defer server.Close()

	options := authTestOptions(t, newMemorySecretStore())
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
	store := newMemorySecretStore()
	store.values[baseURL] = token
	options := authTestOptions(t, store)
	if err := options.saveConfiguration(storedConfiguration{URL: baseURL, Account: "person@example.com"}); err != nil {
		t.Fatal(err)
	}

	statusOutput, err := executeWithOptionsForTest(t, options, "auth", "status", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(statusOutput, token) || !strings.Contains(statusOutput, `"source":"credential_store"`) {
		t.Fatalf("unexpected status output: %s", statusOutput)
	}

	logoutOutput, err := executeWithOptionsForTest(t, options, "auth", "logout", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(logoutOutput, token) || !strings.Contains(logoutOutput, `"remote_token_revoked":false`) {
		t.Fatalf("unexpected logout output: %s", logoutOutput)
	}
	if _, ok := store.values[baseURL]; ok {
		t.Fatal("logout did not delete token")
	}
	if _, err := os.Stat(options.configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("configuration still exists after logout: %v", err)
	}
}

func TestCompleteEnvironmentCredentialsOverrideInvalidStoredConfiguration(t *testing.T) {
	options := authTestOptions(t, newMemorySecretStore())
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
