// Copyright 2026 gorse Project Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"
	"unicode"

	"github.com/go-resty/resty/v2"
	gorse "github.com/gorse-io/gorse-go"
	"github.com/gorse-io/gorse/cmd/version"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/master"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
	"go.uber.org/zap"
	"golang.org/x/term"
)

var rootCmd = &cobra.Command{
	Use:   "gorse-cli",
	Short: "Gorse command line tool for cluster management",
	Run: func(cmd *cobra.Command, args []string) {
		// Show version
		if showVersion, _ := cmd.PersistentFlags().GetBool("version"); showVersion {
			cmd.Println(version.BuildInfo())
			return
		}
		_ = cmd.Help()
	},
}

const (
	keyringService           = "gorse-cli"
	keyringCurrentContextKey = "current-context"
	keyringContextsKey       = "contexts"
	maxInlineArrayValues     = 8
	maxTableLabelArrayValues = 3
)

var (
	keyringGet    = keyring.Get
	keyringSet    = keyring.Set
	keyringDelete = keyring.Delete
)

// newAdminClient creates a new resty client for admin API
func newAdminClient(endpoint, apiKey string) *resty.Client {
	client := resty.New()
	client.SetBaseURL(strings.TrimRight(endpoint, "/") + "/api")
	client.SetHeader("X-Api-Key", apiKey)
	return client
}

type cliContext struct {
	Name     string
	Endpoint string
	APIKey   string
}

func contextEndpointKey(name string) string {
	return "context:" + name + ":admin-endpoint"
}

func contextAPIKeyKey(name string) string {
	return "context:" + name + ":admin-api-key"
}

func validateContextName(name string) error {
	if name == "" {
		return fmt.Errorf("context name is required")
	}
	for i := range len(name) {
		ch := name[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') ||
			ch == '-' || ch == '_' || ch == '.' {
			continue
		}
		return fmt.Errorf("context name %q contains invalid character %q", name, ch)
	}
	return nil
}

func getContextNames() ([]string, error) {
	raw, err := keyringGet(keyringService, keyringContextsKey)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	if err = json.Unmarshal([]byte(raw), &names); err != nil {
		return nil, err
	}
	names = lo.Uniq(names)
	sort.Strings(names)
	return names, nil
}

func saveContextNames(names []string) error {
	names = lo.Uniq(names)
	sort.Strings(names)
	raw, err := json.Marshal(names)
	if err != nil {
		return err
	}
	return keyringSet(keyringService, keyringContextsKey, string(raw))
}

func addContextName(name string) error {
	names, err := getContextNames()
	if err != nil {
		return err
	}
	if !lo.Contains(names, name) {
		names = append(names, name)
	}
	return saveContextNames(names)
}

func removeContextName(name string) ([]string, error) {
	names, err := getContextNames()
	if err != nil {
		return nil, err
	}
	names = lo.Filter(names, func(candidate string, _ int) bool {
		return candidate != name
	})
	if err = saveContextNames(names); err != nil {
		return nil, err
	}
	return names, nil
}

func getCurrentContextName() (string, error) {
	name, err := keyringGet(keyringService, keyringCurrentContextKey)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	return name, err
}

func setCurrentContextName(name string) error {
	return keyringSet(keyringService, keyringCurrentContextKey, name)
}

func clearCurrentContextName() error {
	if err := keyringDelete(keyringService, keyringCurrentContextKey); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return err
	}
	return nil
}

func loadContext(name string) (cliContext, error) {
	if err := validateContextName(name); err != nil {
		return cliContext{}, err
	}
	endpoint, endpointErr := keyringGet(keyringService, contextEndpointKey(name))
	apiKey, apiKeyErr := keyringGet(keyringService, contextAPIKeyKey(name))
	if errors.Is(endpointErr, keyring.ErrNotFound) || errors.Is(apiKeyErr, keyring.ErrNotFound) {
		return cliContext{}, fmt.Errorf("context %q not found", name)
	}
	if endpointErr != nil {
		return cliContext{}, endpointErr
	}
	if apiKeyErr != nil {
		return cliContext{}, apiKeyErr
	}
	return cliContext{Name: name, Endpoint: endpoint, APIKey: apiKey}, nil
}

func saveContext(name, endpoint, apiKey string) error {
	if err := validateContextName(name); err != nil {
		return err
	}
	if endpoint == "" {
		return fmt.Errorf("GORSE_ADMIN_ENDPOINT or --endpoint is required")
	}
	if apiKey == "" {
		return fmt.Errorf("GORSE_ADMIN_API_KEY or --api-key is required")
	}
	if err := keyringSet(keyringService, contextEndpointKey(name), endpoint); err != nil {
		return err
	}
	if err := keyringSet(keyringService, contextAPIKeyKey(name), apiKey); err != nil {
		return err
	}
	if err := addContextName(name); err != nil {
		return err
	}
	return setCurrentContextName(name)
}

func deleteContext(name string) error {
	if _, err := loadContext(name); err != nil {
		return err
	}
	if err := keyringDelete(keyringService, contextEndpointKey(name)); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return err
	}
	if err := keyringDelete(keyringService, contextAPIKeyKey(name)); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return err
	}
	names, err := removeContextName(name)
	if err != nil {
		return err
	}
	current, err := getCurrentContextName()
	if err != nil {
		return err
	}
	if current != name {
		return nil
	}
	if len(names) == 0 {
		return clearCurrentContextName()
	}
	return setCurrentContextName(names[0])
}

