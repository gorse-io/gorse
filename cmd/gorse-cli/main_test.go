package main

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/master"
	"github.com/gorse-io/gorse/protocol"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/zalando/go-keyring"
)

func executeCommand(root *cobra.Command, args ...string) (string, error) {
	return executeCommandWithInput(root, "", args...)
}

func executeCommandWithInput(root *cobra.Command, input string, args ...string) (string, error) {
	buf := new(bytes.Buffer)

	resetHelpFlags(root)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetIn(bytes.NewBufferString(input))
	root.SetArgs(args)

	err := root.Execute()
	return buf.String(), err
}

func resetHelpFlags(cmd *cobra.Command) {
	if flag := cmd.Flags().Lookup("help"); flag != nil {
		_ = flag.Value.Set("false")
		flag.Changed = false
	}
	for _, child := range cmd.Commands() {
		resetHelpFlags(child)
	}
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

	out, err := executeCommand(rootCmd, "login", "--endpoint", "http://localhost:8088", "--api-key", "secret")
	require.NoError(t, err)
	require.Contains(t, out, "Credentials saved")
	require.Equal(t, "http://localhost:8088", sets[keyringEndpointKey])
	require.Equal(t, "secret", sets[keyringAPIKeyKey])
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
	require.NoError(t, err)
	require.Contains(t, out, "Credentials removed")
	require.True(t, deleted[keyringEndpointKey])
	require.True(t, deleted[keyringAPIKeyKey])
}

type CLITestSuite struct {
	suite.Suite
	m        *master.Master
	endpoint string
}

func (s *CLITestSuite) SetupTest() {
	s.m, s.endpoint = newTestMaster(s.T())

	ctx := s.T().Context()
	_, err := s.m.GetMeta(ctx, &protocol.NodeInfo{
		NodeType:      protocol.NodeType_Server,
		Uuid:          "server-node",
		Hostname:      "localhost",
		BinaryVersion: "v1",
	})
	s.Require().NoError(err)
	s.Require().NoError(s.m.DataClient.BatchInsertUsers(ctx, []data.User{
		{UserId: "alice", Labels: map[string]any{"role": "tester"}},
		{UserId: "bob"},
		{UserId: "neighbor-1"},
	}))
	s.Require().NoError(s.m.DataClient.BatchInsertItems(ctx, []data.Item{
		{ItemId: "item-1", Categories: []string{"news"}, Timestamp: time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC), Comment: "raw item"},
		{ItemId: "item-2", Categories: []string{"books"}, Timestamp: time.Date(2026, 1, 1, 2, 0, 0, 0, time.UTC), Comment: "listed item"},
		{ItemId: "latest-1", Categories: []string{"news", "tech"}, Timestamp: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)},
		{ItemId: "popular-1", Categories: []string{"news"}, Timestamp: time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)},
		{ItemId: "recommend-1", Categories: []string{"news"}, Timestamp: time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC)},
		{ItemId: "similar-1", Categories: []string{"news"}, Timestamp: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)},
	}))
	s.Require().NoError(s.m.DataClient.BatchInsertFeedback(ctx, []data.Feedback{{
		FeedbackKey: data.FeedbackKey{FeedbackType: "click", UserId: "alice", ItemId: "item-1"},
		Timestamp:   time.Date(2026, 1, 1, 3, 0, 0, 0, time.UTC),
	}}, true, true, true))
	s.Require().NoError(s.m.CacheClient.AddScores(ctx, cache.ItemCategories, "", []cache.Score{
		{Id: "books", Score: 2},
		{Id: "news", Score: 1},
	}))
	s.Require().NoError(s.m.CacheClient.AddScores(ctx, cache.NonPersonalized, "popular", []cache.Score{
		{Id: "popular-1", Score: 2, Categories: []string{"news"}},
		{Id: "recommend-1", Score: 1, Categories: []string{"news"}},
	}))
	s.Require().NoError(s.m.CacheClient.Set(ctx, cache.String(cache.Key(cache.NonPersonalizedDigest, "popular"), "digest")))
	s.Require().NoError(s.m.CacheClient.AddScores(ctx, cache.ItemToItem, cache.Key("neighbors", "item-1"), []cache.Score{
		{Id: "similar-1", Score: 1, Categories: []string{"news"}},
	}))
	s.Require().NoError(s.m.CacheClient.AddScores(ctx, cache.UserToUser, cache.Key("neighbors", "alice"), []cache.Score{
		{Id: "neighbor-1", Score: 1},
	}))
	s.Require().NoError(s.m.CacheClient.AddTimeSeriesPoints(ctx, []cache.TimeSeriesPoint{{
		Name:      "requests",
		Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Value:     1,
	}}))
}

