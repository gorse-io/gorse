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

	"github.com/go-resty/resty/v2"
	"github.com/gorse-io/gorse/cmd/version"
	"github.com/gorse-io/gorse/common/log"
	"github.com/olekukonko/tablewriter"
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
	keyringService     = "gorse-cli"
	keyringEndpointKey = "admin-endpoint"
	keyringAPIKeyKey   = "admin-api-key"
)

var (
	keyringGet    = keyring.Get
	keyringSet    = keyring.Set
	keyringDelete = keyring.Delete
)

// newAdminClient creates a new resty client for admin API
func newAdminClient(endpoint, apiKey string) *resty.Client {
	client := resty.New()
	client.SetBaseURL(endpoint)
	client.SetHeader("X-Api-Key", apiKey)
	return client
}

// getEndpointAndKey returns the endpoint and API key from flags, environment variables, or keyring.
func getEndpointAndKey(cmd *cobra.Command) (endpoint, apiKey string) {
	endpoint, _ = cmd.Flags().GetString("endpoint")
	apiKey, _ = cmd.Flags().GetString("api-key")
	if endpoint == "" {
		endpoint = os.Getenv("GORSE_ADMIN_ENDPOINT")
	}
	if apiKey == "" {
		apiKey = os.Getenv("GORSE_ADMIN_API_KEY")
	}
	if endpoint == "" {
		endpoint, _ = keyringGet(keyringService, keyringEndpointKey)
	}
	if apiKey == "" {
		apiKey, _ = keyringGet(keyringService, keyringAPIKeyKey)
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

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Save Gorse admin API credentials to the system keyring",
	Run: func(cmd *cobra.Command, args []string) {
		endpoint := getFlagOrEnv(cmd, "endpoint", "GORSE_ADMIN_ENDPOINT")
		apiKey := getFlagOrEnv(cmd, "api-key", "GORSE_ADMIN_API_KEY")

		if endpoint == "" {
			fmt.Fprint(os.Stderr, "Gorse admin API endpoint: ")
			input, err := bufio.NewReader(os.Stdin).ReadString('\n')
			if err != nil {
				log.Logger().Fatal("failed to read endpoint", zap.Error(err))
			}
			endpoint = strings.TrimSpace(input)
		}
		if endpoint == "" {
			log.Logger().Fatal("GORSE_ADMIN_ENDPOINT or --endpoint is required")
		}

		if apiKey == "" {
			var err error
			apiKey, err = readSecret("Gorse admin API key: ")
			if err != nil {
				log.Logger().Fatal("failed to read API key", zap.Error(err))
			}
		}
		if apiKey == "" {
			log.Logger().Fatal("GORSE_ADMIN_API_KEY or --api-key is required")
		}

		if err := keyringSet(keyringService, keyringEndpointKey, endpoint); err != nil {
			log.Logger().Fatal("failed to save endpoint to keyring", zap.Error(err))
		}
		if err := keyringSet(keyringService, keyringAPIKeyKey, apiKey); err != nil {
			log.Logger().Fatal("failed to save API key to keyring", zap.Error(err))
		}
		printStatusTable(cmd, "login", "Credentials saved to system keyring.")
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove saved Gorse admin API credentials from the system keyring",
	Run: func(cmd *cobra.Command, args []string) {
		if err := keyringDelete(keyringService, keyringEndpointKey); err != nil && !errors.Is(err, keyring.ErrNotFound) {
			log.Logger().Fatal("failed to delete endpoint from keyring", zap.Error(err))
		}
		if err := keyringDelete(keyringService, keyringAPIKeyKey); err != nil && !errors.Is(err, keyring.ErrNotFound) {
			log.Logger().Fatal("failed to delete API key from keyring", zap.Error(err))
		}
		printStatusTable(cmd, "logout", "Credentials removed from system keyring.")
	},
}

func requireEndpointAndKey(cmd *cobra.Command) (string, string) {
	endpoint, apiKey := getEndpointAndKey(cmd)
	if endpoint == "" {
		log.Logger().Fatal("GORSE_ADMIN_ENDPOINT, --endpoint, or saved login endpoint is required")
	}
	if apiKey == "" {
		log.Logger().Fatal("GORSE_ADMIN_API_KEY, --api-key, or saved login API key is required")
	}
	return endpoint, apiKey
}

func getAdminAPI(cmd *cobra.Command, path string, query url.Values) {
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
	printJSONTable(cmd, resp.Body())
}

func postAdminAPIBody(cmd *cobra.Command, path, contentType string, reader io.Reader) {
	endpoint, apiKey := requireEndpointAndKey(cmd)
	client := newAdminClient(endpoint, apiKey)
	resp, err := client.R().
		SetHeader("Content-Type", contentType).
		SetBody(reader).
		Post(path)
	if err != nil {
		log.Logger().Fatal("failed to send request", zap.Error(err))
	}
	if resp.IsError() {
		log.Logger().Fatal("API request failed",
			zap.Int("status", resp.StatusCode()),
			zap.String("body", resp.String()))
	}
	printJSONTable(cmd, resp.Body())
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

func printStatusTable(cmd *cobra.Command, action, message string) {
	printTable(cmd.OutOrStdout(), []string{"Action", "Message"}, [][]string{{action, message}})
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
		arrayKeys := make([]string, 0)
		metadata := make(map[string]any)
		for key, child := range typed {
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

func printObjectTable(cmd *cobra.Command, object map[string]any) {
	flattened := make(map[string]string)
	flattenJSON("", object, flattened)
	keys := make([]string, 0, len(flattened))
	for key := range flattened {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([][]string, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, []string{key, flattened[key]})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"", ""})
	}
	printTable(cmd.OutOrStdout(), []string{"Key", "Value"}, rows)
}

func printArrayTable(cmd *cobra.Command, array []any) {
	if len(array) == 0 {
		printTable(cmd.OutOrStdout(), []string{"Result"}, [][]string{{"No data"}})
		return
	}
	allObjects := true
	columnsSet := make(map[string]struct{})
	flattenedRows := make([]map[string]string, 0, len(array))
	for _, element := range array {
		object, ok := element.(map[string]any)
		if !ok {
			allObjects = false
			break
		}
		flattened := make(map[string]string)
		flattenJSON("", object, flattened)
		flattenedRows = append(flattenedRows, flattened)
		for column := range flattened {
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
	sort.Strings(columns)
	rows := make([][]string, len(flattenedRows))
	for i, flattened := range flattenedRows {
		row := make([]string, len(columns))
		for j, column := range columns {
			row[j] = flattened[column]
		}
		rows[i] = row
	}
	printTable(cmd.OutOrStdout(), columns, rows)
}

func flattenJSON(prefix string, value any, output map[string]string) {
	switch typed := value.(type) {
	case map[string]any:
		if len(typed) == 0 {
			output[prefix] = "{}"
			return
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			next := key
			if prefix != "" {
				next = prefix + "." + key
			}
			flattenJSON(next, typed[key], output)
		}
	default:
		output[prefix] = formatTableValue(typed)
	}
}

func formatTableValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case json.Number:
		return typed.String()
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case []any, map[string]any:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(encoded)
	default:
		return fmt.Sprint(typed)
	}
}

func printTable(output io.Writer, headers []string, rows [][]string) {
	table := tablewriter.NewWriter(output)
	table.Header(headers)
	lo.Must0(table.Bulk(rows))
	lo.Must0(table.Render())
}

func addAuthFlags(commands ...*cobra.Command) {
	for _, cmd := range commands {
		cmd.Flags().String("endpoint", "", "Gorse admin API endpoint (default: GORSE_ADMIN_ENDPOINT, then keyring)")
		cmd.Flags().String("api-key", "", "Gorse admin API key (default: GORSE_ADMIN_API_KEY, then keyring)")
	}
}

func addPaginationFlags(cmd *cobra.Command) {
	cmd.Flags().Int("n", 0, "Number of returned records")
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
}

// getClusterCmd gets cluster nodes from the admin API
var getClusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Get cluster nodes from Gorse admin API",
	Run: func(cmd *cobra.Command, args []string) {
		getAdminAPI(cmd, "/dashboard/cluster", nil)
	},
}

// getTasksCmd gets tasks from the admin API
var getTasksCmd = &cobra.Command{
	Use:   "tasks",
	Short: "Get task progress from Gorse admin API",
	Run: func(cmd *cobra.Command, args []string) {
		getAdminAPI(cmd, "/dashboard/tasks", nil)
	},
}

// getConfigCmd gets configuration from the admin API
var getConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Get configuration from Gorse admin API",
	Run: func(cmd *cobra.Command, args []string) {
		getAdminAPI(cmd, "/dashboard/config", nil)
	},
}

// setCmd is the parent command for set operations
var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Set resources in Gorse admin API",
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
		printStatusTable(cmd, "dump", "Data dumped to "+outputPath)
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
			printStatusTable(cmd, "restore", "Restore canceled")
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

// setConfigCmd sets configuration via the admin API
var setConfigCmd = &cobra.Command{
	Use:   "config [key=value]...",
	Short: "Set configuration values in Gorse admin API",
	Example: `  # Set single config value
  gorse-cli set config recommend.cache_size=1000

  # Set multiple config values
  gorse-cli set config recommend.cache_size=1000 recommend.item_ttl=72h`,
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

var resetConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Reset recommendation configuration to the file defaults",
	Run: func(cmd *cobra.Command, args []string) {
		body := deleteAdminAPI(cmd, "/dashboard/config")
		bodyText := strings.TrimSpace(string(body))
		if bodyText != "" && bodyText != "{}" {
			printJSONTable(cmd, body)
			return
		}
		printStatusTable(cmd, "reset config", "Configuration reset to defaults.")
	},
}

var getUserInfoCmd = &cobra.Command{
	Use:   "userinfo",
	Short: "Get current dashboard login user information",
	Run: func(cmd *cobra.Command, args []string) {
		getAdminAPI(cmd, "/dashboard/userinfo", nil)
	},
}

var getCategoriesCmd = &cobra.Command{
	Use:   "categories",
	Short: "Get item categories",
	Run: func(cmd *cobra.Command, args []string) {
		getAdminAPI(cmd, "/dashboard/categories", nil)
	},
}

var getConfigSchemaCmd = &cobra.Command{
	Use:   "config-schema",
	Short: "Get configuration JSON schema",
	Run: func(cmd *cobra.Command, args []string) {
		getAdminAPI(cmd, "/dashboard/config/schema", nil)
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
		getAdminAPI(cmd, "/dashboard/user/"+url.PathEscape(args[0]), nil)
	},
}

var getItemCmd = &cobra.Command{
	Use:   "item <item-id>",
	Short: "Get item details",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		getAdminAPI(cmd, "/item/"+url.PathEscape(args[0]), nil)
	},
}