// getEndpointAndKey returns the base URL and API key from flags, context, or environment variables.
func getEndpointAndKey(cmd *cobra.Command) (endpoint, apiKey string) {
	endpoint, _ = cmd.Flags().GetString("endpoint")
	apiKey, _ = cmd.Flags().GetString("api-key")

	if endpoint == "" || apiKey == "" {
		contextName, _ := cmd.Flags().GetString("context")
		if contextName != "" {
			ctx, err := loadContext(contextName)
			if err != nil {
				fatalUserError(cmd,
					fmt.Sprintf("context %q was not found.", contextName),
					"List available contexts:",
					"  gorse-cli context list",
					"Create it:",
					"  gorse-cli context add "+contextName+" --endpoint http://localhost:8088",
				)
			}
			if endpoint == "" {
				endpoint = ctx.Endpoint
			}
			if apiKey == "" {
				apiKey = ctx.APIKey
			}
		}
	}

	if endpoint == "" {
		endpoint = os.Getenv("GORSE_ADMIN_ENDPOINT")
	}
	if apiKey == "" {
		apiKey = os.Getenv("GORSE_ADMIN_API_KEY")
	}

	if endpoint == "" || apiKey == "" {
		contextName, err := getCurrentContextName()
		if err != nil {
			fatalUserError(cmd, "failed to read the current context from the system keyring: "+err.Error())
		}
		if contextName != "" {
			ctx, err := loadContext(contextName)
			if err != nil {
				fatalUserError(cmd,
					fmt.Sprintf("current context %q is invalid or incomplete.", contextName),
					"Choose another context:",
					"  gorse-cli context use <name>",
					"Or recreate it:",
					"  gorse-cli context add "+contextName+" --endpoint http://localhost:8088",
				)
			}
			if endpoint == "" {
				endpoint = ctx.Endpoint
			}
			if apiKey == "" {
				apiKey = ctx.APIKey
			}
		}
	}
	return
}

func getFlagOrEnv(cmd *cobra.Command, flagName, envName string) string {
	value, _ := cmd.Flags().GetString(flagName)
	if value != "" {
		return value
	}
	return os.Getenv(envName)
}

func readSecret(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	if term.IsTerminal(int(syscall.Stdin)) {
		secret, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Fprintln(os.Stderr)
		return strings.TrimSpace(string(secret)), err
	}
	reader := bufio.NewReader(os.Stdin)
	secret, err := reader.ReadString('\n')
	return strings.TrimSpace(secret), err
}

func fatalUserError(cmd *cobra.Command, message string, suggestions ...string) {
	output := cmd.ErrOrStderr()
	fmt.Fprintf(output, "Error: %s\n", message)
	if len(suggestions) > 0 {
		fmt.Fprintln(output)
		for _, suggestion := range suggestions {
			fmt.Fprintln(output, suggestion)
		}
	}
	os.Exit(1)
}

func fatalMissingCredentials(cmd *cobra.Command, missingEndpoint, missingAPIKey bool) {
	switch {
	case missingEndpoint && missingAPIKey:
		fatalUserError(cmd,
			"no Gorse context is selected and no endpoint/API key was provided.",
			"Create a context:",
			"  gorse-cli context add dev --endpoint http://localhost:8088",
			"Or use environment variables:",
			"  GORSE_ADMIN_ENDPOINT=http://localhost:8088 GORSE_ADMIN_API_KEY=<api-key> "+cmd.CommandPath(),
			"Or pass credentials for this command:",
			"  "+cmd.CommandPath()+" --endpoint http://localhost:8088 --api-key <api-key>",
		)
	case missingEndpoint:
		fatalUserError(cmd,
			"missing Gorse base URL.",
			"Use a saved context:",
			"  gorse-cli context use <name>",
			"Or set an environment variable:",
			"  GORSE_ADMIN_ENDPOINT=http://localhost:8088 "+cmd.CommandPath(),
			"Or pass an endpoint:",
			"  "+cmd.CommandPath()+" --endpoint http://localhost:8088",
		)
	case missingAPIKey:
		fatalUserError(cmd,
			"missing Gorse admin API key.",
			"Save it in a context:",
			"  gorse-cli context add <name> --endpoint http://localhost:8088",
			"Or set an environment variable:",
			"  GORSE_ADMIN_API_KEY=<api-key> "+cmd.CommandPath(),
			"Or pass it for this command:",
			"  "+cmd.CommandPath()+" --api-key <api-key>",
		)
	}
}

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Manage Gorse CLI contexts",
}

var contextAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add or update a Gorse CLI context",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		endpoint := getFlagOrEnv(cmd, "endpoint", "GORSE_ADMIN_ENDPOINT")
		apiKey := getFlagOrEnv(cmd, "api-key", "GORSE_ADMIN_API_KEY")

		if apiKey == "" {
			var err error
			apiKey, err = readSecret("Gorse admin API key: ")
			if err != nil {
				fatalUserError(cmd, "failed to read API key: "+err.Error())
			}
		}
		if err := saveContext(name, endpoint, apiKey); err != nil {
			fatalUserError(cmd,
				"failed to save context "+strconv.Quote(name)+": "+err.Error(),
				"Example:",
				"  gorse-cli context add "+name+" --endpoint http://localhost:8088",
			)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Context %q saved and selected.\n", name)
	},
}

var contextListCmd = &cobra.Command{
	Use:   "list",
	Short: "List Gorse CLI contexts",
	Run: func(cmd *cobra.Command, args []string) {
		names, err := getContextNames()
		if err != nil {
			fatalUserError(cmd, "failed to read contexts from the system keyring: "+err.Error())
		}
		if len(names) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No contexts configured.")
			return
		}
		current, err := getCurrentContextName()
		if err != nil {
			fatalUserError(cmd, "failed to read the current context from the system keyring: "+err.Error())
		}
		rows := make([][]string, 0, len(names))
		for _, name := range names {
			ctx, err := loadContext(name)
			if err != nil {
				fatalUserError(cmd, "context "+strconv.Quote(name)+" is invalid or incomplete: "+err.Error())
			}
			currentMarker := ""
			if name == current {
				currentMarker = "*"
			}
			rows = append(rows, []string{currentMarker, name, ctx.Endpoint})
		}
		printTable(cmd.OutOrStdout(), []string{"Current", "Name", "Endpoint"}, rows)
	},
}

var contextUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Switch the current Gorse CLI context",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		if _, err := loadContext(name); err != nil {
			fatalUserError(cmd,
				"context "+strconv.Quote(name)+" was not found.",
				"List available contexts:",
				"  gorse-cli context list",
			)
		}
		if err := setCurrentContextName(name); err != nil {
			fatalUserError(cmd, "failed to save the current context in the system keyring: "+err.Error())
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Switched to context %q.\n", name)
	},
}

var contextDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a Gorse CLI context",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		if err := deleteContext(name); err != nil {
			fatalUserError(cmd,
				"context "+strconv.Quote(name)+" was not found.",
				"List available contexts:",
				"  gorse-cli context list",
			)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Context %q deleted.\n", name)
	},
}

var contextCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show the current Gorse CLI context",
	Run: func(cmd *cobra.Command, args []string) {
		name, err := getCurrentContextName()
		if err != nil {
			fatalUserError(cmd, "failed to read the current context from the system keyring: "+err.Error())
		}
		if name == "" {
			fmt.Fprintln(cmd.OutOrStdout(), "No current context.")
			return
		}
		ctx, err := loadContext(name)
		if err != nil {
			fatalUserError(cmd,
				"current context "+strconv.Quote(name)+" is invalid or incomplete.",
				"Choose another context:",
				"  gorse-cli context use <name>",
				"Or recreate it:",
				"  gorse-cli context add "+name+" --endpoint http://localhost:8088",
			)
		}
		printTable(cmd.OutOrStdout(), []string{"Name", "Endpoint"}, [][]string{{ctx.Name, ctx.Endpoint}})
	},
}

func requireEndpointAndKey(cmd *cobra.Command) (string, string) {
	endpoint, apiKey := getEndpointAndKey(cmd)
	if endpoint == "" || apiKey == "" {
		fatalMissingCredentials(cmd, endpoint == "", apiKey == "")
	}
	return endpoint, apiKey
}

func newGorseClient(cmd *cobra.Command) *gorse.GorseClient {
	endpoint, apiKey := requireEndpointAndKey(cmd)
	return gorse.NewGorseClient(strings.TrimRight(endpoint, "/"), apiKey)
}

func getAdminAPI(cmd *cobra.Command, path string, query url.Values) {
	body := getAdminAPIBody(cmd, path, query)
	printJSONTable(cmd, body)
}

func getAdminAPIBody(cmd *cobra.Command, path string, query url.Values) []byte {
	endpoint, apiKey := requireEndpointAndKey(cmd)
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	client := newAdminClient(endpoint, apiKey)
	resp, err := client.R().Get(path)
	if err != nil {
		log.Logger().Fatal("failed to send request", zap.Error(err))
	}
	if resp.IsError() {
		log.Logger().Fatal("API request failed",
			zap.Int("status", resp.StatusCode()),
			zap.String("body", resp.String()))
	}
	return resp.Body()
}

func printResultTable(cmd *cobra.Command, result any) {
	body, err := json.Marshal(result)
	if err != nil {
		log.Logger().Fatal("failed to encode API response", zap.Error(err))
	}
	printJSONTable(cmd, body)
}

func postAdminAPIBody(cmd *cobra.Command, path, contentType string, reader io.Reader) {
	endpoint, apiKey := requireEndpointAndKey(cmd)
	client := newAdminClient(endpoint, apiKey)
	stats := new(master.DumpStats)
	resp, err := client.R().
		SetHeader("Content-Type", contentType).
		SetBody(reader).
		SetResult(stats).
		Post(path)
	if err != nil {
		log.Logger().Fatal("failed to send request", zap.Error(err))
	}
	if resp.IsError() {
		log.Logger().Fatal("API request failed",
			zap.Int("status", resp.StatusCode()),
			zap.String("body", resp.String()))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Restored %d users, %d items, %d feedback in %s.\n",
		stats.Users, stats.Items, stats.Feedback, stats.Duration)
}

func deleteAdminAPI(cmd *cobra.Command, path string) []byte {
	endpoint, apiKey := requireEndpointAndKey(cmd)
	client := newAdminClient(endpoint, apiKey)
	resp, err := client.R().Delete(path)
	if err != nil {
		log.Logger().Fatal("failed to send request", zap.Error(err))
	}
	if resp.IsError() {
		log.Logger().Fatal("API request failed",
			zap.Int("status", resp.StatusCode()),
			zap.String("body", resp.String()))
	}
	return resp.Body()
}

func downloadAdminAPI(cmd *cobra.Command, path string, output io.Writer) {
	endpoint, apiKey := requireEndpointAndKey(cmd)
	client := newAdminClient(endpoint, apiKey)
	resp, err := client.R().
		SetDoNotParseResponse(true).
		Get(path)
	if err != nil {
		log.Logger().Fatal("failed to send request", zap.Error(err))
	}
	defer resp.RawBody().Close()
	if resp.IsError() {
		body, _ := io.ReadAll(resp.RawBody())
		log.Logger().Fatal("API request failed",
			zap.Int("status", resp.StatusCode()),
			zap.String("body", string(body)))
	}
	if _, err = io.Copy(output, resp.RawBody()); err != nil {
		log.Logger().Fatal("failed to write response", zap.Error(err))
	}
}

func printJSONTable(cmd *cobra.Command, body []byte) {
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		printTable(cmd.OutOrStdout(), []string{"Response"}, [][]string{{strings.TrimSpace(string(body))}})
		return
	}
	printValueTables(cmd, value)
}

func printValueTables(cmd *cobra.Command, value any) {
	switch typed := value.(type) {
	case []any:
		printArrayTable(cmd, typed)
	case map[string]any:
		if _, ok := typed["UserId"]; ok {
			printObjectTable(cmd, typed)
			return
		}
		if _, ok := typed["ItemId"]; ok {
			printObjectTable(cmd, typed)
			return
		}
		arrayKeys := make([]string, 0)
		metadata := make(map[string]any)
		for key, child := range typed {
			if isPaginationCursorKey(key) {
				continue
			}
			if _, ok := child.([]any); ok {
				arrayKeys = append(arrayKeys, key)
			} else {
				metadata[key] = child
			}
		}
		sort.Strings(arrayKeys)
		if len(arrayKeys) > 0 {
			if len(metadata) > 0 {
				printObjectTable(cmd, metadata)
				fmt.Fprintln(cmd.OutOrStdout())
			}
			for i, key := range arrayKeys {
				if i > 0 {
					fmt.Fprintln(cmd.OutOrStdout())
				}
				printArrayTable(cmd, typed[key].([]any))
			}
			return
		}
		printObjectTable(cmd, typed)
	default:
		printTable(cmd.OutOrStdout(), []string{"Value"}, [][]string{{formatTableValue(typed)}})
	}
}

func isPaginationCursorKey(key string) bool {
	return strings.EqualFold(key, "cursor")
}

func printObjectTable(cmd *cobra.Command, object map[string]any) {
	output := cmd.OutOrStdout()
	if len(object) == 0 {
		fmt.Fprintln(output, "{}")
		return
	}

	keys := orderedObjectKeys(object, "UserId", "ItemId", "Comment", "Categories", "IsHidden", "Timestamp")
	for _, key := range keys {
		printObjectField(output, key, object[key])
	}
}

