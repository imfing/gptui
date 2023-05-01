package rest

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_NewRequest(t *testing.T) {
	baseURL := "http://localhost:8080"
	path := "/api/test"

	client := NewClient(WithBaseURL(baseURL))
	req, err := client.NewRequest(path)

	assert.NoError(t, err)
	assert.NotNil(t, req)
	assert.Equal(t, http.MethodGet, req.Method)
	assert.Equal(t, baseURL+path, req.URL.String())
}

func TestClient_Do(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, world!"))
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	req, err := client.NewRequest("/")

	assert.NoError(t, err)

	resp, err := client.Do(req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)

	assert.NoError(t, err)
	assert.Equal(t, "Hello, world!", string(body))
}

func TestClientOptions(t *testing.T) {
	timeout := 3 * time.Second
	baseURL := "http://localhost:8080"

	client := NewClient(WithTimeout(timeout), WithBaseURL(baseURL))

	assert.Equal(t, timeout, client.httpClient.Timeout)
	assert.Equal(t, baseURL, client.baseURL)
}

func TestRequestOptions(t *testing.T) {
	baseURL := "http://localhost:8080"
	path := "/api/test"
	method := http.MethodPost
	bodyContent := "test body"
	body := bytes.NewBufferString(bodyContent)
	header := http.Header{"Content-Type": []string{"application/json"}}

	client := NewClient(WithBaseURL(baseURL))
	req, err := client.NewRequest(path, WithMethod(method), WithBody(body), WithHeader(header))

	assert.NoError(t, err)
	assert.NotNil(t, req)
	assert.Equal(t, method, req.Method)

	// Read the body content from io.NopCloser
	reqBodyContent, err := io.ReadAll(req.Body)
	assert.NoError(t, err)
	assert.NotNil(t, req)
	assert.Equal(t, bodyContent, string(reqBodyContent))
	assert.Equal(t, method, req.Method)
	assert.Equal(t, header, req.Header)
}
