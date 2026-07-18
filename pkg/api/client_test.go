package api

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func testResponse(status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("")),
	}
}

func testServer(handler http.HandlerFunc) (*httptest.Server, *Client) {
	srv := httptest.NewServer(handler)
	client := NewClient("test-key",
		WithHTTPClient(srv.Client()),
		WithLimiter(rate.NewLimiter(rate.Inf, 1)),
		WithRetryConfig(RetryConfig{
			MaxRetries:     1,
			InitialBackoff: 1 * time.Millisecond,
			MaxBackoff:     10 * time.Millisecond,
		}),
	)
	// Override base URL
	client.baseURL = srv.URL
	return srv, client
}

func TestListStations(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing or wrong authorization header")
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Error("missing accept header")
		}
		if r.URL.Path != "/api/v1/cli/stations" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		json.NewEncoder(w).Encode(StationsResponse{
			Data: []Station{
				{UUID: "uuid-1", Name: "Station One"},
				{UUID: "uuid-2", Name: "Station Two"},
			},
		})
	})
	defer srv.Close()

	stations, err := client.ListStations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(stations) != 2 {
		t.Fatalf("expected 2 stations, got %d", len(stations))
	}
	if stations[0].Name != "Station One" {
		t.Errorf("expected Station One, got %s", stations[0].Name)
	}
}

func TestListStationsDirectArray(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]Station{
			{UUID: "uuid-1", Name: "Station One"},
		})
	})
	defer srv.Close()

	stations, err := client.ListStations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(stations) != 1 {
		t.Fatalf("expected 1 station, got %d", len(stations))
	}
}

func Test401Error(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]string{"message": "Invalid API key"})
	})
	defer srv.Close()

	_, err := client.ListStations(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	ae := AsAPIError(err)
	if ae == nil {
		t.Fatal("expected APIError")
	}
	if ae.Code != ErrAuthFailed {
		t.Errorf("expected ErrAuthFailed, got %s", ae.Code)
	}
	if ae.HTTPStatus != 401 {
		t.Errorf("expected 401, got %d", ae.HTTPStatus)
	}
}

func Test422ErrorPreservesServerMessage(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Unsupported audio codec"})
	})
	defer srv.Close()

	_, err := client.ListStations(context.Background())
	ae := AsAPIError(err)
	if ae == nil {
		t.Fatal("expected API error")
	}
	if ae.Message != "Unsupported audio codec" {
		t.Fatalf("expected server message, got %q", ae.Message)
	}
}

func Test403Error(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(map[string]string{"message": "Forbidden"})
	})
	defer srv.Close()

	_, err := client.ListStations(context.Background())
	ae := AsAPIError(err)
	if ae == nil || ae.Code != ErrAuthForbidden {
		t.Errorf("expected ErrAuthForbidden, got %v", ae)
	}
}

func Test404Error(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})
	defer srv.Close()

	_, err := client.ListStations(context.Background())
	ae := AsAPIError(err)
	if ae == nil || ae.Code != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", ae)
	}
}

func Test429RetryThenSuccess(t *testing.T) {
	attempts := 0
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}
		json.NewEncoder(w).Encode(StationsResponse{Data: []Station{{UUID: "u1", Name: "S1"}}})
	})
	defer srv.Close()

	client.retryCfg.MaxRetries = 2
	client.retryCfg.InitialBackoff = 1 * time.Millisecond

	stations, err := client.ListStations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(stations) != 1 {
		t.Errorf("expected 1 station, got %d", len(stations))
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func Test429RetryExhausted(t *testing.T) {
	attempts := 0
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(429)
	})
	defer srv.Close()

	client.retryCfg.MaxRetries = 2
	client.retryCfg.InitialBackoff = 1 * time.Millisecond

	_, err := client.ListStations(context.Background())
	if err == nil {
		t.Fatal("expected error after retry exhaustion")
	}
	if attempts != 3 { // initial + 2 retries
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestListFolders(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		expected := "/api/v1/cli/stations/station-uuid/media/folders"
		if r.URL.Path != expected {
			t.Errorf("expected %s, got %s", expected, r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": []string{"Folder 1", "Folder 2"},
		})
	})
	defer srv.Close()

	folders, err := client.ListFolders(context.Background(), "station-uuid")
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 2 {
		t.Fatalf("expected 2 folders, got %d", len(folders))
	}
	if folders[0].Name != "Folder 1" {
		t.Errorf("expected Folder 1, got %s", folders[0].Name)
	}
}

func TestCreateFolder(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var req CreateFolderRequest
		json.Unmarshal(body, &req)
		if req.Name != "New Folder" {
			t.Errorf("expected New Folder, got %s", req.Name)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"message": "Folder created.",
			"data":    map[string]string{"folder": "New Folder"},
		})
	})
	defer srv.Close()

	folder, err := client.CreateFolder(context.Background(), "station-uuid", "New Folder")
	if err != nil {
		t.Fatal(err)
	}
	if folder.Name != "New Folder" {
		t.Errorf("expected New Folder, got %s", folder.Name)
	}
}