var getUsersCmd = &cobra.Command{
	Use:   "users",
	Short: "Get users",
	Run: func(cmd *cobra.Command, args []string) {
		query := url.Values{}
		addQueryInt(cmd, query, "n")
		addQueryString(cmd, query, "cursor", "cursor")
		getAdminAPI(cmd, "/dashboard/users", query)
	},
}

var getItemsCmd = &cobra.Command{
	Use:   "items",
	Short: "Get items",
	Run: func(cmd *cobra.Command, args []string) {
		query := url.Values{}
		addQueryInt(cmd, query, "n")
		addQueryString(cmd, query, "cursor", "cursor")
		getAdminAPI(cmd, "/items", query)
	},
}

var getUserFeedbackCmd = &cobra.Command{
	Use:   "user-feedback <user-id> [feedback-type]",
	Short: "Get user feedback",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		query := url.Values{}
		addQueryInt(cmd, query, "n")
		addQueryInt(cmd, query, "offset")
		path := "/dashboard/user/" + url.PathEscape(args[0]) + "/feedback/"
		if len(args) > 1 {
			path += url.PathEscape(args[1])
		}
		getAdminAPI(cmd, path, query)
	},
}

var getLatestCmd = &cobra.Command{
	Use:   "latest",
	Short: "Get latest items",
	Run: func(cmd *cobra.Command, args []string) {
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

var getRecommendCmd = &cobra.Command{
	Use:   "recommend <user-id> [recommender] [name]",
	Short: "Get recommendations for a user",
	Args:  cobra.RangeArgs(1, 3),
	Run: func(cmd *cobra.Command, args []string) {
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
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
	rootCmd.AddCommand(getCmd)
	getCmd.AddCommand(getUserInfoCmd)
	getCmd.AddCommand(getClusterCmd)
	getCmd.AddCommand(getCategoriesCmd)
	getCmd.AddCommand(getTasksCmd)
	getCmd.AddCommand(getConfigCmd)
	getCmd.AddCommand(getConfigSchemaCmd)
	getCmd.AddCommand(getStatsCmd)
	getCmd.AddCommand(getTimeseriesCmd)
	getCmd.AddCommand(getUserCmd)
	getCmd.AddCommand(getUsersCmd)
	getCmd.AddCommand(getItemCmd)
	getCmd.AddCommand(getItemsCmd)
	getCmd.AddCommand(getUserFeedbackCmd)
	getCmd.AddCommand(getLatestCmd)
	getCmd.AddCommand(getNonPersonalizedCmd)
	getCmd.AddCommand(getRecommendCmd)
	getCmd.AddCommand(getItemToItemCmd)
	getCmd.AddCommand(getUserToUserCmd)
	getCmd.AddCommand(getExternalCmd)
	getCmd.AddCommand(getRankerPromptCmd)

	rootCmd.AddCommand(setCmd)
	setCmd.AddCommand(setConfigCmd)
	resetCmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset resources in Gorse admin API",
	}
	rootCmd.AddCommand(resetCmd)
	resetCmd.AddCommand(resetConfigCmd)
	rootCmd.AddCommand(dumpCmd)
	rootCmd.AddCommand(restoreCmd)

	// Add flags for endpoint and api-key
	loginCmd.Flags().String("endpoint", "", "Gorse admin API endpoint (default: GORSE_ADMIN_ENDPOINT)")
	loginCmd.Flags().String("api-key", "", "Gorse admin API key (default: GORSE_ADMIN_API_KEY; prefer prompt to avoid shell history)")
	addAuthFlags(getUserInfoCmd, getClusterCmd, getCategoriesCmd, getTasksCmd, getConfigCmd, getConfigSchemaCmd,
		getStatsCmd, getTimeseriesCmd, getUserCmd, getUsersCmd, getItemCmd, getItemsCmd, getUserFeedbackCmd, getLatestCmd,
		getNonPersonalizedCmd, getRecommendCmd, getItemToItemCmd, getUserToUserCmd, getExternalCmd, getRankerPromptCmd,
		setConfigCmd, resetConfigCmd, dumpCmd, restoreCmd)

	getTimeseriesCmd.Flags().String("begin", "", "Begin time, defaults to 7 days ago")
	getTimeseriesCmd.Flags().String("end", "", "End time, defaults to now")
	getTimeseriesCmd.Flags().String("duration", "", "Aggregation duration, defaults to 24h")
	getUsersCmd.Flags().Int("n", 0, "Number of returned users")
	getUsersCmd.Flags().String("cursor", "", "Cursor for next page")
	getItemsCmd.Flags().Int("n", 0, "Number of returned items")
	getItemsCmd.Flags().String("cursor", "", "Cursor for next page")
	addPaginationFlags(getUserFeedbackCmd)
	addPaginationFlags(getLatestCmd)
	addCategoryFlags(getLatestCmd)
	addPaginationFlags(getNonPersonalizedCmd)
	addCategoryFlags(getNonPersonalizedCmd)
	getNonPersonalizedCmd.Flags().String("user-id", "", "Remove read items of a user")
	getRecommendCmd.Flags().Int("n", 0, "Number of returned items")
	addCategoryFlags(getRecommendCmd)
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
		log.Logger().Fatal("failed to execute command", zap.Error(err))
	}
}
