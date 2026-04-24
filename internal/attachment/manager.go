package attachment

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

type Manager struct {
	basePath string
	mu       sync.Mutex
}

func NewManager(basePath string) (*Manager, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, err
	}
	return &Manager{basePath: basePath}, nil
}

func (m *Manager) GetLocalPath(id string, filename string) string {
	return filepath.Join(m.basePath, id+"_"+filename)
}

func (m *Manager) Download(ctx context.Context, id, filename, url string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	localPath := m.GetLocalPath(id, filename)
	if _, err := os.Stat(localPath); err == nil {
		return localPath, nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download attachment: %s", resp.Status)
	}

	out, err := os.Create(localPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}

	return localPath, nil
}
