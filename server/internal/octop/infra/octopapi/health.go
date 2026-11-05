package octopapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Result struct {
	OK         bool
	StatusCode int
	Error      string
}

// Check 探测 Octop HTTP 健康。优先 GET {base}/api/health，不能只凭端口通判定。
func Check(ctx context.Context, apiURL string, timeout time.Duration) Result {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	base := strings.TrimRight(strings.TrimSpace(apiURL), "/")
	if base == "" {
		return Result{Error: "api url empty"}
	}
	endpoint := base + "/api/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Result{Error: err.Error()}
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return Result{Error: err.Error()}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return Result{OK: true, StatusCode: resp.StatusCode}
	}
	return Result{StatusCode: resp.StatusCode, Error: fmt.Sprintf("http %d", resp.StatusCode)}
}