func TestCLI(t *testing.T) {
	suite.Run(t, new(CLITestSuite))
}

func (s *CLITestSuite) TestGetSubcommands() {
	tests := []struct {
		name         string
		args         []string
		wantInOutput string
	}{
		{name: "userinfo", args: []string{"get", "userinfo"}, wantInOutput: "admin"},
		{name: "cluster", args: []string{"get", "cluster"}, wantInOutput: "server-node"},
		{name: "categories", args: []string{"get", "categories"}, wantInOutput: "books"},
		{name: "tasks", args: []string{"get", "tasks"}},
		{name: "config", args: []string{"get", "config"}, wantInOutput: "cache_size"},
		{name: "config-schema", args: []string{"get", "config-schema"}, wantInOutput: "object"},
		{name: "stats", args: []string{"get", "stats"}, wantInOutput: "BinaryVersion"},
		{name: "timeseries", args: []string{"get", "timeseries", "requests", "--begin", "2026-01-01", "--end", "2026-01-02", "--duration", "1h"}, wantInOutput: "2026-01-01"},
		{name: "user", args: []string{"get", "user", "alice"}, wantInOutput: "alice"},
		{name: "users", args: []string{"get", "users", "--n", "10"}, wantInOutput: "bob"},
		{name: "item", args: []string{"get", "item", "item-1"}, wantInOutput: "item-1"},
		{name: "items", args: []string{"get", "items", "--n", "10"}, wantInOutput: "item-2"},
		{name: "user-feedback", args: []string{"get", "user-feedback", "alice", "click", "--n", "5"}, wantInOutput: "item-1"},
		{name: "latest", args: []string{"get", "latest", "--n", "3", "--category", "tech"}, wantInOutput: "latest-1"},
		{name: "non-personalized", args: []string{"get", "non-personalized", "popular", "--n", "3", "--user-id", "alice", "--category", "news"}, wantInOutput: "popular-1"},
		{name: "recommend", args: []string{"get", "recommend", "alice", "non-personalized", "popular", "--n", "3", "--category", "news"}, wantInOutput: "recommend-1"},
		{name: "item-to-item", args: []string{"get", "item-to-item", "neighbors", "item-1", "--n", "3", "--category", "news"}, wantInOutput: "similar-1"},
		{name: "user-to-user", args: []string{"get", "user-to-user", "neighbors", "alice", "--n", "3"}, wantInOutput: "neighbor-1"},
		{name: "external", args: []string{"get", "external", "--script", `["external-1"]`, "--user-id", "alice"}, wantInOutput: "external-1"},
		{name: "ranker-prompt", args: []string{"get", "ranker-prompt", "--query-template", "query", "--document-template", "documents", "--user-id", "alice"}, wantInOutput: "documents"},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			args := append([]string{}, tt.args...)
			args = append(args, "--endpoint", s.endpoint, "--api-key", testAPIKey)
			out, err := executeCommand(rootCmd, args...)
			s.Require().NoError(err)
			if tt.wantInOutput != "" {
				s.Require().Contains(out, tt.wantInOutput)
			}
			s.Require().Contains(out, "┌")
		})
	}
}

func (s *CLITestSuite) TestSetConfigCmd() {
	out, err := executeCommand(rootCmd, "set", "config", "recommend.cache_size=100", "recommend.ranker.type=llm", "--endpoint", s.endpoint, "--api-key", testAPIKey)
	s.Require().NoError(err)
	s.Require().Contains(out, "CacheSize")
	s.Require().Contains(out, "100")
	s.Require().Contains(out, "llm")
}

func (s *CLITestSuite) TestResetConfigCmd() {
	out, err := executeCommand(rootCmd, "reset", "config", "--endpoint", s.endpoint, "--api-key", testAPIKey)
	s.Require().NoError(err)
	s.Require().Contains(out, "Configuration reset")
}

