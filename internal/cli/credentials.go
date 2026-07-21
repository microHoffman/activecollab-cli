package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	credentialsSchemaVersion = 1
	maxConfigSize            = 64 << 10
)

type storedConfiguration struct {
	Version int    `json:"version"`
	URL     string `json:"url"`
	Account string `json:"account,omitempty"`
	Token   string `json:"token"`
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
			if resolved.Token == "" {
				resolved.Token = configuration.Token
				resolved.Source = "credential_file"
			}
		}
	}
	if resolved.URL == "" {
		return resolvedCredentials{}, errors.New("ActiveCollab URL is required; set ACTIVECOLLAB_URL or run `activecollab auth login --url <self-hosted-api-url>`")
	}
	if resolved.Token == "" {
		return resolvedCredentials{}, errors.New("ActiveCollab token is required; set ACTIVECOLLAB_TOKEN or run `activecollab auth login --url <self-hosted-api-url>`")
	}
	return resolved, nil
}

func (options *rootOptions) configurationPath() (string, error) {
	if options.configPath != "" {
		return options.configPath, nil
	}
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user configuration directory: %w", err)
	}
	return filepath.Join(directory, "activecollab", "credentials.json"), nil
}

func (options *rootOptions) prepareCredentialStorage() error {
	path, err := options.configurationPath()
	if err != nil {
		return err
	}
	directory := filepath.Dir(path)
	if err := prepareCredentialDirectory(directory); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".credentials-probe-*")
	if err != nil {
		return fmt.Errorf("create credential-storage probe: %w", err)
	}
	temporaryName := temporary.Name()
	if err := temporary.Close(); err != nil {
		_ = os.Remove(temporaryName)
		return fmt.Errorf("close credential-storage probe: %w", err)
	}
	if err := protectCredentialPath(temporaryName, false); err != nil {
		_ = os.Remove(temporaryName)
		return fmt.Errorf("protect credential-storage probe: %w", err)
	}
	if err := os.Remove(temporaryName); err != nil {
		return fmt.Errorf("remove credential-storage probe: %w", err)
	}
	return nil
}

func (options *rootOptions) loadConfiguration() (storedConfiguration, error) {
	path, err := options.configurationPath()
	if err != nil {
		return storedConfiguration{}, err
	}
	if err := verifyCredentialFile(path); err != nil {
		return storedConfiguration{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return storedConfiguration{}, err
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxConfigSize+1))
	if err != nil {
		return storedConfiguration{}, fmt.Errorf("read ActiveCollab credentials: %w", err)
	}
	if len(data) > maxConfigSize {
		return storedConfiguration{}, errors.New("ActiveCollab credentials file is unexpectedly large")
	}
	var configuration storedConfiguration
	if err := json.Unmarshal(data, &configuration); err != nil {
		return storedConfiguration{}, fmt.Errorf("decode ActiveCollab credentials: %w", err)
	}
	configuration.URL = strings.TrimSpace(configuration.URL)
	configuration.Account = strings.TrimSpace(configuration.Account)
	configuration.Token = strings.TrimSpace(configuration.Token)
	if configuration.Version != credentialsSchemaVersion {
		return storedConfiguration{}, fmt.Errorf("unsupported ActiveCollab credentials schema version %d", configuration.Version)
	}
	if configuration.URL == "" {
		return storedConfiguration{}, errors.New("ActiveCollab credentials file has no URL")
	}
	if configuration.Token == "" {
		return storedConfiguration{}, errors.New("ActiveCollab credentials file has no token; log in again")
	}
	return configuration, nil
}

func (options *rootOptions) saveLoginCredentials(configuration storedConfiguration) error {
	configuration.Version = credentialsSchemaVersion
	configuration.URL = strings.TrimSpace(configuration.URL)
	configuration.Account = strings.TrimSpace(configuration.Account)
	configuration.Token = strings.TrimSpace(configuration.Token)
	if configuration.URL == "" || configuration.Token == "" {
		return errors.New("ActiveCollab URL and token are required before saving credentials")
	}
	return options.saveConfiguration(configuration)
}

func (options *rootOptions) saveConfiguration(configuration storedConfiguration) error {
	path, err := options.configurationPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(configuration, "", "  ")
	if err != nil {
		return fmt.Errorf("encode ActiveCollab credentials: %w", err)
	}
	data = append(data, '\n')

	directory := filepath.Dir(path)
	if err := prepareCredentialDirectory(directory); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".credentials-*")
	if err != nil {
		return fmt.Errorf("create temporary ActiveCollab credentials file: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)

	writeErr := protectCredentialPath(temporaryName, false)
	if writeErr == nil {
		_, writeErr = temporary.Write(data)
	}
	if writeErr == nil {
		writeErr = temporary.Sync()
	}
	closeErr := temporary.Close()
	if writeErr != nil {
		return fmt.Errorf("write ActiveCollab credentials: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close ActiveCollab credentials: %w", closeErr)
	}
	if err := replaceFile(temporaryName, path); err != nil {
		return fmt.Errorf("install ActiveCollab credentials: %w", err)
	}
	if err := protectCredentialPath(path, false); err != nil {
		return fmt.Errorf("protect ActiveCollab credentials: %w", err)
	}
	return nil
}

func prepareCredentialDirectory(directory string) error {
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create ActiveCollab credentials directory: %w", err)
	}
	if err := protectCredentialPath(directory, true); err != nil {
		return fmt.Errorf("protect ActiveCollab credentials directory: %w", err)
	}
	return nil
}

func verifyCredentialFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("ActiveCollab credentials path must not be a symbolic link")
	}
	if !info.Mode().IsRegular() {
		return errors.New("ActiveCollab credentials path is not a regular file")
	}
	if err := verifyCredentialProtection(path, info); err != nil {
		return fmt.Errorf("verify ActiveCollab credentials protection: %w", err)
	}
	return nil
}
