package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
)

func executeCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)

	root.SetOut(buf)
	root.SetErr(buf)
	root.SetIn(bytes.NewBuffer(nil))
	root.SetArgs(args)

	err := root.Execute()
	return buf.String(), err
}

func TestRootCmd(t *testing.T) {
	out, err := executeCommand(rootCmd)
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, out, "Gorse command line tool for cluster management")
}

func TestLoginCmd(t *testing.T) {
	sets := make(map[string]string)
	restoreKeyring := mockKeyring(t,
		func(service, user string) (string, error) { return "", keyring.ErrNotFound },
		func(service, user, password string) error {
			sets[user] = password
			return nil
		},
		func(service, user string) error { return nil },
	)
	defer restoreKeyring()

	out, err := executeCommand(rootCmd, "login", "--endpoint", "http://localhost:8088/api", "--api-key", "secret")
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, out, "login")
	assertContains(t, out, "Credentials saved")
	if sets[keyringEndpointKey] != "http://localhost:8088/api" || sets[keyringAPIKeyKey] != "secret" {
		t.Fatalf("unexpected saved credentials: %#v", sets)
	}
}

func TestLogoutCmd(t *testing.T) {
	deleted := make(map[string]bool)
	restoreKeyring := mockKeyring(t,
		func(service, user string) (string, error) { return "", keyring.ErrNotFound },
		func(service, user, password string) error { return nil },
		func(service, user string) error {
			deleted[user] = true
			return nil
		},
	)
	defer restoreKeyring()

	out, err := executeCommand(rootCmd, "logout")
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, out, "logout")
	assertContains(t, out, "Credentials removed")
	if !deleted[keyringEndpointKey] || !deleted[keyringAPIKeyKey] {
		t.Fatalf("unexpected deleted credentials: %#v", deleted)
	}
}

func TestGetCmd(t *testing.T) {
	out, err := executeCommand(rootCmd, "get")
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, out, "Get resources from Gorse admin API")
}