func (s *CLITestSuite) TestDumpRestore() {
	outputPath := s.T().TempDir() + "/backup.bin"
	dumpOut, err := executeCommand(rootCmd, "dump", outputPath, "--endpoint", s.endpoint, "--api-key", testAPIKey)
	s.Require().NoError(err)
	s.Require().Contains(dumpOut, "Data dumped to "+outputPath)
	content, err := os.ReadFile(outputPath)
	s.Require().NoError(err)
	s.Require().NotEmpty(content)
	s.Require().NoError(s.m.DataClient.Purge())

	cancelOut, err := executeCommandWithInput(rootCmd, "N\n", "restore", outputPath, "--endpoint", s.endpoint, "--api-key", testAPIKey)
	s.Require().NoError(err)
	s.Require().Contains(cancelOut, "Confirm [y/N]")
	s.Require().Contains(cancelOut, "Restore canceled")
	_, users, err := s.m.DataClient.GetUsers(s.T().Context(), "", 10)
	s.Require().NoError(err)
	s.Require().Empty(users)

	restoreOut, err := executeCommandWithInput(rootCmd, "y\n", "restore", outputPath, "--endpoint", s.endpoint, "--api-key", testAPIKey)
	s.Require().NoError(err)
	s.Require().Contains(restoreOut, "Confirm [y/N]")
	s.Require().Contains(restoreOut, "Restored 3 users, 6 items, 1 feedback in ")
	s.Require().NotContains(restoreOut, "│")
	s.Require().Equal(1, strings.Count(restoreOut, "\n"))

	_, err = s.m.DataClient.GetUser(s.T().Context(), "alice")
	s.Require().NoError(err)
	_, err = s.m.DataClient.GetItem(s.T().Context(), "item-1")
	s.Require().NoError(err)
	endTime := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	_, feedback, err := s.m.DataClient.GetFeedback(s.T().Context(), "", 10, nil, &endTime)
	s.Require().NoError(err)
	s.Require().Len(feedback, 1)
	s.Require().Equal(data.FeedbackKey{FeedbackType: "click", UserId: "alice", ItemId: "item-1"}, feedback[0].FeedbackKey)
}

func mockKeyring(t *testing.T, get func(string, string) (string, error), set func(string, string, string) error, delete func(string, string) error) func() {
	t.Helper()
	oldGet, oldSet, oldDelete := keyringGet, keyringSet, keyringDelete
	keyringGet, keyringSet, keyringDelete = get, set, delete
	return func() {
		keyringGet, keyringSet, keyringDelete = oldGet, oldSet, oldDelete
	}
}

const testAPIKey = "secret"

func newTestMaster(t *testing.T) (*master.Master, string) {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "gorse-cli-*")
	require.NoError(t, err)
	cfg := config.GetDefaultConfig()
	cfg.Database.DataStore = "sqlite://" + filepath.Join(tempDir, "data.db")
	cfg.Database.CacheStore = "sqlite://" + filepath.Join(tempDir, "cache.db")
	cfg.Blob.URI = filepath.Join(tempDir, "blob")
	cfg.Master.Host = "127.0.0.1"
	cfg.Master.Port = freePort(t)
	cfg.Master.HttpHost = "127.0.0.1"
	cfg.Master.HttpPort = freePort(t)
	cfg.Master.AdminAPIKey = testAPIKey
	cfg.Master.DashboardUserName = "admin"
	cfg.Master.DashboardPassword = "pass"
	cfg.OpenAI.AuthToken = "test"

	m := master.NewMaster(cfg, tempDir, true, "")
	go m.Serve()

	endpoint := fmt.Sprintf("http://%s:%d", cfg.Master.HttpHost, cfg.Master.HttpPort)
	waitForMaster(t, endpoint)
	t.Cleanup(func() {
		m.Shutdown()
		for range 10 {
			if err := os.RemoveAll(tempDir); err == nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		require.NoError(t, os.RemoveAll(tempDir))
	})
	return m, endpoint
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func waitForMaster(t *testing.T, endpoint string) {
	t.Helper()
	client := http.Client{Timeout: time.Second}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(endpoint + "/api/health/live")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.Failf(t, "master didn't become ready", "endpoint: %s", endpoint)
}
