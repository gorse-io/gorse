package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/master"
	"github.com/gorse-io/gorse/protocol"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"
)

func executeRawCommand(root *cobra.Command, args ...string) (string, error) {
	return executeRawCommandWithInput(root, "", args...)
}

func executeRawCommandWithInput(root *cobra.Command, input string, args ...string) (string, error) {
	buf := new(bytes.Buffer)

	resetFlags(root)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetIn(bytes.NewBufferString(input))
	root.SetArgs(args)

	err := root.Execute()
	return buf.String(), err
}

func resetFlags(cmd *cobra.Command) {
	resetFlagSet(cmd.Flags())
	resetFlagSet(cmd.PersistentFlags())
	for _, child := range cmd.Commands() {
		resetFlags(child)
	}
}

func resetFlagSet(flags *pflag.FlagSet) {
	flags.VisitAll(func(flag *pflag.Flag) {
		if flag.Value.Type() == "stringArray" {
			flag.Changed = false
			return
		}
		_ = flag.Value.Set(flag.DefValue)
		flag.Changed = false
	})
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
		{ItemId: "item-1", Categories: []string{"news"}, Timestamp: time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC), Labels: map[string]any{"embedding": []any{
			json.Number("0.1"), json.Number("0.2"), json.Number("0.3"), json.Number("0.4"), json.Number("0.5"),
			json.Number("0.6"), json.Number("0.7"), json.Number("0.8"), json.Number("0.9"), json.Number("1.0"),
		}}, Comment: "raw item"},
		{ItemId: "item-2", Categories: []string{"books"}, Timestamp: time.Date(2026, 1, 1, 2, 0, 0, 0, time.UTC), Labels: map[string]any{"description": strings.Repeat("x", 120)}, Comment: "listed item"},
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

func (s *CLITestSuite) execute(args ...string) (string, error) {
	return s.executeCommandWithInput("", args...)
}

func (s *CLITestSuite) executeCommandWithInput(input string, args ...string) (string, error) {
	argsWithAuth := append([]string{}, args...)
	argsWithAuth = append(argsWithAuth, "--endpoint", s.endpoint, "--api-key", testAPIKey)
	return executeRawCommandWithInput(rootCmd, input, argsWithAuth...)
}

func TestCLIHelpOmitsAdminAPIQualifier(t *testing.T) {
	out, err := executeRawCommand(rootCmd, "--help")
	require.NoError(t, err)
	require.NotContains(t, out, "from Gorse admin API")
}

func TestRecommendHelpOmitsRankerPrompt(t *testing.T) {
	out, err := executeRawCommand(rootCmd, "recommend", "--help")
	require.NoError(t, err)
	require.NotContains(t, out, "ranker-prompt")
}

func TestCLI(t *testing.T) {
	suite.Run(t, new(CLITestSuite))
}

func (s *CLITestSuite) TestGetSubcommands() {
	tests := []struct {
		name         string
		args         []string
		wantInOutput string
		wantTable    bool
	}{
		{name: "cluster-info", args: []string{"cluster-info"}, wantInOutput: "server-node", wantTable: true},
		{name: "categories", args: []string{"get", "categories"}, wantInOutput: "books", wantTable: true},
		{name: "ps", args: []string{"ps"}, wantTable: true},
		{name: "stats", args: []string{"stats"}, wantInOutput: "BinaryVersion"},
		{name: "user", args: []string{"get", "user", "alice"}, wantInOutput: "alice"},
		{name: "users", args: []string{"get", "users", "-n", "10"}, wantInOutput: "bob", wantTable: true},
		{name: "item", args: []string{"get", "item", "item-1"}, wantInOutput: "item-1"},
		{name: "items", args: []string{"get", "items", "-n", "10"}, wantInOutput: "item-2", wantTable: true},
		{name: "feedback", args: []string{"get", "feedback", "--type", "click", "--user", "alice", "-n", "5"}, wantInOutput: "item-1", wantTable: true},
		{name: "latest", args: []string{"recommend", "latest", "-n", "3", "--category", "tech"}, wantInOutput: "latest-1", wantTable: true},
		{name: "non-personalized", args: []string{"recommend", "non-personalized", "popular", "-n", "3", "--user-id", "alice", "--category", "news"}, wantInOutput: "popular-1", wantTable: true},
		{name: "recommend-item-to-user", args: []string{"recommend", "item-to-user", "alice", "non-personalized", "popular", "-n", "3", "--category", "news"}, wantInOutput: "recommend-1", wantTable: true},
		{name: "item-to-item", args: []string{"recommend", "item-to-item", "neighbors", "item-1", "-n", "3", "--category", "news"}, wantInOutput: "similar-1", wantTable: true},
		{name: "user-to-user", args: []string{"recommend", "user-to-user", "neighbors", "alice", "-n", "3"}, wantInOutput: "neighbor-1", wantTable: true},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			args := append([]string{}, tt.args...)
			out, err := s.execute(args...)
			s.Require().NoError(err)
			if tt.wantInOutput != "" {
				s.Require().Contains(out, tt.wantInOutput)
			}
			if tt.wantTable {
				s.Require().NotContains(out, "┌")
				s.Require().NotContains(out, "│")
				s.Require().NotContains(out, "└")
			}
		})
	}
}

