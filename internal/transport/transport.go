package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const maxErrorBody = 1 << 20

type Config struct {
	BaseURL    string
	Token      string
	UserAgent  string
	HTTPClient *http.Client
}

type Client struct {
	baseURL   *url.URL
	token     string
	userAgent string
	http      *http.Client
}

type ResponseError struct {
	StatusCode int
	Type       string
	Message    string
}

func (e *ResponseError) Error() string {
	if e.Type != "" {
		return fmt.Sprintf("ActiveCollab API returned HTTP %d (%s): %s", e.StatusCode, e.Type, e.Message)
	}
	return fmt.Sprintf("ActiveCollab API returned HTTP %d: %s", e.StatusCode, e.Message)
}

func New(config Config) (*Client, error) {
	if strings.TrimSpace(config.BaseURL) == "" {
		return nil, errors.New("ActiveCollab base URL is required")
	}
	if strings.TrimSpace(config.Token) == "" {
		return nil, errors.New("ActiveCollab API token is required")
	}
	baseURL, err := url.Parse(strings.TrimRight(config.BaseURL, "/"))
	if err != nil || baseURL.Scheme == "" || baseURL.Host == "" {
		return nil, fmt.Errorf("invalid ActiveCollab base URL %q", config.BaseURL)
	}
	if baseURL.Scheme != "https" && baseURL.Scheme != "http" {
		return nil, fmt.Errorf("unsupported ActiveCollab URL scheme %q", baseURL.Scheme)
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	httpClientCopy := *httpClient
	configuredRedirectCheck := httpClientCopy.CheckRedirect
	httpClientCopy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if !SameOrigin(req.URL, baseURL) {
			return errors.New("refusing ActiveCollab redirect to a different origin")
		}
		if configuredRedirectCheck != nil {
			return configuredRedirectCheck(req, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	return &Client{
		baseURL:   baseURL,
		token:     config.Token,
		userAgent: config.UserAgent,
		http:      &httpClientCopy,
	}, nil
}

func SameOrigin(left, right *url.URL) bool {
	return strings.EqualFold(left.Scheme, right.Scheme) &&
		strings.EqualFold(left.Hostname(), right.Hostname()) &&
		effectivePort(left) == effectivePort(right)
}

func effectivePort(value *url.URL) string {
	if port := value.Port(); port != "" {
		return port
	}
	switch strings.ToLower(value.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func (c *Client) BaseURL() *url.URL {
	copy := *c.baseURL
	return &copy
}

func (c *Client) DoJSON(ctx context.Context, method, path string, query url.Values, payload any) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode request: %w", err)
		}
		body = bytes.NewReader(encoded)
	} else if method == http.MethodPost || method == http.MethodPut {
		body = strings.NewReader("{}")
	}
	req, err := c.newRequest(ctx, method, path, query, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.do(req)
}

func (c *Client) UploadFiles(ctx context.Context, paths []string) ([]byte, error) {
	if len(paths) == 0 {
		return []byte("[]"), nil
	}
	reader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	req, err := c.newRequest(ctx, http.MethodPost, "/upload-files", nil, reader)
	if err != nil {
		_ = reader.Close()
		_ = pipeWriter.Close()
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	go func() {
		err := writeMultipartFiles(writer, paths)
		if err == nil {
			err = writer.Close()
		}
		_ = pipeWriter.CloseWithError(err)
	}()
	return c.do(req)
}

func writeMultipartFiles(writer *multipart.Writer, paths []string) error {
	for index, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open attachment %q: %w", path, err)
		}
		part, partErr := writer.CreateFormFile("attachment_"+strconv.Itoa(index+1), filepath.Base(path))
		if partErr == nil {
			_, partErr = io.Copy(part, file)
		}
		closeErr := file.Close()
		if partErr != nil {
			return fmt.Errorf("encode attachment %q: %w", path, partErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close attachment %q: %w", path, closeErr)
		}
	}
	return nil
}

func (c *Client) Download(ctx context.Context, path string) (io.ReadCloser, int64, string, error) {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, 0, "", err
	}
	response, err := c.http.Do(req)
	if err != nil {
		return nil, 0, "", c.redactError(err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		return nil, 0, "", c.responseError(response)
	}
	return response.Body, response.ContentLength, response.Header.Get("Content-Type"), nil
}

func (c *Client) newRequest(ctx context.Context, method, path string, query url.Values, body io.Reader) (*http.Request, error) {
	parsedPath, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("invalid request URL: %w", err)
	}
	requestURL := *c.baseURL
	if parsedPath.IsAbs() {
		if !SameOrigin(parsedPath, c.baseURL) {
			return nil, fmt.Errorf("refusing to send ActiveCollab credentials to a different origin")
		}
		requestURL = *parsedPath
	} else {
		requestURL.Path = strings.TrimRight(c.baseURL.Path, "/") + "/" + strings.TrimLeft(path, "/")
		requestURL.RawPath = ""
	}
	if query != nil {
		requestURL.RawQuery = query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL.String(), body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Angie-AuthApiToken", c.token)
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	return req, nil
}

func (c *Client) do(req *http.Request) ([]byte, error) {
	response, err := c.http.Do(req)
	if err != nil {
		return nil, c.redactError(err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, c.responseError(response)
	}
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read API response: %w", err)
	}
	return data, nil
}

func (c *Client) responseError(response *http.Response) error {
	data, err := io.ReadAll(io.LimitReader(response.Body, maxErrorBody))
	if err != nil {
		return &ResponseError{StatusCode: response.StatusCode, Message: "unable to read error response"}
	}
	message := strings.TrimSpace(string(data))
	var payload struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	if json.Unmarshal(data, &payload) == nil && payload.Message != "" {
		message = payload.Message
	}
	if message == "" {
		message = http.StatusText(response.StatusCode)
	}
	message = strings.ReplaceAll(message, c.token, "[REDACTED]")
	return &ResponseError{StatusCode: response.StatusCode, Type: payload.Type, Message: message}
}

func (c *Client) redactError(err error) error {
	return errors.New(strings.ReplaceAll(err.Error(), c.token, "[REDACTED]"))
}