func TestListMedia(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") != "1" {
			t.Errorf("expected page=1")
		}
		if r.URL.Query().Get("folder") != "High Rotation" {
			t.Errorf("expected folder query")
		}
		json.NewEncoder(w).Encode(MediaResponse{
			Data: []MediaItem{
				{
					UUID:             "track-1",
					OriginalFilename: "song.mp3",
					Filename:         "song.mp3",
					SizeBytes:        1000,
					DurationSeconds:  120,
					Folder:           "High Rotation",
				},
			},
			Meta: PaginationMeta{
				CurrentPage: 1,
				LastPage:    1,
				PerPage:     50,
				Total:       1,
			},
		})
	})
	defer srv.Close()

	resp, err := client.ListMedia(context.Background(), "station-uuid", MediaListParams{
		Page:    1,
		PerPage: 50,
		Folder:  "High Rotation",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 media item, got %d", len(resp.Data))
	}
	if resp.Data[0].DisplayFilename() != "song.mp3" {
		t.Errorf("expected song.mp3, got %s", resp.Data[0].DisplayFilename())
	}
	if resp.Data[0].DisplaySize() != 1000 {
		t.Errorf("expected size 1000, got %d", resp.Data[0].DisplaySize())
	}
	if resp.Meta.CurrentPage != 1 {
		t.Errorf("expected page 1, got %d", resp.Meta.CurrentPage)
	}
}

func TestListMediaDirectArray(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]MediaItem{
			{ID: 1, Filename: "song.mp3", Size: 1000},
		})
	})
	defer srv.Close()

	resp, err := client.ListMedia(context.Background(), "station-uuid", MediaListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 media item, got %d", len(resp.Data))
	}
}

func TestUploadMedia(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.mp3")
	os.WriteFile(filePath, []byte("fake audio content"), 0644)

	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		expected := "/api/v1/cli/stations/station-uuid/media"
		if r.URL.Path != expected {
			t.Errorf("expected %s, got %s", expected, r.URL.Path)
		}

		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "multipart/form-data") {
			t.Errorf("expected multipart, got %s", ct)
		}

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatal(err)
		}

		file, handler, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if handler.Filename != "test.mp3" {
			t.Errorf("expected test.mp3, got %s", handler.Filename)
		}

		folder := r.FormValue("folder")
		if folder != "Music" {
			t.Errorf("expected folder Music, got %s", folder)
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": MediaItem{
				ID:       1,
				Filename: "test.mp3",
				Folder:   "Music",
				Size:     17,
			},
		})
	})
	defer srv.Close()

	result, err := client.UploadMedia(context.Background(), "station-uuid", UploadMediaInput{
		FilePath: filePath,
		Folder:   "Music",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("upload failed: %s", result.Error)
	}
	if result.Media == nil || result.Media.Filename != "test.mp3" {
		t.Errorf("unexpected media result: %+v", result.Media)
	}
}

func TestUploadMediaWithJingle(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "jingle.mp3")
	os.WriteFile(filePath, []byte("jingle content"), 0644)

	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(10 << 20)

		if r.FormValue("is_jingle") != "true" {
			t.Error("expected is_jingle=true")
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": MediaItem{ID: 1, Filename: "jingle.mp3", IsJingle: true},
		})
	})
	defer srv.Close()

	result, _ := client.UploadMedia(context.Background(), "station-uuid", UploadMediaInput{
		FilePath: filePath,
		IsJingle: true,
	})
	if !result.Success {
		t.Fatal("upload should succeed")
	}
	if result.Media == nil || !result.Media.IsJingle {
		t.Error("expected jingle media")
	}
}

func TestUploadMediaRootNoFolder(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "root.mp3")
	os.WriteFile(filePath, []byte("root content"), 0644)

	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(10 << 20)

		folder := r.FormValue("folder")
		if folder != "" {
			t.Errorf("expected empty folder for root upload, got %q", folder)
		}

		jingle := r.FormValue("is_jingle")
		if jingle != "" {
			t.Errorf("expected no jingle for root upload, got %q", jingle)
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": MediaItem{ID: 1, Filename: "root.mp3"},
		})
	})
	defer srv.Close()

	result, _ := client.UploadMedia(context.Background(), "station-uuid", UploadMediaInput{
		FilePath: filePath,
	})
	if !result.Success {
		t.Fatal("upload should succeed")
	}
}

func TestUploadMediaStreamedRetriesWithFreshBody(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "retry.mp3")
	if err := os.WriteFile(filePath, []byte("complete audio payload"), 0644); err != nil {
		t.Fatal(err)
	}

	var attempts int
	var idempotencyKey string
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			t.Error("missing Idempotency-Key header")
		}
		if idempotencyKey == "" {
			idempotencyKey = key
		} else if key != idempotencyKey {
			t.Errorf("idempotency key changed between attempts: %q != %q", key, idempotencyKey)
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			t.Errorf("attempt %d has invalid multipart body: %v", attempts, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		payload, err := io.ReadAll(file)
		_ = file.Close()
		if err != nil {
			t.Fatal(err)
		}
		if string(payload) != "complete audio payload" {
			t.Errorf("attempt %d sent %q", attempts, string(payload))
		}

		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": MediaItem{ID: 1, Filename: "retry.mp3"},
		})
	})
	defer srv.Close()

	result, err := client.UploadMediaStreamed(context.Background(), "station-uuid", UploadMediaInput{FilePath: filePath})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("upload failed: %s", result.Error)
	}
	if attempts != 2 {
		t.Fatalf("expected two attempts, got %d", attempts)
	}
}