func orderedObjectKeys(object map[string]any, priority ...string) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	return orderedKeys(keys, priority...)
}

func orderedKeys(keys []string, priority ...string) []string {
	keySet := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		keySet[key] = struct{}{}
	}
	ordered := make([]string, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, key := range priority {
		if _, ok := keySet[key]; ok {
			ordered = append(ordered, key)
			seen[key] = struct{}{}
		}
	}
	remaining := make([]string, 0, len(keys)-len(ordered))
	for _, key := range keys {
		if _, ok := seen[key]; !ok {
			remaining = append(remaining, key)
		}
	}
	sort.Strings(remaining)
	return append(ordered, remaining...)
}

func printObjectField(output io.Writer, key string, value any) {
	if summary, ok := formatArraySummary(value); ok {
		fmt.Fprintf(output, "%s: %s\n", key, summary)
		return
	}
	switch value.(type) {
	case map[string]any:
		fmt.Fprintf(output, "%s:\n%s\n", key, formatPrettyJSON(value))
	case []any:
		fmt.Fprintf(output, "%s: %s\n", key, formatTableValue(value))
	default:
		fmt.Fprintf(output, "%s: %s\n", key, formatTableValue(value))
	}
}

func formatPrettyJSON(value any) string {
	encoded, err := json.MarshalIndent(summarizeLongArrays(value), "", "  ")
	if err != nil {
		return formatTableValue(value)
	}
	return string(encoded)
}

func summarizeLongArrays(value any) any {
	return summarizeLongArraysWithLimit(value, maxInlineArrayValues)
}

func summarizeLongArraysWithLimit(value any, maxValues int) any {
	if summary, ok := formatArraySummaryWithLimit(value, maxValues); ok {
		return summary
	}
	switch typed := value.(type) {
	case []any:
		values := make([]any, len(typed))
		for i, element := range typed {
			values[i] = summarizeLongArraysWithLimit(element, maxValues)
		}
		return values
	case map[string]any:
		values := make(map[string]any, len(typed))
		for key, element := range typed {
			values[key] = summarizeLongArraysWithLimit(element, maxValues)
		}
		return values
	default:
		return value
	}
}

func formatArraySummary(value any) (string, bool) {
	return formatArraySummaryWithLimit(value, maxInlineArrayValues)
}

func formatArraySummaryWithLimit(value any, maxValues int) (string, bool) {
	array, ok := value.([]any)
	if maxValues < 1 {
		maxValues = 1
	}
	if !ok || len(array) <= maxValues || !isScalarArray(array) {
		return "", false
	}
	values := make([]string, 0, maxValues)
	for _, element := range array[:maxValues] {
		values = append(values, formatSummaryValue(element))
	}
	return fmt.Sprintf("[%s, ...] (%d values)", strings.Join(values, ", "), len(array)), true
}

func isScalarArray(array []any) bool {
	for _, element := range array {
		switch element.(type) {
		case nil, string, json.Number, bool, float64:
		default:
			return false
		}
	}
	return true
}

func formatSummaryValue(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return formatTableValue(value)
	}
	return string(encoded)
}

func printArrayTable(cmd *cobra.Command, array []any) {
	if len(array) == 0 {
		printTable(cmd.OutOrStdout(), []string{"Result"}, [][]string{{"No data"}})
		return
	}
	allObjects := true
	columnsSet := make(map[string]struct{})
	objectRows := make([]map[string]any, 0, len(array))
	for _, element := range array {
		object, ok := element.(map[string]any)
		if !ok {
			allObjects = false
			break
		}
		object = formatProgressObject(object)
		objectRows = append(objectRows, object)
		for column := range object {
			columnsSet[column] = struct{}{}
		}
	}
	if !allObjects {
		rows := make([][]string, len(array))
		for i, element := range array {
			rows[i] = []string{strconv.Itoa(i), formatTableValue(element)}
		}
		printTable(cmd.OutOrStdout(), []string{"Index", "Value"}, rows)
		return
	}
	columns := make([]string, 0, len(columnsSet))
	for column := range columnsSet {
		columns = append(columns, column)
	}
	columns = orderedArrayColumns(columns)
	rows := make([][]string, len(objectRows))
	for i, object := range objectRows {
		row := make([]string, len(columns))
		for j, column := range columns {
			row[j] = formatTableCellValue(column, object[column])
		}
		rows[i] = row
	}
	printTable(cmd.OutOrStdout(), columns, rows)
}

func orderedArrayColumns(columns []string) []string {
	if lo.Contains(columns, "FeedbackType") {
		return orderedKeys(columns, "FeedbackType", "UserId", "ItemId", "Value", "Timestamp", "Comment", "Updated")
	}
	if lo.Contains(columns, "Progress") {
		return orderedKeys(columns, "Tracer", "Name", "Status", "Progress", "Error", "StartTime", "FinishTime")
	}
	return orderedKeys(columns, "UserId", "ItemId", "Comment", "Categories", "IsHidden", "Timestamp")
}

func formatProgressObject(object map[string]any) map[string]any {
	count, hasCount := numberValue(object["Count"])
	total, hasTotal := numberValue(object["Total"])
	if !hasCount || !hasTotal {
		return object
	}
	updated := make(map[string]any, len(object))
	for key, value := range object {
		if key == "Count" || key == "Total" {
			continue
		}
		updated[key] = value
	}
	updated["Progress"] = formatProgressBar(count, total)
	return updated
}

func numberValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func formatProgressBar(count, total float64) string {
	const width = 20
	if count < 0 {
		count = 0
	}
	if total < 0 {
		total = 0
	}
	if total > 0 && count > total {
		count = total
	}
	ratio := 0.0
	if total > 0 {
		ratio = count / total
	}
	filled := int(ratio*width + 0.5)
	if filled > width {
		filled = width
	}
	percent := int(ratio*100 + 0.5)
	return fmt.Sprintf("[%s%s] %d%%",
		strings.Repeat("#", filled),
		strings.Repeat("-", width-filled),
		percent)
}

func formatTableCellValue(column string, value any) string {
	if column != "Labels" {
		return formatTableValue(value)
	}
	switch value.(type) {
	case []any, map[string]any:
		encoded, err := json.Marshal(summarizeLongArraysWithLimit(value, maxTableLabelArrayValues))
		if err != nil {
			return formatTableValue(value)
		}
		return string(encoded)
	default:
		return formatTableValue(value)
	}
}

func formatTableValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		if formatted, ok := formatTimeValue(typed); ok {
			return formatted
		}
		return typed
	case json.Number:
		return typed.String()
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case []any:
		if summary, ok := formatArraySummary(typed); ok {
			return summary
		}
		encoded, err := json.Marshal(summarizeLongArrays(typed))
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(encoded)
	case map[string]any:
		encoded, err := json.Marshal(summarizeLongArrays(typed))
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(encoded)
	default:
		return fmt.Sprint(typed)
	}
}

func formatTimeValue(value string) (string, bool) {
	timestamp, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return "", false
	}
	return timestamp.Format(time.RFC3339), true
}

func printTable(output io.Writer, headers []string, rows [][]string) {
	table := tabwriter.NewWriter(output, 0, 8, 2, ' ', 0)
	for i, header := range headers {
		if i > 0 {
			_, _ = fmt.Fprint(table, "\t")
		}
		_, _ = fmt.Fprint(table, formatTableHeader(header))
	}
	_, _ = fmt.Fprintln(table)
	for _, row := range rows {
		for i, cell := range row {
			if i > 0 {
				_, _ = fmt.Fprint(table, "\t")
			}
			_, _ = fmt.Fprint(table, cell)
		}
		_, _ = fmt.Fprintln(table)
	}
	lo.Must0(table.Flush())
}

func formatTableHeader(header string) string {
	runes := []rune(header)
	var builder strings.Builder
	for i, ch := range runes {
		if ch == '.' || ch == '_' || ch == '-' || unicode.IsSpace(ch) {
			if builder.Len() > 0 {
				builder.WriteByte(' ')
			}
			continue
		}
		if i > 0 && shouldSplitTableHeader(runes, i) {
			builder.WriteByte(' ')
		}
		builder.WriteRune(unicode.ToUpper(ch))
	}
	return strings.Join(strings.Fields(builder.String()), "-")
}

func shouldSplitTableHeader(runes []rune, i int) bool {
	current := runes[i]
	previous := runes[i-1]
	if !unicode.IsUpper(current) {
		return false
	}
	if unicode.IsLower(previous) || unicode.IsDigit(previous) {
		return true
	}
	return unicode.IsUpper(previous) && i+1 < len(runes) && unicode.IsLower(runes[i+1])
}

func addAuthFlags(commands ...*cobra.Command) {
	for _, cmd := range commands {
		cmd.Flags().String("endpoint", "", "Gorse base URL (default: selected context or GORSE_ADMIN_ENDPOINT)")
		cmd.Flags().String("api-key", "", "Gorse admin API key (default: selected context or GORSE_ADMIN_API_KEY)")
		cmd.Flags().String("context", "", "Gorse CLI context name")
	}
}

func addPaginationFlags(cmd *cobra.Command) {
	cmd.Flags().IntP("n", "n", 0, "Number of returned records")
	cmd.Flags().Int("offset", 0, "Offset of returned records")
}

func addCategoryFlags(cmd *cobra.Command) {
	cmd.Flags().StringArray("category", nil, "Filter by category, repeatable")
}

func addQueryInt(cmd *cobra.Command, query url.Values, name string) {
	value, _ := cmd.Flags().GetInt(name)
	if value != 0 {
		query.Set(name, strconv.Itoa(value))
	}
}

func addQueryString(cmd *cobra.Command, query url.Values, flagName, queryName string) {
	value, _ := cmd.Flags().GetString(flagName)
	if value != "" {
		query.Set(queryName, value)
	}
}

func addQueryStringArray(cmd *cobra.Command, query url.Values, flagName, queryName string) {
	values, _ := cmd.Flags().GetStringArray(flagName)
	for _, value := range values {
		query.Add(queryName, value)
	}
}

func getIntFlag(cmd *cobra.Command, flagName string) int {
	value, _ := cmd.Flags().GetInt(flagName)
	return value
}

func getNFlag(cmd *cobra.Command) int {
	n := getIntFlag(cmd, "n")
	if n == 0 {
		return 10
	}
	return n
}

func getStringFlag(cmd *cobra.Command, flagName string) string {
	value, _ := cmd.Flags().GetString(flagName)
	return value
}

func getStringArrayFlag(cmd *cobra.Command, flagName string) []string {
	values, _ := cmd.Flags().GetStringArray(flagName)
	return values
}

func readFlagOrFile(cmd *cobra.Command, valueFlag, fileFlag string) string {
	value, _ := cmd.Flags().GetString(valueFlag)
	if value != "" {
		return value
	}
	file, _ := cmd.Flags().GetString(fileFlag)
	if file == "" {
		return ""
	}
	content, err := os.ReadFile(file)
	if err != nil {
		log.Logger().Fatal("failed to read file", zap.String("file", file), zap.Error(err))
	}
	return string(content)
}

// getCmd is the parent command for get operations
var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get resources from Gorse admin API",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

var recommendCmd = &cobra.Command{
	Use:   "recommend",
	Short: "Get recommendations from Gorse",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

var pipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Manage recommendation pipeline configuration",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

// getClusterCmd gets cluster nodes from the admin API
var getClusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Get cluster nodes from Gorse admin API",
	Run: func(cmd *cobra.Command, args []string) {
		getAdminAPI(cmd, "/dashboard/cluster", nil)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get task progress from Gorse admin API",
	Run: func(cmd *cobra.Command, args []string) {
		getAdminAPI(cmd, "/dashboard/tasks", nil)
	},
}

var pipelineGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get recommendation pipeline configuration",
	Run: func(cmd *cobra.Command, args []string) {
		getAdminAPI(cmd, "/dashboard/config", nil)
	},
}

var dumpCmd = &cobra.Command{
	Use:   "dump <file>",
	Short: "Dump all Gorse data as a binary backup",
	Example: `  # Dump data to a backup file
  gorse-cli dump backup.bin`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		outputPath := args[0]
		file, err := os.Create(outputPath)
		if err != nil {
			log.Logger().Fatal("failed to create dump file", zap.String("file", outputPath), zap.Error(err))
		}
		defer file.Close()
		downloadAdminAPI(cmd, "/dump", file)
		fmt.Fprintln(cmd.OutOrStdout(), "Data dumped to "+outputPath)
	},
}