func TestGetSubcommands(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantPath     string
		wantRawQuery []string
		wantInOutput string
	}{
		{name: "userinfo", args: []string{"get", "userinfo"}, wantPath: "/dashboard/userinfo", wantInOutput: "admin@example.com"},
		{name: "cluster", args: []string{"get", "cluster"}, wantPath: "/dashboard/cluster", wantInOutput: "server-node"},
		{name: "categories", args: []string{"get", "categories"}, wantPath: "/dashboard/categories", wantInOutput: "books"},
		{name: "tasks", args: []string{"get", "tasks"}, wantPath: "/dashboard/tasks", wantInOutput: "Load Data"},
		{name: "config", args: []string{"get", "config"}, wantPath: "/dashboard/config", wantInOutput: "cache_size"},
		{name: "config-schema", args: []string{"get", "config-schema"}, wantPath: "/dashboard/config/schema", wantInOutput: "object"},
		{name: "stats", args: []string{"get", "stats"}, wantPath: "/dashboard/stats", wantInOutput: "binary_version"},
		{name: "timeseries", args: []string{"get", "timeseries", "requests", "--begin", "2026-01-01", "--end", "2026-01-02", "--duration", "1h"}, wantPath: "/dashboard/timeseries/requests", wantRawQuery: []string{"begin=2026-01-01", "end=2026-01-02", "duration=1h"}, wantInOutput: "2026-01-01"},
		{name: "user", args: []string{"get", "user", "alice"}, wantPath: "/dashboard/user/alice", wantInOutput: "alice"},
		{name: "users", args: []string{"get", "users", "--n", "10", "--cursor", "next"}, wantPath: "/dashboard/users", wantRawQuery: []string{"n=10", "cursor=next"}, wantInOutput: "bob"},
		{name: "user-feedback", args: []string{"get", "user-feedback", "alice", "click", "--n", "5", "--offset", "2"}, wantPath: "/dashboard/user/alice/feedback/click", wantRawQuery: []string{"n=5", "offset=2"}, wantInOutput: "item-1"},
		{name: "latest", args: []string{"get", "latest", "--n", "3", "--offset", "1", "--category", "news", "--category", "tech"}, wantPath: "/dashboard/latest", wantRawQuery: []string{"n=3", "offset=1", "category=news", "category=tech"}, wantInOutput: "latest-1"},
		{name: "non-personalized", args: []string{"get", "non-personalized", "popular", "--n", "3", "--offset", "1", "--user-id", "alice", "--category", "news"}, wantPath: "/dashboard/non-personalized/popular", wantRawQuery: []string{"n=3", "offset=1", "user-id=alice", "category=news"}, wantInOutput: "popular-1"},
		{name: "recommend", args: []string{"get", "recommend", "alice", "non-personalized", "popular", "--n", "3", "--category", "news"}, wantPath: "/dashboard/recommend/alice/non-personalized/popular", wantRawQuery: []string{"n=3", "category=news"}, wantInOutput: "recommend-1"},
		{name: "item-to-item", args: []string{"get", "item-to-item", "neighbors", "item-1", "--n", "3", "--offset", "1", "--category", "news"}, wantPath: "/dashboard/item-to-item/neighbors/item-1", wantRawQuery: []string{"n=3", "offset=1", "category=news"}, wantInOutput: "similar-1"},
		{name: "user-to-user", args: []string{"get", "user-to-user", "neighbors", "alice", "--n", "3", "--offset", "1"}, wantPath: "/dashboard/user-to-user/neighbors/alice", wantRawQuery: []string{"n=3", "offset=1"}, wantInOutput: "neighbor-1"},
		{name: "external", args: []string{"get", "external", "--script", "return []", "--user-id", "alice"}, wantPath: "/dashboard/external", wantRawQuery: []string{"script=" + base64.StdEncoding.EncodeToString([]byte("return []")), "user-id=alice"}, wantInOutput: "external-1"},
		{name: "ranker-prompt", args: []string{"get", "ranker-prompt", "--query-template", "query", "--document-template", "document", "--user-id", "alice"}, wantPath: "/dashboard/ranker/prompt", wantRawQuery: []string{"query-template=" + base64.StdEncoding.EncodeToString([]byte("query")), "document-template=" + base64.StdEncoding.EncodeToString([]byte("document")), "user-id=alice"}, wantInOutput: "documents"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newTestAdminServer(t, http.MethodGet, tt.wantPath, tt.wantRawQuery, sampleResponse(tt.wantPath))
			defer server.Close()

			args := append([]string{}, tt.args...)
			args = append(args, "--endpoint", server.URL, "--api-key", "secret")
			out, err := executeCommand(rootCmd, args...)
			if err != nil {
				t.Fatal(err)
			}
			assertContains(t, out, tt.wantInOutput)
			assertContains(t, out, "┌")
		})
	}
}

func TestSetCmd(t *testing.T) {
	out, err := executeCommand(rootCmd, "set")
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, out, "Set resources in Gorse admin API")
}

func TestSetConfigCmd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/dashboard/config" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-Api-Key") != "secret" {
			t.Fatalf("unexpected api key: %s", r.Header.Get("X-Api-Key"))
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		var patch map[string]any
		if err := json.Unmarshal(body, &patch); err != nil {
			t.Fatal(err)
		}
		if patch["recommend.cache_size"] != float64(100) || patch["recommend.enable_latest_recommend"] != true || patch["recommend.item_ttl"] != "72h" {
			t.Fatalf("unexpected patch: %#v", patch)
		}
		writeJSON(t, w, map[string]any{"recommend": map[string]any{"cache_size": 100}})
	}))
	defer server.Close()

	out, err := executeCommand(rootCmd, "set", "config", "recommend.cache_size=100", "recommend.enable_latest_recommend=true", "recommend.item_ttl=72h", "--endpoint", server.URL, "--api-key", "secret")
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, out, "cache_size")
	assertContains(t, out, "100")
}

