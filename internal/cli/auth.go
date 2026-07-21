package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	activecollab "github.com/microHoffman/activecollab-cli"
	"github.com/microHoffman/activecollab-cli/internal/transport"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const maxTokenSize = 16 << 10

func newAuthCommand(options *rootOptions) *cobra.Command {
	command := newCommandGroup(
		"auth",
		"Authenticate with ActiveCollab",
		"Create, inspect, or remove locally stored ActiveCollab credentials.",
	)
	command.AddCommand(
		newAuthLoginCommand(options),
		newAuthStatusCommand(options),
		newAuthLogoutCommand(options),
	)
	return command
}

func newAuthLoginCommand(options *rootOptions) *cobra.Command {
	var baseURL string
	var email string
	var tokenStdin bool
	var allowInsecureHTTP bool
	command := &cobra.Command{
		Use:   "login",
		Short: "Log in to a self-hosted ActiveCollab server",
		Long: `Issue and securely store an API token for a self-hosted ActiveCollab server.

Pass the server's complete /api/v1 URL. By default, login prompts for an email
address and a password without echoing the password. Use --token-stdin to save
an existing token supplied by a secret manager. The server URL, account name,
and token are stored in a protected credentials file for the current user.`,
		Example: `  activecollab auth login --url https://activecollab.example.com/api/v1
  secret-manager-command | activecollab auth login --url https://activecollab.example.com/api/v1 --token-stdin`,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, _ []string) error {
			normalizedURL, err := normalizeLoginURL(baseURL, allowInsecureHTTP)
			if err != nil {
				return err
			}
			if err := options.prepareCredentialStorage(); err != nil {
				return fmt.Errorf("prepare protected ActiveCollab credential storage: %w", err)
			}

			account := strings.TrimSpace(email)
			var token string
			if tokenStdin {
				token, err = options.readTokenFromStdin()
			} else {
				if account == "" {
					account, err = options.promptLine("ActiveCollab email: ")
				}
				if err == nil && account == "" {
					err = errors.New("ActiveCollab email is required")
				}
				var password string
				if err == nil {
					password, err = options.readSecret("ActiveCollab password: ")
				}
				if err == nil {
					token, err = options.issueToken(cmd.Context(), normalizedURL, account, password)
				}
			}
			if err != nil {
				return err
			}

			info, err := options.validateToken(cmd.Context(), normalizedURL, token)
			if err != nil {
				return fmt.Errorf("validate ActiveCollab token: %w", err)
			}
			if err := options.saveLoginCredentials(storedConfiguration{URL: normalizedURL, Account: account, Token: token}); err != nil {
				return err
			}
			return writeOutput(options.json, map[string]any{
				"authenticated": true,
				"url":           normalizedURL,
				"account":       account,
				"storage":       "credential_file",
				"server":        info,
			})
		},
	}
	command.Flags().StringVar(&baseURL, "url", "", "complete self-hosted ActiveCollab /api/v1 URL")
	command.Flags().StringVar(&email, "email", "", "ActiveCollab account email (prompted when omitted)")
	command.Flags().BoolVar(&tokenStdin, "token-stdin", false, "read an existing API token from standard input")
	command.Flags().BoolVar(&allowInsecureHTTP, "allow-insecure-http", false, "allow credentials over unencrypted HTTP")
	markFlagRequired(command, "url")
	return command
}

func newAuthStatusCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the active authentication source",
		Long:  "Show whether ActiveCollab credentials are available without displaying the API token or contacting the server.",
		Example: `  activecollab auth status
  activecollab auth status --json`,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(_ *cobra.Command, _ []string) error {
			credentials, err := options.resolveCredentials()
			if err != nil {
				return err
			}
			return writeOutput(options.json, map[string]any{
				"authenticated": true,
				"url":           credentials.URL,
				"account":       credentials.Account,
				"source":        credentials.Source,
			})
		},
	}
}

func newAuthLogoutCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove locally stored ActiveCollab credentials",
		Long: `Delete the local ActiveCollab credentials file. This does not revoke
the token on the ActiveCollab server and cannot clear ACTIVECOLLAB_URL or
ACTIVECOLLAB_TOKEN in the parent shell.`,
		Example: `  activecollab auth logout
  activecollab auth logout --json`,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(_ *cobra.Command, _ []string) error {
			path, err := options.configurationPath()
			if err != nil {
				return err
			}
			if err := os.Remove(path); errors.Is(err, os.ErrNotExist) {
				return writeOutput(options.json, map[string]any{
					"logged_out":                      true,
					"stored_credentials_removed":      false,
					"environment_credentials_present": environmentCredentialsPresent(),
					"remote_token_revoked":            false,
				})
			} else if err != nil {
				return fmt.Errorf("remove ActiveCollab credentials file: %w", err)
			}
			return writeOutput(options.json, map[string]any{
				"logged_out":                      true,
				"stored_credentials_removed":      true,
				"environment_credentials_present": environmentCredentialsPresent(),
				"remote_token_revoked":            false,
			})
		},
	}
}

