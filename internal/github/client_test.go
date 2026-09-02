package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return New("test-token", srv.URL, "mkgo-test")
}

func TestViewerSendsAuthHeaders(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("X-GitHub-Api-Version"); got != apiVersion {
			t.Errorf("X-GitHub-Api-Version = %q", got)
		}
		if r.URL.Path != "/user" {
			t.Errorf("path = %q, want /user", r.URL.Path)
		}
		json.NewEncoder(w).Encode(User{Login: "octocat", Name: "The Octocat"})
	})

	user, err := client.Viewer(context.Background())
	if err != nil {
		t.Fatalf("Viewer returned an error: %v", err)
	}
	if user.Login != "octocat" {
		t.Errorf("login = %q, want octocat", user.Login)
	}
}

func TestCreateRepositoryForUser(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/repos" {
			t.Errorf("path = %q, want /user/repos", r.URL.Path)
		}
		var body createRepoPayload
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Name != "demo" || !body.Private {
			t.Errorf("unexpected payload %+v", body)
		}
		if body.AutoInit {
			t.Error("auto_init must stay false: mkgo pushes its own initial commit")
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"name":       "demo",
			"full_name":  "octocat/demo",
			"private":    true,
			"html_url":   "https://github.com/octocat/demo",
			"clone_url":  "https://github.com/octocat/demo.git",
			"ssh_url":    "git@github.com:octocat/demo.git",
			"owner":      map[string]string{"login": "octocat"},
			"default_br": "main",
		})
	})

	repo, err := client.CreateRepository(context.Background(), CreateRepoOptions{Name: "demo", Private: true})
	if err != nil {
		t.Fatalf("CreateRepository returned an error: %v", err)
	}
	if repo.FullName != "octocat/demo" {
		t.Errorf("full name = %q", repo.FullName)
	}
	if repo.SSHURL != "git@github.com:octocat/demo.git" {
		t.Errorf("ssh url = %q", repo.SSHURL)
	}
}

func TestCreateRepositoryInOrganization(t *testing.T) {
	var gotPath string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Repository{Name: "demo", FullName: "acme/demo"})
	})

	if _, err := client.CreateRepository(context.Background(), CreateRepoOptions{Name: "demo", Org: "acme"}); err != nil {
		t.Fatalf("CreateRepository returned an error: %v", err)
	}
	if gotPath != "/orgs/acme/repos" {
		t.Errorf("path = %q, want /orgs/acme/repos", gotPath)
	}
}

func TestCreateRepositorySetsTopics(t *testing.T) {
	var topics []string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{
				"name":  "demo",
				"owner": map[string]string{"login": "octocat"},
			})
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/topics"):
			var body struct {
				Names []string `json:"names"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			topics = body.Names
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{}"))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})

	if _, err := client.CreateRepository(context.Background(), CreateRepoOptions{
		Name:   "demo",
		Topics: []string{"go", "cli"},
	}); err != nil {
		t.Fatalf("CreateRepository returned an error: %v", err)
	}
	if len(topics) != 2 || topics[0] != "go" {
		t.Errorf("topics = %v", topics)
	}
}

func TestRepoExists(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/taken") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{}"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Not Found"}`))
	})

	exists, err := client.RepoExists(context.Background(), "octocat", "taken")
	if err != nil || !exists {
		t.Errorf("expected taken to exist (exists=%v, err=%v)", exists, err)
	}
	exists, err = client.RepoExists(context.Background(), "octocat", "free")
	if err != nil || exists {
		t.Errorf("expected free to be available (exists=%v, err=%v)", exists, err)
	}
}

func TestAPIErrorCarriesDetails(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"message":"Repository creation failed.","errors":[{"resource":"Repository","field":"name","code":"custom","message":"name already exists on this account"}]}`))
	})

	_, err := client.CreateRepository(context.Background(), CreateRepoOptions{Name: "demo"})
	if err == nil {
		t.Fatal("expected an error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected an *APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d", apiErr.StatusCode)
	}
	if msg := HumanizeError(err); !strings.Contains(msg, "already exists") {
		t.Errorf("HumanizeError = %q", msg)
	}
}

func TestRequestWithoutTokenFails(t *testing.T) {
	client := New("", "https://example.invalid", "mkgo-test")
	if _, err := client.Viewer(context.Background()); !errors.Is(err, ErrNoToken) {
		t.Fatalf("expected ErrNoToken, got %v", err)
	}
	if msg := HumanizeError(ErrNoToken); !strings.Contains(msg, "GITHUB_TOKEN") {
		t.Errorf("HumanizeError = %q", msg)
	}
}

func TestHumanizeUnauthorized(t *testing.T) {
	err := &APIError{StatusCode: http.StatusUnauthorized, Message: "Bad credentials"}
	if msg := HumanizeError(err); !strings.Contains(msg, "401") {
		t.Errorf("HumanizeError = %q", msg)
	}
}
