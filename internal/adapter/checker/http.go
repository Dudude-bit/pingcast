package checker

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
)

var _ port.MonitorChecker = (*HTTPChecker)(nil)

const (
	defaultTimeout = 10 * time.Second
	maxBodyRead    = 1 << 20 // 1MB
	maxRedirects   = 5
	userAgent      = "PingCast/1.0 (uptime monitor; https://pingcast.io)"
)

type HTTPMethod string

const (
	MethodGET  HTTPMethod = "GET"
	MethodPOST HTTPMethod = "POST"
)

type HTTPCheckConfig struct {
	URL            string     `json:"url"`
	Method         HTTPMethod `json:"method"`
	ExpectedStatus int        `json:"expected_status"`
	Keyword        *string    `json:"keyword,omitempty"`
	Timeout        int        `json:"timeout,omitempty"`    // seconds, 0 = use default
	BodyLimit      int        `json:"body_limit,omitempty"` // bytes, 0 = use default 1MB
	UserAgent      string     `json:"user_agent,omitempty"` // empty = use default
}

type HTTPChecker struct {
	httpClient *http.Client
}

func NewHTTPChecker() *HTTPChecker {
	return NewHTTPCheckerWithTimeout(10)
}

func NewHTTPCheckerWithTimeout(timeoutSeconds int) *HTTPChecker {
	return &HTTPChecker{
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= maxRedirects {
					return fmt.Errorf("too many redirects (max %d)", maxRedirects)
				}
				return nil
			},
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{},
			},
		},
	}
}

func (c *HTTPChecker) Check(ctx context.Context, monitor *domain.Monitor) *domain.CheckResult {
	start := time.Now()
	result := &domain.CheckResult{
		MonitorID: monitor.ID,
		CheckedAt: start,
	}

	var cfg HTTPCheckConfig
	if err := json.Unmarshal(monitor.CheckConfig, &cfg); err != nil {
		errMsg := fmt.Sprintf("invalid http config: %s", err)
		result.Status = domain.StatusDown
		result.ErrorMessage = &errMsg
		result.ResponseTimeMs = int(time.Since(start).Milliseconds())
		return result
	}

	method := string(cfg.Method)
	if method == "" {
		method = "GET"
	}
	expectedStatus := cfg.ExpectedStatus
	if expectedStatus == 0 {
		expectedStatus = 200
	}

	req, err := http.NewRequestWithContext(ctx, method, cfg.URL, nil)
	if err != nil {
		errMsg := fmt.Sprintf("invalid request: %s", err)
		result.Status = domain.StatusDown
		result.ErrorMessage = &errMsg
		result.ResponseTimeMs = int(time.Since(start).Milliseconds())
		return result
	}

	ua := userAgent
	if cfg.UserAgent != "" {
		ua = cfg.UserAgent
	}
	req.Header.Set("User-Agent", ua)

	client := c.httpClient
	if cfg.Timeout > 0 {
		client = &http.Client{
			Timeout:       time.Duration(cfg.Timeout) * time.Second,
			CheckRedirect: c.httpClient.CheckRedirect,
			Transport:     c.httpClient.Transport,
		}
	}

	resp, err := client.Do(req)
	result.ResponseTimeMs = int(time.Since(start).Milliseconds())

	if err != nil {
		errMsg := classifyError(err)
		result.Status = domain.StatusDown
		result.ErrorMessage = &errMsg
		return result
	}
	defer resp.Body.Close()

	statusCode := resp.StatusCode
	result.StatusCode = &statusCode

	if resp.StatusCode != expectedStatus {
		errMsg := fmt.Sprintf("expected status %d, got %d", expectedStatus, resp.StatusCode)
		result.Status = domain.StatusDown
		result.ErrorMessage = &errMsg
		return result
	}

	if cfg.Keyword != nil && *cfg.Keyword != "" {
		bodyLimit := int64(maxBodyRead)
		if cfg.BodyLimit > 0 {
			bodyLimit = int64(cfg.BodyLimit)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, bodyLimit))
		if err != nil {
			errMsg := fmt.Sprintf("failed to read body: %s", err)
			result.Status = domain.StatusDown
			result.ErrorMessage = &errMsg
			return result
		}
		if !strings.Contains(string(body), *cfg.Keyword) {
			errMsg := fmt.Sprintf("keyword %q not found in response body", *cfg.Keyword)
			result.Status = domain.StatusDown
			result.ErrorMessage = &errMsg
			return result
		}
	}

	result.Status = domain.StatusUp
	return result
}

func (c *HTTPChecker) ValidateConfig(raw json.RawMessage) error {
	var cfg HTTPCheckConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("invalid http config: %w", err)
	}
	if cfg.URL == "" {
		return fmt.Errorf("url required")
	}
	if _, err := url.Parse(cfg.URL); err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	return nil
}

func (c *HTTPChecker) ConfigSchema() port.ConfigSchema {
	return port.ConfigSchema{Fields: []port.ConfigField{
		{Name: "url", Label: "URL", Type: "text", Required: true, Placeholder: "https://example.com/health"},
		{Name: "method", Label: "Method", Type: "select", Default: "GET", Options: []port.Option{
			{Value: "GET", Label: "GET"}, {Value: "POST", Label: "POST"},
		}},
		{Name: "expected_status", Label: "Expected Status", Type: "number", Default: 200},
		{Name: "keyword", Label: "Keyword", Type: "text", Placeholder: "optional"},
	}}
}

func (c *HTTPChecker) Target(raw json.RawMessage) (string, error) {
	var cfg HTTPCheckConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return "", fmt.Errorf("invalid http config: %w", err)
	}
	method := string(cfg.Method)
	if method == "" {
		method = "GET"
	}
	return method + " " + cfg.URL, nil
}

func (c *HTTPChecker) Host(raw json.RawMessage) (string, error) {
	var cfg HTTPCheckConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return "", fmt.Errorf("invalid http config: %w", err)
	}
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}
	return u.Host, nil
}

func classifyError(err error) string {
	errStr := err.Error()
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		return "timeout"
	}
	if strings.Contains(errStr, "tls") || strings.Contains(errStr, "certificate") || strings.Contains(errStr, "x509") {
		return fmt.Sprintf("TLS error: %s", errStr)
	}
	if strings.Contains(errStr, "no such host") {
		return "DNS resolution failed"
	}
	if strings.Contains(errStr, "connection refused") {
		return "connection refused"
	}
	return errStr
}