func (s *CLITestSuite) TestGetUser() {
	out, err := s.execute("get", "user", "alice")
	s.Require().NoError(err)
	s.Require().Equal(`UserId: alice
Comment: 
Labels:
{
  "role": "tester"
}
`, out)
}

func (s *CLITestSuite) TestGetUsers() {
	out, err := s.execute("get", "users", "-n", "10")
	s.Require().NoError(err)

	lines := strings.Split(strings.TrimSpace(out), "\n")
	s.Require().Len(lines, 4)
	s.Require().Regexp(`^USER-ID\s+COMMENT\s+LABELS$`, lines[0])
	s.Require().Regexp(`^alice\s+\{"role":"tester"\}$`, lines[1])
	s.Require().Regexp(`^bob\s*$`, lines[2])
	s.Require().Regexp(`^neighbor-1\s*$`, lines[3])
}

func (s *CLITestSuite) TestGetItem() {
	out, err := s.execute("get", "item", "item-1")
	s.Require().NoError(err)
	s.Require().Equal(`ItemId: item-1
Comment: raw item
Categories: ["news"]
IsHidden: false
Timestamp: 2026-01-01T01:00:00Z
Labels:
{
  "embedding": "[0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, ...] (10 values)"
}
`, out)
}

func (s *CLITestSuite) TestGetFeedback() {
	tests := [][]string{
		{"get", "feedback", "-n", "1"},
		{"get", "feedback", "--type", "click", "--user", "alice", "-n", "1"},
		{"get", "feedback", "--type", "click", "--item", "item-1", "-n", "1"},
		{"get", "feedback", "--type", "click", "--user", "alice", "--item", "item-1", "-n", "1"},
	}
	for _, args := range tests {
		out, err := s.execute(args...)
		s.Require().NoError(err)

		lines := strings.Split(strings.TrimSpace(out), "\n")
		s.Require().Len(lines, 2)
		s.Require().Regexp(`^FEEDBACK-TYPE\s+USER-ID\s+ITEM-ID\s+VALUE\s+TIMESTAMP\s+COMMENT\s+UPDATED$`, lines[0])
		s.Require().Regexp(`^click\s+alice\s+item-1\s+0\s+2026-01-01T03:00:00Z\s+\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`, lines[1])
		s.Require().NotContains(out, "Cursor")
		s.Require().NotContains(out, "┌")
		s.Require().NotContains(out, "│")
		s.Require().NotContains(out, "└")
	}

	out, err := executeRawCommand(rootCmd, "get", "feedback", "click")
	s.Require().Error(err)
	s.Require().Contains(out, `unknown command "click"`)
}

func (s *CLITestSuite) TestGetItems() {
	out, err := s.execute("get", "items", "-n", "10")
	s.Require().NoError(err)

	lines := strings.Split(strings.TrimSpace(out), "\n")
	s.Require().Len(lines, 7)
	s.Require().Regexp(`^ITEM-ID\s+COMMENT\s+CATEGORIES\s+IS-HIDDEN\s+TIMESTAMP\s+LABELS$`, lines[0])
	s.Require().Regexp(`^item-1\s+raw item\s+\["news"\]\s+false\s+2026-01-01T01:00:00Z\s+\{"embedding":"\[0\.1, 0\.2, 0\.3, \.\.\.\] \(10 values\)"\}$`, lines[1])
	s.Require().Regexp(`^item-2\s+listed item\s+\["books"\]\s+false\s+2026-01-01T02:00:00Z\s+\{"description":"`+strings.Repeat("x", 120)+`"\}$`, lines[2])
	s.Require().Regexp(`^latest-1\s+\["news","tech"\]\s+false\s+2026-01-02T00:00:00Z\s*$`, lines[3])
	s.Require().Regexp(`^popular-1\s+\["news"\]\s+false\s+2026-01-03T00:00:00Z\s*$`, lines[4])
	s.Require().Regexp(`^recommend-1\s+\["news"\]\s+false\s+2026-01-04T00:00:00Z\s*$`, lines[5])
	s.Require().Regexp(`^similar-1\s+\["news"\]\s+false\s+2026-01-05T00:00:00Z\s*$`, lines[6])
	s.Require().NotContains(out, "LABELS-EMBEDDING")
	s.Require().NotContains(out, "0.4")
	s.Require().NotContains(out, "┌")
	s.Require().NotContains(out, "│")
	s.Require().NotContains(out, "└")
}

