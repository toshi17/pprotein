package summary

import (
	"fmt"
	"io"
	"net/http"

	"github.com/goccy/go-json"
	"github.com/kaz/pprotein/internal/collect"
)

type client struct {
	baseURL string
}

func (c *client) list(typ string) ([]*collect.Entry, error) {
	resp, err := http.DefaultClient.Get(c.baseURL + "/api/" + typ)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	entries := []*collect.Entry{}
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("failed to decode: %w", err)
	}
	return entries, nil
}

func (c *client) text(path string) (string, error) {
	resp, err := http.DefaultClient.Get(c.baseURL + path)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d: %s", resp.StatusCode, body)
	}
	return string(body), nil
}
