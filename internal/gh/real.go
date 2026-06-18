package gh

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const apiBase = "https://api.github.com"

// Client is the real, read-only GitHub metadata client.
type Client struct {
	token string
	http  *http.Client
}

// New returns a metadata client using the given token (may be empty).
func New(token string) *Client {
	return &Client{token: token, http: &http.Client{Timeout: 15 * time.Second}}
}

// Available reports whether a token is configured.
func (c *Client) Available() bool { return c.token != "" }

func (c *Client) get(ctx context.Context, path string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+path, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("github request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body, nil
}

// Verify confirms the token authenticates.
func (c *Client) Verify(ctx context.Context) error {
	if !c.Available() {
		return fmt.Errorf("no token configured")
	}
	code, _, err := c.get(ctx, "/user")
	if err != nil {
		return err
	}
	if code != http.StatusOK {
		return fmt.Errorf("token verify failed (HTTP %d)", code)
	}
	return nil
}

// Login returns the authenticated user's login.
func (c *Client) Login(ctx context.Context) (string, error) {
	code, body, err := c.get(ctx, "/user")
	if err != nil {
		return "", err
	}
	if code != http.StatusOK {
		return "", fmt.Errorf("github /user: HTTP %d", code)
	}
	var u struct {
		Login string `json:"login"`
	}
	if err := json.Unmarshal(body, &u); err != nil {
		return "", fmt.Errorf("parse /user: %w", err)
	}
	return u.Login, nil
}

// RepoExists reports whether owner/repo is visible to the token.
func (c *Client) RepoExists(ctx context.Context, owner, repo string) (bool, error) {
	code, _, err := c.get(ctx, "/repos/"+owner+"/"+repo)
	if err != nil {
		return false, err
	}
	switch code {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("github repo lookup: HTTP %d", code)
	}
}

// RepoPublic reports whether owner/repo is public. A 404 (private or missing,
// when seen anonymously) is reported as not-public without error.
func (c *Client) RepoPublic(ctx context.Context, owner, repo string) (bool, error) {
	code, body, err := c.get(ctx, "/repos/"+owner+"/"+repo)
	if err != nil {
		return false, err
	}
	switch code {
	case http.StatusOK:
		var r struct {
			Private bool `json:"private"`
		}
		if err := json.Unmarshal(body, &r); err != nil {
			return false, fmt.Errorf("parse repo: %w", err)
		}
		return !r.Private, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("github repo lookup: HTTP %d", code)
	}
}