var restoreCmd = &cobra.Command{
	Use:   "restore <file>",
	Short: "Restore Gorse data from a binary backup",
	Example: `  # Restore data from a backup file
  gorse-cli restore backup.bin`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		confirmed := confirmRestore(cmd, args[0])
		if !confirmed {
			fmt.Fprintln(cmd.OutOrStdout(), "Restore canceled")
			return
		}
		file, err := os.Open(args[0])
		if err != nil {
			log.Logger().Fatal("failed to open restore file", zap.String("file", args[0]), zap.Error(err))
		}
		defer file.Close()
		postAdminAPIBody(cmd, "/restore", "application/octet-stream", file)
	},
}

func confirmRestore(cmd *cobra.Command, file string) bool {
	fmt.Fprintf(cmd.OutOrStdout(), "Restore data from %s? Existing users, items, feedback, and cache will be overwritten. Confirm [y/N]: ", file)
	input, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		log.Logger().Fatal("failed to read confirmation", zap.Error(err))
	}
	return strings.EqualFold(strings.TrimSpace(input), "y")
}

var pipelineSetCmd = &cobra.Command{
	Use:   "set [key=value]...",
	Short: "Set recommendation pipeline configuration values",
	Example: `  # Set single config value
  gorse-cli pipeline set recommend.cache_size=1000

  # Set multiple config values
  gorse-cli pipeline set recommend.cache_size=1000 recommend.item_ttl=72h`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		endpoint, apiKey := requireEndpointAndKey(cmd)
		client := newAdminClient(endpoint, apiKey)

		resp, err := client.R().Get("/dashboard/config")
		if err != nil {
			log.Logger().Fatal("failed to send request", zap.Error(err))
		}
		if resp.IsError() {
			log.Logger().Fatal("API request failed",
				zap.Int("status", resp.StatusCode()),
				zap.String("body", resp.String()))
		}

		var configPatch map[string]interface{}
		if err = json.Unmarshal(resp.Body(), &configPatch); err != nil {
			log.Logger().Fatal("failed to parse config", zap.Error(err))
		}

		// Apply config patch from arguments.
		for _, arg := range args {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) != 2 {
				log.Logger().Fatal("invalid config format, expected key=value", zap.String("arg", arg))
			}
			key := parts[0]
			value := parts[1]

			// Parse the value
			if err = setConfigValue(configPatch, key, parseConfigValue(value)); err != nil {
				log.Logger().Fatal("failed to set config value", zap.String("key", key), zap.Error(err))
			}
		}

		resp, err = client.R().
			SetHeader("Content-Type", "application/json").
			SetBody(configPatch).
			Post("/dashboard/config")
		if err != nil {
			log.Logger().Fatal("failed to send request", zap.Error(err))
		}

		if resp.IsError() {
			log.Logger().Fatal("API request failed",
				zap.Int("status", resp.StatusCode()),
				zap.String("body", resp.String()))
		}

		printJSONTable(cmd, resp.Body())
	},
}

var pipelineResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset recommendation pipeline configuration to the file defaults",
	Run: func(cmd *cobra.Command, args []string) {
		body := deleteAdminAPI(cmd, "/dashboard/config")
		bodyText := strings.TrimSpace(string(body))
		if bodyText != "" && bodyText != "{}" {
			printJSONTable(cmd, body)
			return
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Pipeline reset to defaults.")
	},
}

var getCategoriesCmd = &cobra.Command{
	Use:   "categories",
	Short: "Get item categories",
	Run: func(cmd *cobra.Command, args []string) {
		getAdminAPI(cmd, "/dashboard/categories", nil)
	},
}

var getStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Get global statistics",
	Run: func(cmd *cobra.Command, args []string) {
		getAdminAPI(cmd, "/dashboard/stats", nil)
	},
}

var getTimeseriesCmd = &cobra.Command{
	Use:   "timeseries <name>",
	Short: "Get time series data",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := url.Values{}
		addQueryString(cmd, query, "begin", "begin")
		addQueryString(cmd, query, "end", "end")
		addQueryString(cmd, query, "duration", "duration")
		getAdminAPI(cmd, "/dashboard/timeseries/"+url.PathEscape(args[0]), query)
	},
}

var getUserCmd = &cobra.Command{
	Use:   "user <user-id>",
	Short: "Get user details",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		user, err := newGorseClient(cmd).GetUser(cmd.Context(), args[0])
		if err != nil {
			log.Logger().Fatal("API request failed", zap.Error(err))
		}
		printResultTable(cmd, user)
	},
}

var getItemCmd = &cobra.Command{
	Use:   "item <item-id>",
	Short: "Get item details",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		item, err := newGorseClient(cmd).GetItem(cmd.Context(), args[0])
		if err != nil {
			log.Logger().Fatal("API request failed", zap.Error(err))
		}
		printResultTable(cmd, item)
	},
}

var getUsersCmd = &cobra.Command{
	Use:   "users",
	Short: "Get users",
	Run: func(cmd *cobra.Command, args []string) {
		users, err := newGorseClient(cmd).GetUsers(cmd.Context(), getNFlag(cmd), "")
		if err != nil {
			log.Logger().Fatal("API request failed", zap.Error(err))
		}
		printResultTable(cmd, users)
	},
}

var getItemsCmd = &cobra.Command{
	Use:   "items",
	Short: "Get items",
	Run: func(cmd *cobra.Command, args []string) {
		items, err := newGorseClient(cmd).GetItems(cmd.Context(), getNFlag(cmd), "")
		if err != nil {
			log.Logger().Fatal("API request failed", zap.Error(err))
		}
		printResultTable(cmd, items)
	},
}

var getFeedbackCmd = &cobra.Command{
	Use:   "feedback",
	Short: "Get feedback",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		printFeedback(cmd, getStringFlag(cmd, "type"), getStringFlag(cmd, "user"), getStringFlag(cmd, "item"), getNFlag(cmd))
	},
}

func printFeedback(cmd *cobra.Command, feedbackType, userID, itemID string, n int) {
	path, query := feedbackRequest(feedbackType, userID, itemID, n)
	body := getAdminAPIBody(cmd, path, query)
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		printTable(cmd.OutOrStdout(), []string{"Response"}, [][]string{{strings.TrimSpace(string(body))}})
		return
	}
	feedback, ok := feedbackRecords(value)
	if !ok {
		printValueTables(cmd, value)
		return
	}
	sortFeedbackRecords(feedback)
	if n > 0 && n < len(feedback) {
		feedback = feedback[:n]
	}
	printValueTables(cmd, feedback)
}

