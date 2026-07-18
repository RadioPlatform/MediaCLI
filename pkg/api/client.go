package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
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

	"golang.org/x/time/rate"
)

const APIBaseURL = "https://radioplatform.streamafrica.cloud"

const defaultRequestTimeout = 30 * time.Minute

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	limiter    *rate.Limiter
	retryCfg   RetryConfig
	clock      Clock
}

type ClientOption func(*Client)

func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = hc
	}
}

func WithLimiter(l *rate.Limiter) ClientOption {
	return func(c *Client) {
		c.limiter = l
	}
}

func WithRetryConfig(cfg RetryConfig) ClientOption {
	return func(c *Client) {
		c.retryCfg = cfg
	}
}

func WithClock(clock Clock) ClientOption {
	return func(c *Client) {
		c.clock = clock
	}
}

func NewClient(apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL:    APIBaseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: defaultRequestTimeout},
		limiter:    rate.NewLimiter(rate.Every(time.Second), 1),
		retryCfg:   DefaultRetryConfig,
		clock:      RealClock{},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) BaseURL() string {
	return c.baseURL
}

func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader, contentType string, headers http.Header) (*http.Response, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, NewAPIError(ErrNetwork, 0, "rate limiter context cancelled", err)
	}

	u := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, NewAPIError(ErrNetwork, 0, "failed to create request", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for name, values := range headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	return c.httpClient.Do(req)
}

func (c *Client) doRequestWithRetry(ctx context.Context, method, path string, body io.Reader, contentType string) (*http.Response, error) {
	var factory requestBodyFactory
	if body != nil {
		if seeker, ok := body.(io.Seeker); ok {
			factory = func() (io.Reader, error) {
				if _, err := seeker.Seek(0, io.SeekStart); err != nil {
					return nil, err
				}
				return body, nil
			}
		} else {
			used := false
			factory = func() (io.Reader, error) {
				if used {
					return nil, fmt.Errorf("request body cannot be replayed")
				}
				used = true
				return body, nil
			}
		}
	}

	return c.doRequestWithRetryFactory(ctx, method, path, factory, contentType, nil)
}

type requestBodyFactory func() (io.Reader, error)

func (c *Client) doRequestWithRetryFactory(ctx context.Context, method, path string, bodyFactory requestBodyFactory, contentType string, headers http.Header) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= c.retryCfg.MaxRetries; attempt++ {
		var bodyReader io.Reader
		if bodyFactory != nil {
			var err error
			bodyReader, err = bodyFactory()
			if err != nil {
				return nil, NewAPIError(ErrNetwork, 0, "failed to prepare request body", err)
			}
		}

		resp, err := c.doRequest(ctx, method, path, bodyReader, contentType, headers)
		if err != nil {
			if isTLSCertificateError(err) {
				return nil, NewAPIError(ErrTLSCertificate, 0, "TLS certificate verification failed", err)
			}
			lastErr = NewAPIError(ErrNetwork, 0, "request failed", err)
			if attempt < c.retryCfg.MaxRetries && ctx.Err() == nil && isIdempotentMethod(method) {
				if err := c.waitForRetry(ctx, backoff(attempt, c.retryCfg)); err != nil {
					return nil, err
				}
				continue
			}
			return nil, lastErr
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}

		if resp.StatusCode == 401 {
			message := extractErrorMessage(resp)
			resp.Body.Close()
			return nil, NewAPIError(ErrAuthFailed, 401, message, nil)
		}

		if resp.StatusCode == 403 {
			message := extractErrorMessage(resp)
			resp.Body.Close()
			return nil, NewAPIError(ErrAuthForbidden, 403, message, nil)
		}

		if resp.StatusCode == 404 {
			message := extractErrorMessage(resp)
			resp.Body.Close()
			return nil, NewAPIError(ErrNotFound, 404, message, nil)
		}

		if resp.StatusCode == 422 {
			message := extractErrorMessage(resp)
			resp.Body.Close()
			return nil, NewAPIError(ErrValidation, 422, message, nil)
		}

		if isRetryableStatusCode(resp.StatusCode) {
			retryAfter := parseRetryAfter(resp)
			statusCode := resp.StatusCode
			resp.Body.Close()

			if attempt < c.retryCfg.MaxRetries && (statusCode == http.StatusTooManyRequests || isIdempotentMethod(method)) {
				wait := retryAfter
				if wait == 0 {
					wait = backoff(attempt, c.retryCfg)
				}
				if err := c.waitForRetry(ctx, wait); err != nil {
					return nil, err
				}
				continue
			}

			if statusCode == http.StatusTooManyRequests {
				return nil, NewAPIError(ErrRateLimited, statusCode, "rate limit exceeded after retries", nil)
			}
			return nil, NewAPIError(ErrUnknown, statusCode, fmt.Sprintf("server returned status %d", statusCode), nil)
		}

		if resp.StatusCode == 400 {
			bodyBytes, rerr := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			if rerr != nil {
				return nil, NewAPIError(ErrValidation, 400, "bad request", nil)
			}
			return nil, NewAPIError(ErrValidation, 400, string(bodyBytes), nil)
		}

		resp.Body.Close()
		return nil, NewAPIError(ErrUnknown, resp.StatusCode, fmt.Sprintf("unexpected status: %d", resp.StatusCode), nil)
	}

	return nil, lastErr
}

