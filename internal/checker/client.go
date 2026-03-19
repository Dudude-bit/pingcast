package checker

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	defaultTimeout = 10 * time.Second
	maxBodyRead    = 1 << 20 // 1MB
	maxRedirects   = 5
	userAgent      = "PingCast/1.0 (uptime monitor; https://pingcast.io)"
)

type MonitorInfo struct {
	ID                 uuid.UUID
	URL                string
	Method             string
	IntervalSeconds    int
	ExpectedStatus     int
	Keyword            *string
	AlertAfterFailures int
}

type CheckResult struct {
	MonitorID      uuid.UUID
	Status         string // "up" or "down"
	StatusCode     *int32
	ResponseTimeMs int
	ErrorMessage   *string
	CheckedAt      time.Time
}

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return NewClientWithTimeout(10)
}

func NewClientWithTimeout(timeoutSeconds int) *Client {
	return &Client{
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

func (c *Client) Check(ctx context.Context, monitor *MonitorInfo) *CheckResult {
	start := time.Now()
	result := &CheckResult{
		MonitorID: monitor.ID,
		CheckedAt: start,
	}

	req, err := http.NewRequestWithContext(ctx, monitor.Method, monitor.URL, nil)
	if err != nil {
		errMsg := fmt.Sprintf("invalid request: %s", err)
		result.Status = "down"
		result.ErrorMessage = &errMsg
		result.ResponseTimeMs = int(time.Since(start).Milliseconds())
		return result
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	result.ResponseTimeMs = int(time.Since(start).Milliseconds())

	if err != nil {
		errMsg := classifyError(err)
		result.Status = "down"
		result.ErrorMessage = &errMsg
		return result
	}
	defer resp.Body.Close()

	statusCode := int32(resp.StatusCode)
	result.StatusCode = &statusCode

	if resp.StatusCode != monitor.ExpectedStatus {
		errMsg := fmt.Sprintf("expected status %d, got %d", monitor.ExpectedStatus, resp.StatusCode)
		result.Status = "down"
		result.ErrorMessage = &errMsg
		return result
	}

	if monitor.Keyword != nil && *monitor.Keyword != "" {
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyRead))
		if err != nil {
			errMsg := fmt.Sprintf("failed to read body: %s", err)
			result.Status = "down"
			result.ErrorMessage = &errMsg
			return result
		}
		if !strings.Contains(string(body), *monitor.Keyword) {
			errMsg := fmt.Sprintf("keyword %q not found in response body", *monitor.Keyword)
			result.Status = "down"
			result.ErrorMessage = &errMsg
			return result
		}
	}

	result.Status = "up"
	return result
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