func TestUploadMediaStreamedDoesNotRetryServerFailure(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "failure.mp3")
	if err := os.WriteFile(filePath, []byte("audio"), 0644); err != nil {
		t.Fatal(err)
	}

	attempts := 0
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer srv.Close()

	result, err := client.UploadMediaStreamed(context.Background(), "station-uuid", UploadMediaInput{FilePath: filePath})
	if err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Fatal("upload should fail")
	}
	if attempts != 1 {
		t.Fatalf("non-idempotent server failure was retried %d times", attempts)
	}
}

func TestBearerHeaderEveryRequest(t *testing.T) {
	reqCount := 0
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		reqCount++
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("request %d: missing auth header", reqCount)
		}
		w.WriteHeader(429)
	})
	defer srv.Close()

	client.retryCfg.MaxRetries = 2
	client.retryCfg.InitialBackoff = 1 * time.Millisecond

	client.ListStations(context.Background())
	if reqCount < 2 {
		t.Errorf("expected at least 2 requests, got %d", reqCount)
	}
}

func TestNetworkFailure(t *testing.T) {
	client := NewClient("test-key",
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("synthetic network failure")
		})}),
		WithLimiter(rate.NewLimiter(rate.Inf, 1)),
		WithRetryConfig(RetryConfig{MaxRetries: 0}),
	)

	_, err := client.ListStations(context.Background())
	if err == nil {
		t.Fatal("expected network error")
	}
	ae := AsAPIError(err)
	if ae == nil || ae.Code != ErrNetwork {
		t.Errorf("expected ErrNetwork, got %v", ae)
	}
}

func TestTLSCertificateFailureIsSpecificAndNotRetried(t *testing.T) {
	attempts := 0
	client := NewClient("test-key",
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			attempts++
			return nil, x509.UnknownAuthorityError{}
		})}),
		WithLimiter(rate.NewLimiter(rate.Inf, 1)),
		WithRetryConfig(RetryConfig{MaxRetries: 5}),
	)

	_, err := client.ListStations(context.Background())
	apiErr := AsAPIError(err)
	if apiErr == nil {
		t.Fatal("expected API error")
	}
	if apiErr.Code != ErrTLSCertificate {
		t.Fatalf("expected %s, got %s", ErrTLSCertificate, apiErr.Code)
	}
	if !strings.Contains(strings.ToLower(apiErr.FriendlyMessage()), "certificate") {
		t.Fatalf("friendly message does not explain certificate failure: %q", apiErr.FriendlyMessage())
	}
	if attempts != 1 {
		t.Fatalf("certificate failure should not be retried, got %d attempts", attempts)
	}
}

func TestHTTPSWithoutTLSServerIsSpecific(t *testing.T) {
	client := NewClient("test-key",
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("http: server gave HTTP response to HTTPS client")
		})}),
		WithLimiter(rate.NewLimiter(rate.Inf, 1)),
	)

	_, err := client.ListStations(context.Background())
	apiErr := AsAPIError(err)
	if apiErr == nil || apiErr.Code != ErrTLSCertificate {
		t.Fatalf("expected TLS-specific error, got %v", err)
	}
}

func TestAsAPIErrorFindsWrappedError(t *testing.T) {
	original := NewAPIError(ErrTLSCertificate, 0, "certificate failed", x509.UnknownAuthorityError{})
	wrapper := fmt.Errorf("request wrapper: %w", original)
	if got := AsAPIError(wrapper); got != original {
		t.Fatalf("expected wrapped API error, got %#v", got)
	}
}

func TestDefaultClientTimeoutSupportsLongUploads(t *testing.T) {
	client := NewClient("test-key")
	if client.httpClient.Timeout <= 60*time.Second {
		t.Fatalf("default timeout must exceed 60 seconds, got %s", client.httpClient.Timeout)
	}
}

func TestFriendlyErrorMessage(t *testing.T) {
	err := NewAPIError(ErrAuthFailed, 401, "invalid", nil)
	msg := err.FriendlyMessage()
	if msg == "" {
		t.Error("expected non-empty friendly message")
	}
	if strings.Contains(msg, "test-key") {
		t.Error("friendly message should not contain API key")
	}
}

func TestKeyNotInError(t *testing.T) {
	err := NewAPIError(ErrAuthFailed, 401, "invalid key", fmt.Errorf("underlying"))
	errStr := err.Error()
	if strings.Contains(errStr, "test-key") {
		t.Error("error string should not contain API key")
	}
}

func TestRateLimiterShared(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(StationsResponse{Data: []Station{}})
	})
	defer srv.Close()

	ctx := context.Background()
	done := make(chan struct{})

	go func() {
		client.ListStations(ctx)
		client.ListStations(ctx)
		client.ListStations(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("rate limiter appears to be blocking")
	}
}