func feedbackRequest(feedbackType, userID, itemID string, n int) (string, url.Values) {
	query := url.Values{}
	if userID == "" && itemID == "" && n > 0 {
		query.Set("n", strconv.Itoa(n))
	}
	switch {
	case userID != "" && itemID != "" && feedbackType != "":
		return "/feedback/" + url.PathEscape(feedbackType) + "/" + url.PathEscape(userID) + "/" + url.PathEscape(itemID), query
	case userID != "" && itemID != "":
		return "/feedback/" + url.PathEscape(userID) + "/" + url.PathEscape(itemID), query
	case userID != "" && feedbackType != "":
		return "/user/" + url.PathEscape(userID) + "/feedback/" + url.PathEscape(feedbackType), query
	case userID != "":
		return "/user/" + url.PathEscape(userID) + "/feedback", query
	case itemID != "" && feedbackType != "":
		return "/item/" + url.PathEscape(itemID) + "/feedback/" + url.PathEscape(feedbackType), query
	case itemID != "":
		return "/item/" + url.PathEscape(itemID) + "/feedback/", query
	case feedbackType != "":
		return "/feedback/" + url.PathEscape(feedbackType), query
	default:
		return "/feedback", query
	}
}

func feedbackRecords(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, true
	case map[string]any:
		if feedback, ok := typed["Feedback"].([]any); ok {
			return feedback, true
		}
		if _, ok := typed["FeedbackType"]; ok {
			return []any{typed}, true
		}
	}
	return nil, false
}

func sortFeedbackRecords(feedback []any) {
	sort.SliceStable(feedback, func(i, j int) bool {
		return feedbackTimestamp(feedback[i]) > feedbackTimestamp(feedback[j])
	})
}

func feedbackTimestamp(value any) string {
	object, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	timestamp, _ := object["Timestamp"].(string)
	return timestamp
}

var getLatestCmd = &cobra.Command{
	Use:   "latest",
	Short: "Get latest items",
	Run: func(cmd *cobra.Command, args []string) {
		categories := getStringArrayFlag(cmd, "category")
		if len(categories) <= 1 {
			var category string
			if len(categories) == 1 {
				category = categories[0]
			}
			latest, err := newGorseClient(cmd).GetLatestItems(cmd.Context(), "", category, getNFlag(cmd), getIntFlag(cmd, "offset"))
			if err != nil {
				log.Logger().Fatal("API request failed", zap.Error(err))
			}
			printResultTable(cmd, latest)
			return
		}
		query := url.Values{}
		addQueryInt(cmd, query, "n")
		addQueryInt(cmd, query, "offset")
		addQueryStringArray(cmd, query, "category", "category")
		getAdminAPI(cmd, "/dashboard/latest", query)
	},
}

var getNonPersonalizedCmd = &cobra.Command{
	Use:   "non-personalized <name>",
	Short: "Get non-personalized recommendations",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := url.Values{}
		addQueryInt(cmd, query, "n")
		addQueryInt(cmd, query, "offset")
		addQueryString(cmd, query, "user-id", "user-id")
		addQueryStringArray(cmd, query, "category", "category")
		getAdminAPI(cmd, "/dashboard/non-personalized/"+url.PathEscape(args[0]), query)
	},
}

var recommendUserCmd = &cobra.Command{
	Use:   "user <user-id> [recommender] [name]",
	Short: "Get recommendations for a user",
	Args:  cobra.RangeArgs(1, 3),
	Run: func(cmd *cobra.Command, args []string) {
		categories := getStringArrayFlag(cmd, "category")
		if len(args) == 1 && len(categories) <= 1 {
			var category string
			if len(categories) == 1 {
				category = categories[0]
			}
			recommend, err := newGorseClient(cmd).GetRecommend(cmd.Context(), args[0], category, getNFlag(cmd), 0)
			if err != nil {
				log.Logger().Fatal("API request failed", zap.Error(err))
			}
			printResultTable(cmd, recommend)
			return
		}
		query := url.Values{}
		addQueryInt(cmd, query, "n")
		addQueryStringArray(cmd, query, "category", "category")
		path := "/dashboard/recommend/" + url.PathEscape(args[0])
		if len(args) > 1 {
			path += "/" + url.PathEscape(args[1])
		}
		if len(args) > 2 {
			path += "/" + url.PathEscape(args[2])
		}
		getAdminAPI(cmd, path, query)
	},
}

var getItemToItemCmd = &cobra.Command{
	Use:   "item-to-item <name> <item-id>",
	Short: "Get item-to-item recommendations",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		query := url.Values{}
		addQueryInt(cmd, query, "n")
		addQueryInt(cmd, query, "offset")
		addQueryStringArray(cmd, query, "category", "category")
		getAdminAPI(cmd, "/dashboard/item-to-item/"+url.PathEscape(args[0])+"/"+url.PathEscape(args[1]), query)
	},
}

var getUserToUserCmd = &cobra.Command{
	Use:   "user-to-user <name> <user-id>",
	Short: "Get user-to-user recommendations",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		query := url.Values{}
		addQueryInt(cmd, query, "n")
		addQueryInt(cmd, query, "offset")
		getAdminAPI(cmd, "/dashboard/user-to-user/"+url.PathEscape(args[0])+"/"+url.PathEscape(args[1]), query)
	},
}

var getExternalCmd = &cobra.Command{
	Use:   "external",
	Short: "Preview external recommendations",
	Run: func(cmd *cobra.Command, args []string) {
		script := readFlagOrFile(cmd, "script", "script-file")
		if script == "" {
			log.Logger().Fatal("--script or --script-file is required")
		}
		query := url.Values{}
		query.Set("script", base64.StdEncoding.EncodeToString([]byte(script)))
		addQueryString(cmd, query, "user-id", "user-id")
		getAdminAPI(cmd, "/dashboard/external", query)
	},
}

