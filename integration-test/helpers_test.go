package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-json"
)

const (
	// Base settings.
	host     = "app"
	attempts = 20

	// Attempts connection.
	httpURL        = "http://" + host + ":8080"
	healthPath     = httpURL + "/healthz"
	requestTimeout = 5 * time.Second

	// HTTP REST.
	basePathV1 = httpURL + "/v1"

	// Test password used across helpers.
	testPassword = "testpass123"
)

var errHealthCheck = fmt.Errorf("url %s is not available", healthPath)

// doWebRequestWithTimeout sends an HTTP request with a Content-Type of application/json.
func doWebRequestWithTimeout(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	return http.DefaultClient.Do(req)
}

// doAuthenticatedRequest sends an HTTP request with a Bearer token.
func doAuthenticatedRequest(ctx context.Context, method, url string, body io.Reader, token string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	return http.DefaultClient.Do(req)
}

// registerUser registers a new user via the HTTP REST API.
func registerUser(t *testing.T, username, email, password string) *http.Response {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	defer cancel()

	body := fmt.Sprintf(`{"username":%q,"email":%q,"password":%q}`, username, email, password)

	resp, err := doWebRequestWithTimeout(ctx, http.MethodPost, basePathV1+"/auth/register", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("registerUser: failed to send request: %v", err)
	}

	return resp
}

// loginUser logs in a user and returns the JWT token.
func loginUser(t *testing.T, email, password string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	defer cancel()

	body := fmt.Sprintf(`{"email":%q,"password":%q}`, email, password)

	resp, err := doWebRequestWithTimeout(ctx, http.MethodPost, basePathV1+"/auth/login", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("loginUser: failed to send request: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("loginUser: expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var result struct {
		Token string `json:"token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("loginUser: failed to decode response: %v", err)
	}

	return result.Token
}

// sanitizeTestName converts t.Name() into a safe string for use as a username.
func sanitizeTestName(t *testing.T) string {
	t.Helper()

	name := t.Name()
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ToLower(name)

	return name
}

// registerAndLogin creates a unique user via HTTP and returns the JWT token.
func registerAndLogin(t *testing.T) string {
	t.Helper()

	name := sanitizeTestName(t)
	email := name + "@test.com"
	password := testPassword

	resp := registerUser(t, name, email, password)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("registerAndLogin: register expected 201, got %d", resp.StatusCode)
	}

	return loginUser(t, email, password)
}

// parseJSON is a generic JSON parser for HTTP responses.
func parseJSON[T any](t *testing.T, resp *http.Response) T {
	t.Helper()

	var result T

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("parseJSON: failed to decode response: %v", err)
	}

	return result
}

func getHealthCheck(url string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)

	defer cancel()

	resp, err := doWebRequestWithTimeout(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return -1, err
	}

	defer resp.Body.Close()

	return resp.StatusCode, nil
}

func healthCheck(attempts int) error {
	for attempts > 0 {
		statusCode, err := getHealthCheck(healthPath)
		if err != nil {
			return err
		}

		if statusCode == http.StatusOK {
			return nil
		}

		log.Printf("Integration tests: url %s is not available, attempts left: %d", healthPath, attempts)

		time.Sleep(time.Second)

		attempts--
	}

	return errHealthCheck
}

func TestMain(m *testing.M) {
	err := healthCheck(attempts)
	if err != nil {
		log.Fatalf("Integration tests: httpURL %s is not available: %s", httpURL, err)
	}

	log.Printf("Integration tests: httpURL %s is available", httpURL)

	code := m.Run()
	os.Exit(code)
}
