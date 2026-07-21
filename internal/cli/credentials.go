package cli

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"
)

const (
	credentialService = "activecollab-cli"
	maxConfigSize     = 64 << 10
)

var errSecretNotFound = errors.New("credential not found")

type secretStore interface {
	Get(string) (string, error)
	Set(string, string) error
	Delete(string) error
	Probe() error
}

type keyringSecretStore struct{}

func (keyringSecretStore) Get(key string) (string, error) {
	value, err := keyring.Get(credentialService, key)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", errSecretNotFound
	}
	return value, err
}

func (keyringSecretStore) Set(key, value string) error {
	return keyring.Set(credentialService, key, value)
}

func (keyringSecretStore) Delete(key string) error {
	err := keyring.Delete(credentialService, key)
	if errors.Is(err, keyring.ErrNotFound) {
		return errSecretNotFound
	}
	return err
}

func (store keyringSecretStore) Probe() error {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return fmt.Errorf("generate credential-store probe: %w", err)
	}
	key := "probe-" + hex.EncodeToString(random)
	if err := store.Set(key, "probe"); err != nil {
		return err
	}
	if err := store.Delete(key); err != nil {
		return fmt.Errorf("remove credential-store probe: %w", err)
	}
	return nil
}

type storedConfiguration struct {
	URL     string `json:"url"`
	Account string `json:"account,omitempty"`
}

type resolvedCredentials struct {
	URL     string
	Token   string
	Account string
	Source  string
}

func (options *rootOptions) resolveCredentials() (resolvedCredentials, error) {
	resolved := resolvedCredentials{
		URL:   strings.TrimSpace(os.Getenv("ACTIVECOLLAB_URL")),
		Token: strings.TrimSpace(os.Getenv("ACTIVECOLLAB_TOKEN")),
	}
	if resolved.Token != "" {
		resolved.Source = "environment"
	}
	if resolved.URL != "" && resolved.Token != "" {
		return resolved, nil
	}

	configuration, configErr := options.loadConfiguration()
	if configErr != nil && !errors.Is(configErr, os.ErrNotExist) {
		return resolvedCredentials{}, configErr
	}
	if configErr == nil {
		if resolved.URL == "" {
			resolved.URL = configuration.URL
		}
		if resolved.URL == configuration.URL {
			resolved.Account = configuration.Account
		}
	}
	if resolved.URL == "" {
		return resolvedCredentials{}, errors.New("ActiveCollab URL is required; set ACTIVECOLLAB_URL or run `activecollab auth login --url <self-hosted-api-url>`")
	}
	if resolved.Token == "" {
		token, err := options.secrets().Get(resolved.URL)
		if errors.Is(err, errSecretNotFound) {
			return resolvedCredentials{}, errors.New("ActiveCollab token is required; set ACTIVECOLLAB_TOKEN or run `activecollab auth login --url <self-hosted-api-url>`")
		}
		if err != nil {
			return resolvedCredentials{}, fmt.Errorf("read ActiveCollab token from OS credential store: %w", err)
		}
		resolved.Token = strings.TrimSpace(token)
		resolved.Source = "credential_store"
	}
	if resolved.Token == "" {
		return resolvedCredentials{}, errors.New("ActiveCollab token is empty; log in again or set ACTIVECOLLAB_TOKEN")
	}
	return resolved, nil
}

func (options *rootOptions) secrets() secretStore {
	if options.secretStore != nil {
		return options.secretStore
	}
	return keyringSecretStore{}
}

func (options *rootOptions) configurationPath() (string, error) {
	if options.configPath != "" {
		return options.configPath, nil
	}
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user configuration directory: %w", err)
	}
	return filepath.Join(directory, "activecollab", "config.json"), nil
}

func (options *rootOptions) loadConfiguration() (storedConfiguration, error) {
	path, err := options.configurationPath()
	if err != nil {
		return storedConfiguration{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return storedConfiguration{}, err
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxConfigSize+1))
	if err != nil {
		return storedConfiguration{}, fmt.Errorf("read ActiveCollab configuration: %w", err)
	}
	if len(data) > maxConfigSize {
		return storedConfiguration{}, errors.New("ActiveCollab configuration is unexpectedly large")
	}
	var configuration storedConfiguration
	if err := json.Unmarshal(data, &configuration); err != nil {
		return storedConfiguration{}, fmt.Errorf("decode ActiveCollab configuration: %w", err)
	}
	configuration.URL = strings.TrimSpace(configuration.URL)
	configuration.Account = strings.TrimSpace(configuration.Account)
	if configuration.URL == "" {
		return storedConfiguration{}, errors.New("ActiveCollab configuration has no URL")
	}
	return configuration, nil
}

func (options *rootOptions) saveLoginCredentials(configuration storedConfiguration, token string) error {
	store := options.secrets()
	previousConfiguration, previousConfigErr := options.loadConfiguration()
	if previousConfigErr != nil && !errors.Is(previousConfigErr, os.ErrNotExist) {
		return previousConfigErr
	}

	previousToken, previousTokenErr := store.Get(configuration.URL)
	if previousTokenErr != nil && !errors.Is(previousTokenErr, errSecretNotFound) {
		return fmt.Errorf("read existing ActiveCollab token from OS credential store: %w", previousTokenErr)
	}
	if err := store.Set(configuration.URL, token); err != nil {
		return fmt.Errorf("save ActiveCollab token in OS credential store: %w", err)
	}
	if err := options.saveConfiguration(configuration); err != nil {
		if previousTokenErr == nil {
			_ = store.Set(configuration.URL, previousToken)
		} else {
			_ = store.Delete(configuration.URL)
		}
		return err
	}

	if previousConfigErr == nil && previousConfiguration.URL != configuration.URL {
		if err := store.Delete(previousConfiguration.URL); err != nil && !errors.Is(err, errSecretNotFound) {
			return fmt.Errorf("new credentials saved, but old credential could not be removed: %w", err)
		}
	}
	return nil
}

func (options *rootOptions) saveConfiguration(configuration storedConfiguration) error {
	path, err := options.configurationPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(configuration, "", "  ")
	if err != nil {
		return fmt.Errorf("encode ActiveCollab configuration: %w", err)
	}
	data = append(data, '\n')

	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create ActiveCollab configuration directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".config-*")
	if err != nil {
		return fmt.Errorf("create temporary ActiveCollab configuration: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)

	writeErr := temporary.Chmod(0o600)
	if writeErr == nil {
		_, writeErr = temporary.Write(data)
	}
	if writeErr == nil {
		writeErr = temporary.Sync()
	}
	closeErr := temporary.Close()
	if writeErr != nil {
		return fmt.Errorf("write ActiveCollab configuration: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close ActiveCollab configuration: %w", closeErr)
	}
	if err := replaceFile(temporaryName, path); err != nil {
		return fmt.Errorf("install ActiveCollab configuration: %w", err)
	}
	return nil
}