func (s *CLITestSuite) TestPipelineGetCmd() {
	out, err := s.execute("pipeline", "get")
	s.Require().NoError(err)

	var config map[string]any
	s.Require().NoError(yaml.Unmarshal([]byte(out), &config))
	s.Require().Contains(config, "cache_size")
	s.Require().NotContains(config, "recommend")
	s.Require().Contains(out, "cache_size:")
	s.Require().NotContains(out, "recommend:")
}

func (s *CLITestSuite) TestPipelineSchemaCmd() {
	out, err := s.execute("pipeline", "schema")
	s.Require().NoError(err)

	var schema map[string]any
	s.Require().NoError(yaml.Unmarshal([]byte(out), &schema))
	s.Require().Contains(schema, "$schema")
	s.Require().Contains(schema, "$defs")
	s.Require().Contains(out, "$schema:")
	s.Require().Contains(out, "$defs:")
	s.Require().Contains(out, "RecommendConfig")
}

func (s *CLITestSuite) TestPipelinePatchCmd() {
	patch := `[{"op":"replace","path":"/recommend/cache_size","value":100},{"op":"replace","path":"/recommend/ranker/type","value":"llm"}]`
	out, err := s.execute("pipeline", "patch", patch)
	s.Require().NoError(err)
	s.Require().Contains(out, "CacheSize")
	s.Require().Contains(out, "100")
	s.Require().Contains(out, "llm")

	setOut, err := executeRawCommand(rootCmd, "pipeline", "set", "recommend.cache_size=100")
	s.Require().Error(err)
	s.Require().Contains(setOut, `unknown command "set"`)
}

func (s *CLITestSuite) TestPipelineResetCmd() {
	cancelOut, err := s.executeCommandWithInput("N\n", "pipeline", "reset")
	s.Require().NoError(err)
	s.Require().Contains(cancelOut, "Confirm [y/N]")
	s.Require().Contains(cancelOut, "Pipeline reset canceled")
	s.Require().NotContains(cancelOut, "Pipeline reset to defaults")

	resetOut, err := s.executeCommandWithInput("y\n", "pipeline", "reset")
	s.Require().NoError(err)
	s.Require().Contains(resetOut, "Confirm [y/N]")
	s.Require().Contains(resetOut, "Pipeline reset")
}

func (s *CLITestSuite) TestDumpRestore() {
	outputPath := s.T().TempDir() + "/backup.bin"
	dumpOut, err := s.execute("dump", outputPath)
	s.Require().NoError(err)
	s.Require().Contains(dumpOut, "Data dumped to "+outputPath)
	content, err := os.ReadFile(outputPath)
	s.Require().NoError(err)
	s.Require().NotEmpty(content)
	s.Require().NoError(s.m.DataClient.Purge())

	cancelOut, err := s.executeCommandWithInput("N\n", "restore", outputPath)
	s.Require().NoError(err)
	s.Require().Contains(cancelOut, "Confirm [y/N]")
	s.Require().Contains(cancelOut, "Restore canceled")
	_, users, err := s.m.DataClient.GetUsers(s.T().Context(), "", 10)
	s.Require().NoError(err)
	s.Require().Empty(users)

	restoreOut, err := s.executeCommandWithInput("y\n", "restore", outputPath)
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
		closeTestMasterStores(t, m)
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

func closeTestMasterStores(t *testing.T, m *master.Master) {
	t.Helper()
	require.NoError(t, m.DataClient.Close())
	require.NoError(t, m.CacheClient.Close())

	metaStoreValue := reflect.ValueOf(m).Elem().FieldByName("metaStore")
	if metaStoreValue.IsNil() {
		return
	}
	metaStore := reflect.NewAt(metaStoreValue.Type(), unsafe.Pointer(metaStoreValue.UnsafeAddr())).Elem().Interface()
	closer, ok := metaStore.(interface{ Close() error })
	require.True(t, ok)
	require.NoError(t, closer.Close())
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
