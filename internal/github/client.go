// Package github talks to the GitHub REST API directly over HTTP. It
// deliberately does not shell out to the gh CLI, so mkgo works on any machine
// that has a token, with nothing else installed.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL is the public GitHub API endpoint.
const DefaultBaseURL = "https://api.github.com"

// apiVersion pins the REST API version so responses stay stable.
const apiVersion = "2022-11-28"

// Client is a minimal GitHub REST client covering the endpoints mkgo needs.
type Client struct {
	BaseURL    string
	Token      string
	UserAgent  string
	HTTPClient *http.Client
}

// New returns a client for the given token and base URL. An empty baseURL
// falls back to the public API.
func New(token, baseURL, userAgent string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if userAgent == "" {
		userAgent = "mkgo"
	}
	return &Client{
		BaseURL:   strings.TrimSuffix(baseURL, "/"),
		Token:     token,
		UserAgent: userAgent,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// User is the subset of the authenticated user we care about.
type User struct {
	Login string `json:"login"`
	Name  string `json:"name"`
	Type  string `json:"type"`
}

// Repository is the subset of a created repository we report back.
type Repository struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	HTMLURL       string `json:"html_url"`
	CloneURL      string `json:"clone_url"`
	SSHURL        string `json:"ssh_url"`
	DefaultBranch string `json:"default_branch"`
	Owner         struct {
		Login string `json:"login"`
	} `json:"owner"`
}

// CreateRepoOptions describes the repository to create.
type CreateRepoOptions struct {
	Name        string
	Description string
	Homepage    string
	Private     bool
	Org         string // when set, the repo is created inside this organization
	AutoInit    bool
	HasIssues   bool
	HasWiki     bool
	HasProjects bool
	Topics      []string
}

// createRepoPayload is the JSON body sent to GitHub.
type createRepoPayload struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Homepage    string `json:"homepage,omitempty"`
	Private     bool   `json:"private"`
	AutoInit    bool   `json:"auto_init"`
	HasIssues   bool   `json:"has_issues"`
	HasWiki     bool   `json:"has_wiki"`
	HasProjects bool   `json:"has_projects"`
}

// APIError carries the status code and message returned by GitHub.
type APIError struct {
	StatusCode int
	Message    string
	Errors     []string
	DocURL     string
}

func (e *APIError) Error() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("github api: %d", e.StatusCode))
	if e.Message != "" {
		b.WriteString(": " + e.Message)
	}
	for _, detail := range e.Errors {
		b.WriteString("; " + detail)
	}
	return b.String()
}

// ErrNoToken is returned when no credential could be resolved.
var ErrNoToken = errors.New("no GitHub token found")

// errorResponse mirrors GitHub's error envelope.
type errorResponse struct {
	Message string `json:"message"`
	DocURL  string `json:"documentation_url"`
	Errors  []struct {
		Resource string `json:"resource"`
		Field    string `json:"field"`
		Code     string `json:"code"`
		Message  string `json:"message"`
	} `json:"errors"`
}

// do performs a request and decodes the response into out.
func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	if c.Token == "" {
		return ErrNoToken
	}

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("User-Agent", c.UserAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request %s %s: %w", method, path, redactErr(err, c.Token))
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 300 {
		return parseAPIError(resp, payload)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// parseAPIError turns a non-2xx response into an *APIError with the most
// actionable message we can extract.
func parseAPIError(resp *http.Response, payload []byte) error {
	apiErr := &APIError{StatusCode: resp.StatusCode}
	var body errorResponse
	if err := json.Unmarshal(payload, &body); err == nil {
		apiErr.Message = body.Message
		apiErr.DocURL = body.DocURL
		for _, e := range body.Errors {
			detail := e.Message
			if detail == "" {
				detail = strings.TrimSpace(e.Field + " " + e.Code)
			}
			if detail != "" {
				apiErr.Errors = append(apiErr.Errors, detail)
			}
		}
	}
	if apiErr.Message == "" {
		apiErr.Message = strings.TrimSpace(string(payload))
	}
	if resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0" {
		apiErr.Message = "rate limit exceeded; try again later"
	}
	return apiErr
}

// redactErr keeps a token out of error strings, which end up on screen.
func redactErr(err error, token string) error {
	if token == "" || err == nil {
		return err
	}
	msg := strings.ReplaceAll(err.Error(), token, "***")
	return errors.New(msg)
}

// Viewer returns the authenticated user, which doubles as a token check.
func (c *Client) Viewer(ctx context.Context) (*User, error) {
	var u User
	if err := c.do(ctx, http.MethodGet, "/user", nil, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// RepoExists reports whether owner/name is already taken.
func (c *Client) RepoExists(ctx context.Context, owner, name string) (bool, error) {
	err := c.do(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s", owner, name), nil, nil)
	if err == nil {
		return true, nil
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return false, err
}

// CreateRepository creates a repository for the authenticated user, or inside
// an organization when opts.Org is set.
func (c *Client) CreateRepository(ctx context.Context, opts CreateRepoOptions) (*Repository, error) {
	path := "/user/repos"
	if opts.Org != "" {
		path = "/orgs/" + opts.Org + "/repos"
	}
	payload := createRepoPayload{
		Name:        opts.Name,
		Description: opts.Description,
		Homepage:    opts.Homepage,
		Private:     opts.Private,
		AutoInit:    opts.AutoInit,
		HasIssues:   opts.HasIssues,
		HasWiki:     opts.HasWiki,
		HasProjects: opts.HasProjects,
	}
	var repo Repository
	if err := c.do(ctx, http.MethodPost, path, payload, &repo); err != nil {
		return nil, err
	}
	if len(opts.Topics) > 0 {
		if err := c.ReplaceTopics(ctx, repo.Owner.Login, repo.Name, opts.Topics); err != nil {
			// Topics are cosmetic; report but do not fail the whole run.
			return &repo, fmt.Errorf("set topics: %w", err)
		}
	}
	return &repo, nil
}

// ReplaceTopics overwrites the repository topic list.
func (c *Client) ReplaceTopics(ctx context.Context, owner, repo string, topics []string) error {
	body := struct {
		Names []string `json:"names"`
	}{Names: topics}
	return c.do(ctx, http.MethodPut, fmt.Sprintf("/repos/%s/%s/topics", owner, repo), body, nil)
}

// HumanizeError turns an API failure into a hint a user can act on.
func HumanizeError(err error) string {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		if errors.Is(err, ErrNoToken) {
			return "set GITHUB_TOKEN (or pass --token) to let mkgo create the repository"
		}
		return err.Error()
	}
	switch apiErr.StatusCode {
	case http.StatusUnauthorized:
		return "GitHub rejected the token (401); check that it is valid and not expired"
	case http.StatusForbidden:
		return "GitHub refused the request (403): " + apiErr.Message
	case http.StatusNotFound:
		return "not found (404); the token may lack the 'repo' scope or the organization does not exist"
	case http.StatusUnprocessableEntity:
		msg := apiErr.Message
		if len(apiErr.Errors) > 0 {
			msg = strings.Join(apiErr.Errors, "; ")
		}
		return "GitHub could not create the repository: " + msg
	default:
		return apiErr.Error()
	}
}