func mockKeyring(t *testing.T, get func(string, string) (string, error), set func(string, string, string) error, delete func(string, string) error) func() {
	t.Helper()
	oldGet, oldSet, oldDelete := keyringGet, keyringSet, keyringDelete
	keyringGet, keyringSet, keyringDelete = get, set, delete
	return func() {
		keyringGet, keyringSet, keyringDelete = oldGet, oldSet, oldDelete
	}
}

func newTestAdminServer(t *testing.T, method, path string, rawQueryParts []string, response any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			t.Fatalf("unexpected method: got %s want %s", r.Method, method)
		}
		if r.URL.Path != path {
			t.Fatalf("unexpected path: got %s want %s", r.URL.Path, path)
		}
		if r.Header.Get("X-Api-Key") != "secret" {
			t.Fatalf("unexpected api key: %s", r.Header.Get("X-Api-Key"))
		}
		query := r.URL.Query()
		for _, part := range rawQueryParts {
			key, value, ok := strings.Cut(part, "=")
			if !ok {
				t.Fatalf("invalid query assertion: %q", part)
			}
			found := false
			for _, got := range query[key] {
				if got == value {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected query %s=%q in %q", key, value, r.URL.RawQuery)
			}
		}
		writeJSON(t, w, response)
	}))
}

func sampleResponse(path string) any {
	switch path {
	case "/dashboard/userinfo":
		return map[string]any{"name": "admin", "email": "admin@example.com", "email_verified": true}
	case "/dashboard/cluster":
		return []map[string]any{{"type": "server", "uuid": "server-node", "hostname": "localhost", "version": "v1"}}
	case "/dashboard/categories":
		return []string{"books", "news"}
	case "/dashboard/tasks":
		return []map[string]any{{"name": "Load Data", "progress": 0.5}}
	case "/dashboard/config":
		return map[string]any{"recommend": map[string]any{"cache_size": 100}}
	case "/dashboard/config/schema":
		return map[string]any{"type": "object", "title": "Config"}
	case "/dashboard/stats":
		return map[string]any{"binary_version": "v1", "num_users": 1}
	case "/dashboard/timeseries/requests":
		return map[string]any{"requests": []map[string]any{{"timestamp": "2026-01-01T00:00:00Z", "value": 1}}}
	case "/dashboard/user/alice":
		return map[string]any{"user_id": "alice", "labels": map[string]any{"role": "tester"}}
	case "/dashboard/users":
		return map[string]any{"cursor": "", "users": []map[string]any{{"user_id": "bob"}}}
	case "/dashboard/user/alice/feedback/click":
		return []map[string]any{{"feedback_type": "click", "user_id": "alice", "item_id": "item-1"}}
	case "/dashboard/latest":
		return []map[string]any{{"item_id": "latest-1", "score": 1}}
	case "/dashboard/non-personalized/popular":
		return []map[string]any{{"item_id": "popular-1", "score": 1}}
	case "/dashboard/recommend/alice/non-personalized/popular":
		return []map[string]any{{"item_id": "recommend-1", "score": 1}}
	case "/dashboard/item-to-item/neighbors/item-1":
		return []map[string]any{{"item_id": "similar-1", "score": 1}}
	case "/dashboard/user-to-user/neighbors/alice":
		return []map[string]any{{"user_id": "neighbor-1", "score": 1}}
	case "/dashboard/external":
		return []map[string]any{{"item_id": "external-1", "score": 1}}
	case "/dashboard/ranker/prompt":
		return map[string]any{"query": "query", "documents": []string{"documents"}}
	default:
		return map[string]any{"ok": true}
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatal(err)
	}
}

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Fatalf("expected %q to contain %q", s, substr)
	}
}
