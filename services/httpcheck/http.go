package httpcheck

import (
	"context"
	"net/http"
	"time"
)

const defaultProbeTimeout = time.Second

// HTTPOptions configures ProbeHTTP.
type HTTPOptions struct {
	Client  *http.Client
	Timeout time.Duration
	Now     func() time.Time
}

// HTTPResult captures the dashboard-relevant outcome of an HTTP probe.
type HTTPResult struct {
	URL        string
	StatusCode int
	Class      string
	Latency    time.Duration
}

// ProbeHTTP performs a GET request and returns status class plus
// latency. The default timeout is one second per Sprint 02 TASK-02.7.
func ProbeHTTP(ctx context.Context, url string, opts HTTPOptions) (HTTPResult, error) {
	opts = normalizeHTTPOptions(opts)
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	start := opts.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return HTTPResult{}, err
	}
	resp, err := opts.Client.Do(req)
	if err != nil {
		return HTTPResult{}, err
	}
	defer resp.Body.Close()

	return HTTPResult{
		URL:        url,
		StatusCode: resp.StatusCode,
		Class:      statusClass(resp.StatusCode),
		Latency:    opts.Now().Sub(start),
	}, nil
}

func normalizeHTTPOptions(opts HTTPOptions) HTTPOptions {
	if opts.Timeout <= 0 {
		opts.Timeout = defaultProbeTimeout
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Client == nil {
		opts.Client = &http.Client{
			Timeout: opts.Timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	return opts
}

func statusClass(statusCode int) string {
	switch {
	case statusCode >= 200 && statusCode <= 299:
		return "2xx"
	case statusCode >= 300 && statusCode <= 399:
		return "3xx"
	case statusCode >= 400 && statusCode <= 499:
		return "4xx"
	case statusCode >= 500 && statusCode <= 599:
		return "5xx"
	default:
		return "unknown"
	}
}
