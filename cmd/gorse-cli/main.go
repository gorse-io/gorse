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
	"text/tabwriter"
	"time"
	"unicode"

	gorse "github.com/gorse-io/gorse-go"
	"github.com/gorse-io/gorse/cmd/version"
	"github.com/gorse-io/gorse/common/log"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
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
	maxInlineArrayValues     = 8
	maxTableLabelArrayValues = 3
)

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

func newAdminClient(cmd *cobra.Command) *AdminClient {
	endpoint, apiKey := requireEndpointAndKey(cmd)
	return NewAdminClient(endpoint, apiKey)
}

func printResultTable(cmd *cobra.Command, result any) {
	body, err := json.Marshal(result)
	if err != nil {
		log.Logger().Fatal("failed to encode API response", zap.Error(err))
	}
	printJSONTable(cmd, body)
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
		cluster, err := newAdminClient(cmd).GetCluster()
		if err != nil {
			log.Logger().Fatal("admin API request failed", zap.Error(err))
		}
		printResultTable(cmd, cluster)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get task progress from Gorse admin API",
	Run: func(cmd *cobra.Command, args []string) {
		tasks, err := newAdminClient(cmd).GetTasks()
		if err != nil {
			log.Logger().Fatal("admin API request failed", zap.Error(err))
		}
		printResultTable(cmd, tasks)
	},
}

var pipelineGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get recommendation pipeline configuration",
	Run: func(cmd *cobra.Command, args []string) {
		config, err := newAdminClient(cmd).GetConfig()
		if err != nil {
			log.Logger().Fatal("admin API request failed", zap.Error(err))
		}
		printResultTable(cmd, config)
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
		if err = newAdminClient(cmd).Dump(file); err != nil {
			log.Logger().Fatal("admin API request failed", zap.Error(err))
		}
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
		stats, err := newAdminClient(cmd).Restore(file)
		if err != nil {
			log.Logger().Fatal("admin API request failed", zap.Error(err))
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Restored %d users, %d items, %d feedback in %s.\n",
			stats.Users, stats.Items, stats.Feedback, stats.Duration)
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
		client := newAdminClient(cmd)
		configPatch, err := client.GetConfig()
		if err != nil {
			log.Logger().Fatal("admin API request failed", zap.Error(err))
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

		updatedConfig, err := client.UpdateConfig(configPatch)
		if err != nil {
			log.Logger().Fatal("admin API request failed", zap.Error(err))
		}

		printResultTable(cmd, updatedConfig)
	},
}

var pipelineResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset recommendation pipeline configuration to the file defaults",
	Run: func(cmd *cobra.Command, args []string) {
		result, err := newAdminClient(cmd).ResetConfig()
		if err != nil {
			log.Logger().Fatal("admin API request failed", zap.Error(err))
		}
		if len(result) > 0 {
			printResultTable(cmd, result)
			return
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Pipeline reset to defaults.")
	},
}

var getCategoriesCmd = &cobra.Command{
	Use:   "categories",
	Short: "Get item categories",
	Run: func(cmd *cobra.Command, args []string) {
		categories, err := newAdminClient(cmd).GetCategories()
		if err != nil {
			log.Logger().Fatal("admin API request failed", zap.Error(err))
		}
		printResultTable(cmd, categories)
	},
}

var getStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Get global statistics",
	Run: func(cmd *cobra.Command, args []string) {
		stats, err := newAdminClient(cmd).GetStats()
		if err != nil {
			log.Logger().Fatal("admin API request failed", zap.Error(err))
		}
		printResultTable(cmd, stats)
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
		timeseries, err := newAdminClient(cmd).GetTimeseries(args[0], query)
		if err != nil {
			log.Logger().Fatal("admin API request failed", zap.Error(err))
		}
		printResultTable(cmd, timeseries)
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
	value := adminFeedback(cmd, feedbackType, userID, itemID, n)
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

func adminFeedback(cmd *cobra.Command, feedbackType, userID, itemID string, n int) any {
	client := newAdminClient(cmd)
	var (
		result any
		err    error
	)
	switch {
	case userID != "" && itemID != "" && feedbackType != "":
		result, err = client.GetTypedUserItemFeedback(feedbackType, userID, itemID)
	case userID != "" && itemID != "":
		result, err = client.GetUserItemFeedback(userID, itemID)
	case userID != "" && feedbackType != "":
		result, err = client.GetTypedUserFeedback(userID, feedbackType)
	case userID != "":
		result, err = client.GetUserFeedback(userID)
	case itemID != "" && feedbackType != "":
		result, err = client.GetTypedItemFeedback(itemID, feedbackType)
	case itemID != "":
		result, err = client.GetItemFeedback(itemID)
	case feedbackType != "":
		result, err = client.GetTypedFeedback(feedbackType, n)
	default:
		result, err = client.GetFeedback(n)
	}
	if err != nil {
		log.Logger().Fatal("admin API request failed", zap.Error(err))
	}
	return tableValue(cmd, result)
}

func tableValue(cmd *cobra.Command, result any) any {
	body, err := json.Marshal(result)
	if err != nil {
		log.Logger().Fatal("failed to encode API response", zap.Error(err))
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		log.Logger().Fatal("failed to parse API response", zap.Error(err))
	}
	return value
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
		latest, err := newAdminClient(cmd).GetLatest(query)
		if err != nil {
			log.Logger().Fatal("admin API request failed", zap.Error(err))
		}
		printResultTable(cmd, latest)
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
		recommendations, err := newAdminClient(cmd).GetNonPersonalized(args[0], query)
		if err != nil {
			log.Logger().Fatal("admin API request failed", zap.Error(err))
		}
		printResultTable(cmd, recommendations)
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
		var recommender, name string
		if len(args) > 1 {
			recommender = args[1]
		}
		if len(args) > 2 {
			name = args[2]
		}
		recommendations, err := newAdminClient(cmd).GetRecommend(args[0], recommender, name, query)
		if err != nil {
			log.Logger().Fatal("admin API request failed", zap.Error(err))
		}
		printResultTable(cmd, recommendations)
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
		recommendations, err := newAdminClient(cmd).GetItemToItem(args[0], args[1], query)
		if err != nil {
			log.Logger().Fatal("admin API request failed", zap.Error(err))
		}
		printResultTable(cmd, recommendations)
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
		recommendations, err := newAdminClient(cmd).GetUserToUser(args[0], args[1], query)
		if err != nil {
			log.Logger().Fatal("admin API request failed", zap.Error(err))
		}
		printResultTable(cmd, recommendations)
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
		items, err := newAdminClient(cmd).GetExternal(query)
		if err != nil {
			log.Logger().Fatal("admin API request failed", zap.Error(err))
		}
		printResultTable(cmd, items)
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
		prompt, err := newAdminClient(cmd).GetRankerPrompt(query)
		if err != nil {
			log.Logger().Fatal("admin API request failed", zap.Error(err))
		}
		printResultTable(cmd, prompt)
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