func (c *Client) waitForRetry(ctx context.Context, wait time.Duration) error {
	if c.clock == nil {
		return nil
	}
	select {
	case <-c.clock.After(wait):
		return nil
	case <-ctx.Done():
		return NewAPIError(ErrNetwork, 0, "retry cancelled", ctx.Err())
	}
}

func isIdempotentMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPut, http.MethodDelete:
		return true
	default:
		return false
	}
}

func extractErrorMessage(resp *http.Response) string {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if err != nil {
		return ""
	}

	var envelope struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if json.Unmarshal(body, &envelope) == nil {
		if envelope.Message != "" {
			return envelope.Message
		}
		if envelope.Error != "" {
			return envelope.Error
		}
	}

	return strings.TrimSpace(string(body))
}

func decodeJSON[T any](resp *http.Response) (*T, error) {
	defer resp.Body.Close()
	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, NewAPIError(ErrInvalidResponse, resp.StatusCode, "failed to decode response", err)
	}
	return &result, nil
}

func readBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
}

func (c *Client) ListStations(ctx context.Context) ([]Station, error) {
	resp, err := c.doRequestWithRetry(ctx, "GET", "/api/v1/cli/stations", nil, "")
	if err != nil {
		return nil, err
	}

	body, err := readBody(resp)
	if err != nil {
		return nil, NewAPIError(ErrInvalidResponse, resp.StatusCode, "failed to read response", err)
	}

	var wrapper StationsResponse
	if err := json.Unmarshal(body, &wrapper); err == nil {
		return wrapper.Data, nil
	}

	var arr []Station
	if err := json.Unmarshal(body, &arr); err == nil {
		return arr, nil
	}

	return nil, NewAPIError(ErrInvalidResponse, resp.StatusCode, "failed to decode response", nil)
}

func (c *Client) ListFolders(ctx context.Context, stationUUID string) ([]Folder, error) {
	path := fmt.Sprintf("/api/v1/cli/stations/%s/media/folders", stationUUID)
	resp, err := c.doRequestWithRetry(ctx, "GET", path, nil, "")
	if err != nil {
		return nil, err
	}

	body, err := readBody(resp)
	if err != nil {
		return nil, NewAPIError(ErrInvalidResponse, resp.StatusCode, "failed to read response", err)
	}

	var stringWrapper struct {
		Data []string `json:"data"`
	}
	if err := json.Unmarshal(body, &stringWrapper); err == nil && stringWrapper.Data != nil {
		folders := make([]Folder, 0, len(stringWrapper.Data))
		for _, name := range stringWrapper.Data {
			folders = append(folders, Folder{Name: name})
		}
		return folders, nil
	}

	var names []string
	if err := json.Unmarshal(body, &names); err == nil {
		folders := make([]Folder, 0, len(names))
		for _, name := range names {
			folders = append(folders, Folder{Name: name})
		}
		return folders, nil
	}

	var wrapper FoldersResponse
	if err := json.Unmarshal(body, &wrapper); err == nil {
		return wrapper.Data, nil
	}

	var arr []Folder
	if err := json.Unmarshal(body, &arr); err == nil {
		return arr, nil
	}

	return nil, NewAPIError(ErrInvalidResponse, resp.StatusCode, "failed to decode response", nil)
}

func (c *Client) CreateFolder(ctx context.Context, stationUUID, name string) (*Folder, error) {
	path := fmt.Sprintf("/api/v1/cli/stations/%s/media/folders", stationUUID)
	body := CreateFolderRequest{Name: name}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, NewAPIError(ErrInvalidResponse, 0, "failed to marshal request", err)
	}

	resp, err := c.doRequestWithRetry(ctx, "POST", path, bytes.NewReader(jsonBody), "application/json")
	if err != nil {
		return nil, err
	}

	wrapper, err := decodeJSON[CreateFolderResponse](resp)
	if err != nil {
		return nil, err
	}

	folderName := wrapper.Data.Folder
	if folderName == "" {
		folderName = name
	}

	return &Folder{Name: folderName}, nil
}

type MediaListParams struct {
	Page    int
	PerPage int
	Folder  string
	Search  string
}

