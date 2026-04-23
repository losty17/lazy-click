package clickup

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type BackendOAuthConfig struct {
	BackendURL string
	ClientID   string
}

type backendOAuthBeginResponse struct {
	AuthURL   string `json:"auth_url"`
	SessionID string `json:"session_id"`
}

type backendOAuthCompleteResponse struct {
	AccessToken string `json:"access_token"`
	Provider    string `json:"provider"`
}

func OAuthTokenViaBackend(ctx context.Context, cfg BackendOAuthConfig) (string, error) {
	backendURL := strings.TrimSpace(cfg.BackendURL)
	if backendURL == "" {
		return "", fmt.Errorf("oauth backend url is required")
	}
	base, err := url.Parse(backendURL)
	if err != nil {
		return "", fmt.Errorf("invalid oauth backend url: %w", err)
	}

	beginURL := base.ResolveReference(&url.URL{Path: "/oauth/clickup/begin"})
	beginReq, err := http.NewRequestWithContext(ctx, http.MethodGet, beginURL.String(), nil)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(cfg.ClientID) != "" {
		beginReq.Header.Set("X-ClickUp-Client-ID", strings.TrimSpace(cfg.ClientID))
	}

	beginRes, err := http.DefaultClient.Do(beginReq)
	if err != nil {
		return "", err
	}
	defer beginRes.Body.Close()
	if beginRes.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("oauth begin failed: %s", beginRes.Status)
	}
	var beginPayload backendOAuthBeginResponse
	if err := json.NewDecoder(beginRes.Body).Decode(&beginPayload); err != nil {
		return "", err
	}
	if strings.TrimSpace(beginPayload.AuthURL) == "" {
		return "", fmt.Errorf("oauth begin response missing auth_url")
	}
	if strings.TrimSpace(beginPayload.SessionID) == "" {
		return "", fmt.Errorf("oauth begin response missing session_id")
	}
	if err := openBrowser(beginPayload.AuthURL); err != nil {
		return "", fmt.Errorf("failed to open browser: %w", err)
	}

	completeURL := base.ResolveReference(&url.URL{Path: "/oauth/clickup/complete"})
	query := completeURL.Query()
	query.Set("session_id", beginPayload.SessionID)
	completeURL.RawQuery = query.Encode()
	deadline := time.Now().Add(3 * time.Minute)
	for {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("oauth completion timed out")
		}
		completeReq, err := http.NewRequestWithContext(ctx, http.MethodGet, completeURL.String(), nil)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(cfg.ClientID) != "" {
			completeReq.Header.Set("X-ClickUp-Client-ID", strings.TrimSpace(cfg.ClientID))
		}
		completeRes, err := http.DefaultClient.Do(completeReq)
		if err != nil {
			return "", err
		}
		if completeRes.StatusCode == http.StatusAccepted {
			completeRes.Body.Close()
			time.Sleep(1200 * time.Millisecond)
			continue
		}
		if completeRes.StatusCode >= http.StatusBadRequest {
			completeRes.Body.Close()
			return "", fmt.Errorf("oauth complete failed: %s", completeRes.Status)
		}
		var done backendOAuthCompleteResponse
		err = json.NewDecoder(completeRes.Body).Decode(&done)
		completeRes.Body.Close()
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(done.AccessToken) == "" {
			return "", fmt.Errorf("oauth complete response missing access_token")
		}
		return done.AccessToken, nil
	}
}
