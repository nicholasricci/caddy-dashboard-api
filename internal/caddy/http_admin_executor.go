package caddy

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nicholasricci/caddy-dashboard/internal/secrets"
)

// HTTPAdminExecutor talks to the Caddy admin API over HTTP(S).
type HTTPAdminExecutor struct {
	resolver *secrets.MultiResolver
	client   *http.Client
	maxBody  int64
}

// NewHTTPAdminExecutor builds an executor. maxBody limits response size for GET /config/.
func NewHTTPAdminExecutor(resolver *secrets.MultiResolver, timeout time.Duration, maxBody int64) *HTTPAdminExecutor {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	if maxBody <= 0 {
		maxBody = 32 << 20
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.MaxIdleConnsPerHost = 4
	return &HTTPAdminExecutor{
		resolver: resolver,
		maxBody:  maxBody,
		client: &http.Client{
			Timeout:   timeout,
			Transport: tr,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (e *HTTPAdminExecutor) ApplyConfig(ctx context.Context, t ExecTarget, payload []byte) (*ExecutionResult, error) {
	if e == nil || t.HTTP == nil {
		return nil, ErrTransportNotConfigured
	}
	u := t.HTTP.BaseURL + "/load"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if err := e.applyAuth(ctx, t, req); err != nil {
		return nil, err
	}
	return e.do(req, t.HTTP.TLSSkipVerify, false)
}

func (e *HTTPAdminExecutor) Reload(_ context.Context, _ ExecTarget) (*ExecutionResult, error) {
	return nil, ErrTransportUnsupportedOp
}

func (e *HTTPAdminExecutor) RunCommand(_ context.Context, _ ExecTarget, _ string) (*ExecutionResult, error) {
	return nil, ErrTransportUnsupportedOp
}

func (e *HTTPAdminExecutor) FetchConfig(ctx context.Context, t ExecTarget) (*ExecutionResult, error) {
	if e == nil || t.HTTP == nil {
		return nil, ErrTransportNotConfigured
	}
	u := t.HTTP.BaseURL + "/config/"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	if err := e.applyAuth(ctx, t, req); err != nil {
		return nil, err
	}
	return e.do(req, t.HTTP.TLSSkipVerify, true)
}

func (e *HTTPAdminExecutor) applyAuth(ctx context.Context, t ExecTarget, req *http.Request) error {
	ref := strings.TrimSpace(t.HTTP.BearerTokenRef)
	if ref == "" {
		return nil
	}
	if e.resolver == nil {
		return fmt.Errorf("%w: bearer_token_ref requires secret resolver", ErrTransportNotConfigured)
	}
	b, err := e.resolver.Resolve(ctx, ref)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(b)))
	return nil
}

func (e *HTTPAdminExecutor) do(req *http.Request, tlsSkipVerify, isGet bool) (*ExecutionResult, error) {
	client := e.client
	if tlsSkipVerify {
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
		client = &http.Client{
			Timeout:   e.client.Timeout,
			Transport: tr,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTransportUnreachable, err)
	}
	defer resp.Body.Close()

	var bodyReader io.Reader = resp.Body
	if isGet && e.maxBody > 0 {
		bodyReader = io.LimitReader(resp.Body, e.maxBody)
	}
	body, err := io.ReadAll(bodyReader)
	if err != nil {
		return nil, fmt.Errorf("%w: read body: %v", ErrTransportUnreachable, err)
	}

	meta := map[string]string{"http_status": fmt.Sprintf("%d", resp.StatusCode)}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return executionSuccess(string(body), meta), nil
	}
	stderr := strings.TrimSpace(string(body))
	if len(stderr) > 2048 {
		stderr = stderr[:2048] + "…"
	}
	res := &ExecutionResult{
		Status: ExecStatusFailed,
		Stderr: fmt.Sprintf("http %d: %s", resp.StatusCode, stderr),
		Meta:   meta,
	}
	return res, nil
}
