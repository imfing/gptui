package rest

import (
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client is a simple HTTP REST client
type Client struct {
	httpClient *http.Client
	baseURL    string
}

type ClientOption func(*Client)

func NewClient(opts ...ClientOption) *Client {
	client := &Client{
		httpClient: &http.Client{},
		baseURL:    "",
	}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

func (c *Client) NewRequest(path string, opts ...RequestOption) (*http.Request, error) {
	reqURL, err := url.JoinPath(c.baseURL, path)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	for _, opt := range opts {
		opt(req)
	}
	return req, nil
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// RequestOption is a function that operates on a http.Request.
type RequestOption func(*http.Request)

// WithMethod sets the HTTP method for the request.
func WithMethod(method string) RequestOption {
	return func(req *http.Request) {
		req.Method = method
	}
}

// WithBody sets the body for the request.
func WithBody(body io.Reader) RequestOption {
	return func(req *http.Request) {
		req.Body = io.NopCloser(body)
	}
}

// WithHeader sets header for the request.
func WithHeader(header http.Header) RequestOption {
	return func(req *http.Request) {
		req.Header = header
	}
}