func (c *Client) ListMedia(ctx context.Context, stationUUID string, params MediaListParams) (*MediaResponse, error) {
	path := fmt.Sprintf("/api/v1/cli/stations/%s/media", stationUUID)
	q := url.Values{}
	if params.Page > 0 {
		q.Set("page", strconv.Itoa(params.Page))
	}
	if params.PerPage > 0 {
		q.Set("per_page", strconv.Itoa(params.PerPage))
	}
	if params.Folder != "" {
		q.Set("folder", params.Folder)
	}
	if params.Search != "" {
		q.Set("search", params.Search)
	}
	if len(q) > 0 {
		path += "?" + q.Encode()
	}

	resp, err := c.doRequestWithRetry(ctx, "GET", path, nil, "")
	if err != nil {
		return nil, err
	}

	body, err := readBody(resp)
	if err != nil {
		return nil, NewAPIError(ErrInvalidResponse, resp.StatusCode, "failed to read response", err)
	}

	var wrapper MediaResponse
	if err := json.Unmarshal(body, &wrapper); err == nil {
		return &wrapper, nil
	}

	var arr []MediaItem
	if err := json.Unmarshal(body, &arr); err == nil {
		return &MediaResponse{Data: arr}, nil
	}

	return nil, NewAPIError(ErrInvalidResponse, resp.StatusCode, "failed to decode response", nil)
}

type UploadMediaInput struct {
	FilePath string
	Folder   string
	IsJingle bool
}

func (c *Client) UploadMedia(ctx context.Context, stationUUID string, input UploadMediaInput) (*UploadResult, error) {
	file, err := os.Open(input.FilePath)
	if err != nil {
		return &UploadResult{
			Success: false,
			Error:   fmt.Sprintf("Could not read the local file: %s", err.Error()),
		}, nil
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(input.FilePath))
	if err != nil {
		return &UploadResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to create multipart form: %s", err.Error()),
		}, nil
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return &UploadResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to read file: %s", err.Error()),
		}, nil
	}

	if input.Folder != "" {
		_ = writer.WriteField("folder", input.Folder)
	}
	if input.IsJingle {
		_ = writer.WriteField("is_jingle", "true")
	}

	writer.Close()

	path := fmt.Sprintf("/api/v1/cli/stations/%s/media", stationUUID)

	resp, err := c.doRequestWithRetry(ctx, "POST", path, bytes.NewReader(body.Bytes()), writer.FormDataContentType())
	if err != nil {
		// Check if it's a validation error from the API
		if ae := AsAPIError(err); ae != nil {
			return &UploadResult{
				Success: false,
				Error:   ae.FriendlyMessage(),
			}, nil
		}
		return &UploadResult{
			Success: false,
			Error:   fmt.Sprintf("Upload failed: %s", err.Error()),
		}, nil
	}

	var mediaData struct {
		Data *MediaItem `json:"data,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&mediaData); err != nil {
		resp.Body.Close()
		return &UploadResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to decode response: %s", err.Error()),
		}, nil
	}
	resp.Body.Close()

	media := mediaData.Data
	if media == nil {
		media = &MediaItem{}
	}
	return &UploadResult{
		Success: true,
		Media:   media,
	}, nil
}

func (c *Client) UploadMediaStreamed(ctx context.Context, stationUUID string, input UploadMediaInput) (*UploadResult, error) {
	if _, err := os.Stat(input.FilePath); err != nil {
		return &UploadResult{
			Success: false,
			Error:   fmt.Sprintf("Could not read the local file: %s", err.Error()),
		}, nil
	}

	prototype := multipart.NewWriter(io.Discard)
	boundary := prototype.Boundary()
	contentType := prototype.FormDataContentType()
	_ = prototype.Close()

	bodyFactory := func() (io.Reader, error) {
		return newMultipartUploadBody(input, boundary)
	}
	idempotencyKey, err := newIdempotencyKey()
	if err != nil {
		return nil, NewAPIError(ErrUploadFailed, 0, "failed to create upload idempotency key", err)
	}
	headers := make(http.Header)
	headers.Set("Idempotency-Key", idempotencyKey)
	path := fmt.Sprintf("/api/v1/cli/stations/%s/media", stationUUID)

	resp, err := c.doRequestWithRetryFactory(ctx, "POST", path, bodyFactory, contentType, headers)
	if err != nil {
		message := err.Error()
		if apiErr := AsAPIError(err); apiErr != nil {
			message = apiErr.FriendlyMessage()
		}
		return &UploadResult{
			Success: false,
			Error:   message,
		}, nil
	}
	defer resp.Body.Close()

	var mediaData struct {
		Data *MediaItem `json:"data,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&mediaData); err != nil {
		return &UploadResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to decode response: %s", err.Error()),
		}, nil
	}

	media := mediaData.Data
	if media == nil {
		media = &MediaItem{}
	}
	return &UploadResult{
		Success: true,
		Media:   media,
	}, nil
}

func newMultipartUploadBody(input UploadMediaInput, boundary string) (io.Reader, error) {
	file, err := os.Open(input.FilePath)
	if err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()
	go func() {
		defer file.Close()

		writer := multipart.NewWriter(pw)
		if err := writer.SetBoundary(boundary); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		part, err := writer.CreateFormFile("file", filepath.Base(input.FilePath))
		if err == nil {
			_, err = io.Copy(part, file)
		}
		if err == nil && input.Folder != "" {
			err = writer.WriteField("folder", input.Folder)
		}
		if err == nil && input.IsJingle {
			err = writer.WriteField("is_jingle", "true")
		}
		if err == nil {
			err = writer.Close()
		}
		_ = pw.CloseWithError(err)
	}()

	return pr, nil
}

func newIdempotencyKey() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
