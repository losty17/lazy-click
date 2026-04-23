package clickup

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type oauthTokenResponse struct {
	AccessToken string `json:"access_token"`
}

func OAuthToken(ctx context.Context, cfg OAuthConfig) (string, error) {
	if strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.ClientSecret) == "" {
		return "", fmt.Errorf("clickup oauth requires CLICKUP_CLIENT_ID and CLICKUP_CLIENT_SECRET")
	}
	redirectURL := strings.TrimSpace(cfg.RedirectURL)
	if redirectURL == "" {
		redirectURL = "http://127.0.0.1:45713/callback"
	}
	parsedRedirect, err := url.Parse(redirectURL)
	if err != nil {
		return "", fmt.Errorf("invalid oauth redirect url: %w", err)
	}
	if parsedRedirect.Host == "" {
		return "", fmt.Errorf("oauth redirect url must include host")
	}

	state, err := randomState(24)
	if err != nil {
		return "", err
	}
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	mux := http.NewServeMux()
	server := &http.Server{Addr: parsedRedirect.Host, Handler: mux}

	mux.HandleFunc(parsedRedirect.Path, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			errCh <- fmt.Errorf("oauth state mismatch")
			http.Error(w, "OAuth state mismatch", http.StatusBadRequest)
			return
		}
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if code == "" {
			errCh <- fmt.Errorf("oauth callback missing code")
			http.Error(w, "OAuth callback missing code", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte("ClickUp connected. You can close this window and return to lazy-click."))
		select {
		case codeCh <- code:
		default:
		}
	})

	go func() {
		if serveErr := server.ListenAndServe(); serveErr != nil && serveErr != http.ErrServerClosed {
			errCh <- serveErr
		}
	}()

	params := url.Values{}
	params.Set("client_id", cfg.ClientID)
	params.Set("redirect_uri", redirectURL)
	params.Set("response_type", "code")
	params.Set("state", state)
	authURL := "https://app.clickup.com/api?" + params.Encode()
	if openErr := openBrowser(authURL); openErr != nil {
		_ = server.Shutdown(context.Background())
		return "", fmt.Errorf("failed to open browser: %w", openErr)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	var code string
	select {
	case <-timeoutCtx.Done():
		_ = server.Shutdown(context.Background())
		return "", fmt.Errorf("oauth authorization timed out")
	case serveErr := <-errCh:
		_ = server.Shutdown(context.Background())
		return "", serveErr
	case code = <-codeCh:
	}

	_ = server.Shutdown(context.Background())
	return exchangeOAuthCode(timeoutCtx, cfg.ClientID, cfg.ClientSecret, code)
}

func exchangeOAuthCode(ctx context.Context, clientID string, clientSecret string, code string) (string, error) {
	payload := map[string]string{
		"client_id":     clientID,
		"client_secret": clientSecret,
		"code":          code,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.clickup.com/api/v2/oauth/token", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	if res.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("clickup oauth token exchange failed: %d %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}
	var parsed oauthTokenResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	if strings.TrimSpace(parsed.AccessToken) == "" {
		return "", fmt.Errorf("clickup oauth did not return an access token")
	}
	return parsed.AccessToken, nil
}

func randomState(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func openBrowser(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Start()
}