func normalizeLoginURL(value string, allowInsecureHTTP bool) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(value), "/")
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid ActiveCollab URL %q", value)
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("ActiveCollab URL must not contain credentials, a query, or a fragment")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", fmt.Errorf("unsupported ActiveCollab URL scheme %q", parsed.Scheme)
	}
	if parsed.Scheme == "http" && !allowInsecureHTTP {
		return "", errors.New("refusing to send credentials over HTTP; use HTTPS or explicitly pass --allow-insecure-http")
	}
	if !strings.HasSuffix(parsed.Path, "/api/v1") {
		return "", errors.New("self-hosted ActiveCollab URL must include the complete /api/v1 base path")
	}
	return parsed.String(), nil
}

func (options *rootOptions) issueToken(ctx context.Context, baseURL, email, password string) (string, error) {
	payload, err := json.Marshal(map[string]string{
		"username":      email,
		"password":      password,
		"client_name":   "activecollab-cli",
		"client_vendor": "microHoffman",
	})
	if err != nil {
		return "", errors.New("encode ActiveCollab login request")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/issue-token", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create ActiveCollab login request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "activecollab-cli/"+options.version)

	base, _ := url.Parse(baseURL)
	client := options.newHTTPClient()
	configuredRedirectCheck := client.CheckRedirect
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if !transport.SameOrigin(request.URL, base) {
			return errors.New("refusing ActiveCollab login redirect to a different origin")
		}
		if configuredRedirectCheck != nil {
			return configuredRedirectCheck(request, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("send ActiveCollab login request: %w", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxConfigSize+1))
	if err != nil {
		return "", fmt.Errorf("read ActiveCollab login response: %w", err)
	}
	if len(data) > maxConfigSize {
		return "", errors.New("ActiveCollab login response is unexpectedly large")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("ActiveCollab login failed with HTTP %d", response.StatusCode)
	}
	var result struct {
		IsOK  bool   `json:"is_ok"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", errors.New("decode ActiveCollab login response")
	}
	result.Token = strings.TrimSpace(result.Token)
	if !result.IsOK || result.Token == "" {
		return "", errors.New("ActiveCollab did not issue an API token")
	}
	return result.Token, nil
}

func (options *rootOptions) validateToken(ctx context.Context, baseURL, token string) (activecollab.Info, error) {
	client, err := activecollab.NewClient(activecollab.Config{
		BaseURL:    baseURL,
		Token:      token,
		HTTPClient: options.newHTTPClient(),
		UserAgent:  "activecollab-cli/" + options.version,
	})
	if err != nil {
		return activecollab.Info{}, err
	}
	return client.Info(ctx)
}

func (options *rootOptions) readTokenFromStdin() (string, error) {
	input := options.input()
	if file, ok := input.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		return "", errors.New("--token-stdin requires a pipe or redirected standard input")
	}
	data, err := io.ReadAll(io.LimitReader(input, maxTokenSize+1))
	if err != nil {
		return "", fmt.Errorf("read ActiveCollab token from standard input: %w", err)
	}
	if len(data) > maxTokenSize {
		return "", errors.New("ActiveCollab token from standard input is unexpectedly large")
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", errors.New("ActiveCollab token from standard input is empty")
	}
	return token, nil
}

func (options *rootOptions) promptLine(prompt string) (string, error) {
	if _, err := fmt.Fprint(options.errorOutput(), prompt); err != nil {
		return "", err
	}
	value, err := bufio.NewReader(options.input()).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read input: %w", err)
	}
	return strings.TrimSpace(value), nil
}

func (options *rootOptions) readSecret(prompt string) (string, error) {
	if options.promptSecret != nil {
		return options.promptSecret(prompt)
	}
	input, ok := options.input().(*os.File)
	if !ok || !term.IsTerminal(int(input.Fd())) {
		return "", errors.New("interactive password entry requires a terminal; run `activecollab auth login` yourself or use --token-stdin")
	}
	if _, err := fmt.Fprint(options.errorOutput(), prompt); err != nil {
		return "", err
	}
	data, err := term.ReadPassword(int(input.Fd()))
	_, _ = fmt.Fprintln(options.errorOutput())
	if err != nil {
		return "", fmt.Errorf("read ActiveCollab password: %w", err)
	}
	password := string(data)
	if password == "" {
		return "", errors.New("ActiveCollab password is required")
	}
	return password, nil
}

func (options *rootOptions) input() io.Reader {
	if options.stdin != nil {
		return options.stdin
	}
	return os.Stdin
}

func (options *rootOptions) errorOutput() io.Writer {
	if options.stderr != nil {
		return options.stderr
	}
	return os.Stderr
}

func environmentCredentialsPresent() bool {
	return strings.TrimSpace(os.Getenv("ACTIVECOLLAB_URL")) != "" ||
		strings.TrimSpace(os.Getenv("ACTIVECOLLAB_TOKEN")) != ""
}