var getRankerPromptCmd = &cobra.Command{
	Use:   "ranker-prompt",
	Short: "Render ranker prompt preview",
	Run: func(cmd *cobra.Command, args []string) {
		queryTemplate := readFlagOrFile(cmd, "query-template", "query-template-file")
		documentTemplate := readFlagOrFile(cmd, "document-template", "document-template-file")
		userID, _ := cmd.Flags().GetString("user-id")
		if queryTemplate == "" || documentTemplate == "" || userID == "" {
			log.Logger().Fatal("--query-template/--query-template-file, --document-template/--document-template-file, and --user-id are required")
		}
		query := url.Values{}
		query.Set("query-template", base64.StdEncoding.EncodeToString([]byte(queryTemplate)))
		query.Set("document-template", base64.StdEncoding.EncodeToString([]byte(documentTemplate)))
		query.Set("user-id", userID)
		getAdminAPI(cmd, "/dashboard/ranker/prompt", query)
	},
}

func setConfigValue(config map[string]interface{}, key string, value interface{}) error {
	parts := strings.Split(key, ".")
	if len(parts) == 0 {
		return fmt.Errorf("empty config key")
	}
	for _, part := range parts {
		if part == "" {
			return fmt.Errorf("invalid config key %q", key)
		}
	}
	current := config
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part]
		if !ok {
			child := make(map[string]interface{})
			current[part] = child
			current = child
			continue
		}
		child, ok := next.(map[string]interface{})
		if !ok {
			return fmt.Errorf("config key %q is not an object", part)
		}
		current = child
	}
	current[parts[len(parts)-1]] = value
	return nil
}

// parseConfigValue parses a string value into appropriate type
func parseConfigValue(value string) interface{} {
	// Try parsing as bool
	if value == "true" {
		return true
	}
	if value == "false" {
		return false
	}

	// Try parsing as int
	if intVal, err := strconv.Atoi(value); err == nil {
		return intVal
	}

	// Try parsing as float
	if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
		return floatVal
	}

	// Return as string
	return value
}

func init() {
	rootCmd.PersistentFlags().BoolP("version", "v", false, "gorse-cli version")
	rootCmd.AddCommand(contextCmd)
	contextCmd.AddCommand(contextAddCmd)
	contextCmd.AddCommand(contextListCmd)
	contextCmd.AddCommand(contextUseCmd)
	contextCmd.AddCommand(contextDeleteCmd)
	contextCmd.AddCommand(contextCurrentCmd)
	rootCmd.AddCommand(getCmd)
	getCmd.AddCommand(getClusterCmd)
	getCmd.AddCommand(getCategoriesCmd)
	getCmd.AddCommand(getStatsCmd)
	getCmd.AddCommand(getTimeseriesCmd)
	getCmd.AddCommand(getUserCmd)
	getCmd.AddCommand(getUsersCmd)
	getCmd.AddCommand(getItemCmd)
	getCmd.AddCommand(getItemsCmd)
	getCmd.AddCommand(getFeedbackCmd)
	rootCmd.AddCommand(recommendCmd)
	recommendCmd.AddCommand(recommendUserCmd)
	recommendCmd.AddCommand(getLatestCmd)
	recommendCmd.AddCommand(getNonPersonalizedCmd)
	recommendCmd.AddCommand(getItemToItemCmd)
	recommendCmd.AddCommand(getUserToUserCmd)
	recommendCmd.AddCommand(getExternalCmd)
	recommendCmd.AddCommand(getRankerPromptCmd)

	rootCmd.AddCommand(pipelineCmd)
	pipelineCmd.AddCommand(pipelineGetCmd)
	pipelineCmd.AddCommand(pipelineSetCmd)
	pipelineCmd.AddCommand(pipelineResetCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(dumpCmd)
	rootCmd.AddCommand(restoreCmd)

	contextAddCmd.Flags().String("endpoint", "", "Gorse base URL (default: GORSE_ADMIN_ENDPOINT)")
	contextAddCmd.Flags().String("api-key", "", "Gorse admin API key (default: GORSE_ADMIN_API_KEY; prefer prompt to avoid shell history)")
	addAuthFlags(getClusterCmd, getCategoriesCmd, statusCmd,
		getStatsCmd, getTimeseriesCmd, getUserCmd, getUsersCmd, getItemCmd, getItemsCmd, getFeedbackCmd, getLatestCmd,
		getNonPersonalizedCmd, recommendUserCmd, getItemToItemCmd, getUserToUserCmd, getExternalCmd, getRankerPromptCmd,
		pipelineGetCmd, pipelineSetCmd, pipelineResetCmd, dumpCmd, restoreCmd)

	getTimeseriesCmd.Flags().String("begin", "", "Begin time, defaults to 7 days ago")
	getTimeseriesCmd.Flags().String("end", "", "End time, defaults to now")
	getTimeseriesCmd.Flags().String("duration", "", "Aggregation duration, defaults to 24h")
	getUsersCmd.Flags().IntP("n", "n", 0, "Number of returned users")
	getItemsCmd.Flags().IntP("n", "n", 0, "Number of returned items")
	getFeedbackCmd.Flags().IntP("n", "n", 0, "Number of returned feedback")
	getFeedbackCmd.Flags().String("type", "", "Filter by feedback type")
	getFeedbackCmd.Flags().String("user", "", "Filter by user ID")
	getFeedbackCmd.Flags().String("item", "", "Filter by item ID")
	addPaginationFlags(getLatestCmd)
	addCategoryFlags(getLatestCmd)
	addPaginationFlags(getNonPersonalizedCmd)
	addCategoryFlags(getNonPersonalizedCmd)
	getNonPersonalizedCmd.Flags().String("user-id", "", "Remove read items of a user")
	recommendUserCmd.Flags().IntP("n", "n", 0, "Number of returned items")
	addCategoryFlags(recommendUserCmd)
	addPaginationFlags(getItemToItemCmd)
	addCategoryFlags(getItemToItemCmd)
	addPaginationFlags(getUserToUserCmd)
	getExternalCmd.Flags().String("script", "", "External recommender script")
	getExternalCmd.Flags().String("script-file", "", "Path to external recommender script")
	getExternalCmd.Flags().String("user-id", "", "User ID")
	getRankerPromptCmd.Flags().String("query-template", "", "Ranker query template")
	getRankerPromptCmd.Flags().String("query-template-file", "", "Path to ranker query template")
	getRankerPromptCmd.Flags().String("document-template", "", "Ranker document template")
	getRankerPromptCmd.Flags().String("document-template-file", "", "Path to ranker document template")
	getRankerPromptCmd.Flags().String("user-id", "", "User ID")

}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
