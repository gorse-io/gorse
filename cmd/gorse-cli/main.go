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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	gorse "github.com/gorse-io/gorse-go"
	"github.com/gorse-io/gorse/cmd/version"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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

func requireEndpointAndKey(cmd *cobra.Command) (string, string) {
	endpoint, apiKey := getEndpointAndKey(cmd)
	if endpoint == "" || apiKey == "" {
		switch {
		case endpoint == "" && apiKey == "":
			fatal(cmd,
				"no Gorse context is selected and no endpoint/API key was provided.",
				"Create a context:",
				"  gorse-cli context add dev --endpoint http://localhost:8088",
				"Or use environment variables:",
				"  GORSE_ADMIN_ENDPOINT=http://localhost:8088 GORSE_ADMIN_API_KEY=<api-key> "+cmd.CommandPath(),
				"Or pass credentials for this command:",
				"  "+cmd.CommandPath()+" --endpoint http://localhost:8088 --api-key <api-key>",
			)
		case endpoint == "":
			fatal(cmd,
				"missing Gorse base URL.",
				"Use a saved context:",
				"  gorse-cli context use <name>",
				"Or set an environment variable:",
				"  GORSE_ADMIN_ENDPOINT=http://localhost:8088 "+cmd.CommandPath(),
				"Or pass an endpoint:",
				"  "+cmd.CommandPath()+" --endpoint http://localhost:8088",
			)
		case apiKey == "":
			fatal(cmd,
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

// getCmd is the parent command for get operations
var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get items, users and feedback",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

var recommendCmd = &cobra.Command{
	Use:   "recommend",
	Short: "Get recommendations",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

var pipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Config recommendation pipeline",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

// getClusterCmd gets cluster nodes
var getClusterCmd = &cobra.Command{
	Use:   "cluster-info",
	Short: "List cluster nodes",
	Run: func(cmd *cobra.Command, args []string) {
		cluster, err := newAdminClient(cmd).GetCluster()
		if err != nil {
			fatalErr(cmd, "admin API request failed", err)
		}
		printArrayTable(cmd, cluster)
	},
}

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "List task progress",
	Run: func(cmd *cobra.Command, args []string) {
		tasks, err := newAdminClient(cmd).GetTasks()
		if err != nil {
			fatalErr(cmd, "admin API request failed", err)
		}
		rows := make([][]string, len(tasks))
		for i, task := range tasks {
			rows[i] = []string{
				formatTableValue(task.Tracer),
				formatTableValue(task.Name),
				formatTableValue(task.Status),
				formatTableValue(formatProgressBar(task.Count, task.Total)),
				formatTableValue(task.Error),
				formatTableValue(task.StartTime),
				formatTableValue(task.FinishTime),
			}
		}
		printTable(cmd.OutOrStdout(), []string{"Tracer", "Name", "Status", "Progress", "Error", "StartTime", "FinishTime"}, rows)
	},
}

var pipelineGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get recommendation pipeline configuration",
	Run: func(cmd *cobra.Command, args []string) {
		configValue, err := newAdminClient(cmd).GetConfig()
		if err != nil {
			fatalErr(cmd, "admin API request failed", err)
		}
		configMap := formatConfigMap(configValue)
		_, recommend, ok := getConfigValue(configMap, "recommend", "Recommend")
		if !ok {
			fatal(cmd, "recommend config not found")
		}
		encoder := yaml.NewEncoder(cmd.OutOrStdout())
		defer encoder.Close()
		if err = encoder.Encode(recommend); err != nil {
			fatalErr(cmd, "failed to encode config", err)
		}
	},
}

var pipelineSchemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Get recommendation pipeline configuration schema",
	Run: func(cmd *cobra.Command, args []string) {
		schema, err := newAdminClient(cmd).GetConfigSchema()
		if err != nil {
			fatalErr(cmd, "admin API request failed", err)
		}
		encoder := yaml.NewEncoder(cmd.OutOrStdout())
		defer encoder.Close()
		if err = encoder.Encode(schema); err != nil {
			fatalErr(cmd, "failed to encode schema", err)
		}
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
			fatalErr(cmd, fmt.Sprintf("failed to create dump file %q", outputPath), err)
		}
		defer file.Close()
		if err = newAdminClient(cmd).Dump(file); err != nil {
			fatalErr(cmd, "admin API request failed", err)
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
		fmt.Fprintf(cmd.OutOrStdout(), "Restore data from %s? Existing users, items, feedback, and cache will be overwritten. Confirm [y/N]: ", args[0])
		input, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			fatalErr(cmd, "failed to read confirmation", err)
		}
		if !strings.EqualFold(strings.TrimSpace(input), "y") {
			fmt.Fprintln(cmd.OutOrStdout(), "Restore canceled")
			return
		}
		file, err := os.Open(args[0])
		if err != nil {
			fatalErr(cmd, fmt.Sprintf("failed to open restore file %q", args[0]), err)
		}
		defer file.Close()
		stats, err := newAdminClient(cmd).Restore(file)
		if err != nil {
			fatalErr(cmd, "admin API request failed", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Restored %d users, %d items, %d feedback in %s.\n",
			stats.Users, stats.Items, stats.Feedback, stats.Duration)
	},
}

var pipelinePatchCmd = &cobra.Command{
	Use:   "patch <json-patch>",
	Short: "Patch recommendation pipeline configuration values",
	Example: `  # Replace a single config value
  gorse-cli pipeline patch '[{"op":"replace","path":"/cache_size","value":1000}]'

  # Replace multiple config values
  gorse-cli pipeline patch '[{"op":"replace","path":"/cache_size","value":1000},{"op":"replace","path":"/data_source/item_ttl","value":72}]'`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := newAdminClient(cmd)
		currentConfigMap, err := client.GetConfigMap()
		if err != nil {
			fatalErr(cmd, "admin API request failed", err)
		}
		recommendKey, recommendConfig, ok := getConfigValue(currentConfigMap, "recommend", "Recommend")
		if !ok {
			fatal(cmd, "recommend config not found")
		}

		configBytes, err := json.Marshal(recommendConfig)
		if err != nil {
			fatalErr(cmd, "failed to apply JSON patch: failed to encode config", err)
		}
		patch, err := jsonpatch.DecodePatch([]byte(args[0]))
		if err != nil {
			fatalErr(cmd, "failed to apply JSON patch: failed to decode JSON patch", err)
		}
		patchedConfigBytes, err := patch.Apply(configBytes)
		if err != nil {
			fatalErr(cmd, "failed to apply JSON patch", err)
		}
		var configPatch map[string]any
		if err = json.Unmarshal(patchedConfigBytes, &configPatch); err != nil {
			fatalErr(cmd, "failed to apply JSON patch: failed to decode patched config", err)
		}
		currentConfigMap[recommendKey] = configPatch

		updatedConfig, err := client.UpdateConfig(currentConfigMap)
		if err != nil {
			fatalErr(cmd, "admin API request failed", err)
		}

		printStruct(cmd, updatedConfig)
	},
}

var pipelineResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset recommendation pipeline configuration to the file defaults",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprint(cmd.OutOrStdout(), "Reset recommendation pipeline configuration to file defaults? Current pipeline settings will be overwritten. Confirm [y/N]: ")
		input, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			fatalErr(cmd, "failed to read confirmation", err)
		}
		if !strings.EqualFold(strings.TrimSpace(input), "y") {
			fmt.Fprintln(cmd.OutOrStdout(), "Pipeline reset canceled")
			return
		}
		result, err := newAdminClient(cmd).ResetConfig()
		if err != nil {
			fatalErr(cmd, "admin API request failed", err)
		}
		if len(result) > 0 {
			printStruct(cmd, result)
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
			fatalErr(cmd, "admin API request failed", err)
		}
		printArrayTable(cmd, categories)
	},
}

var getStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Get global statistics",
	Run: func(cmd *cobra.Command, args []string) {
		stats, err := newAdminClient(cmd).GetStats()
		if err != nil {
			fatalErr(cmd, "admin API request failed", err)
		}
		printStruct(cmd, stats)
	},
}

var getUserCmd = &cobra.Command{
	Use:   "user <user-id>",
	Short: "Get user details",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		user, err := newGorseClient(cmd).GetUser(cmd.Context(), args[0])
		if err != nil {
			fatalErr(cmd, "API request failed", err)
		}
		printStruct(cmd, user)
	},
}

var getItemCmd = &cobra.Command{
	Use:   "item <item-id>",
	Short: "Get item details",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		item, err := newGorseClient(cmd).GetItem(cmd.Context(), args[0])
		if err != nil {
			fatalErr(cmd, "API request failed", err)
		}
		printStruct(cmd, item)
	},
}

var getUsersCmd = &cobra.Command{
	Use:   "users",
	Short: "Get users",
	Run: func(cmd *cobra.Command, args []string) {
		n := lo.Must(cmd.Flags().GetInt("n"))
		if n == 0 {
			n = 10
		}
		users, err := newGorseClient(cmd).GetUsers(cmd.Context(), n, "")
		if err != nil {
			fatalErr(cmd, "API request failed", err)
		}
		printArrayTable(cmd, users.Users)
	},
}

var getItemsCmd = &cobra.Command{
	Use:   "items [query...]",
	Short: "Get or search items",
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		n := lo.Must(cmd.Flags().GetInt("n"))
		if n == 0 {
			n = 10
		}
		client := newGorseClient(cmd)
		var (
			items gorse.ItemIterator
			err   error
		)
		if len(args) > 0 {
			items, err = client.SearchItems(cmd.Context(), strings.Join(args, " "), n)
		} else {
			items, err = client.GetItems(cmd.Context(), n, "")
		}
		if err != nil {
			fatalErr(cmd, "API request failed", err)
		}
		printArrayTable(cmd, items.Items)
	},
}

var getFeedbackCmd = &cobra.Command{
	Use:   "feedback",
	Short: "Get feedback",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		n := lo.Must(cmd.Flags().GetInt("n"))
		if n == 0 {
			n = 10
		}
		feedbackType := lo.Must(cmd.Flags().GetString("type"))
		userID := lo.Must(cmd.Flags().GetString("user"))
		itemID := lo.Must(cmd.Flags().GetString("item"))
		client := newAdminClient(cmd)
		var (
			feedback []Feedback
			err      error
		)
		switch {
		case userID != "" && itemID != "" && feedbackType != "":
			record, requestErr := client.GetTypedUserItemFeedback(feedbackType, userID, itemID)
			err = requestErr
			feedback = []Feedback{record}
		case userID != "" && itemID != "":
			feedback, err = client.GetUserItemFeedback(userID, itemID)
		case userID != "" && feedbackType != "":
			feedback, err = client.GetTypedUserFeedback(userID, feedbackType)
		case userID != "":
			feedback, err = client.GetUserFeedback(userID)
		case itemID != "" && feedbackType != "":
			feedback, err = client.GetTypedItemFeedback(itemID, feedbackType)
		case itemID != "":
			feedback, err = client.GetItemFeedback(itemID)
		case feedbackType != "":
			iterator, requestErr := client.GetTypedFeedback(feedbackType, n)
			err = requestErr
			feedback = iterator.Feedback
		default:
			iterator, requestErr := client.GetFeedback(n)
			err = requestErr
			feedback = iterator.Feedback
		}
		if err != nil {
			fatalErr(cmd, "admin API request failed", err)
		}
		sortFeedback(feedback)
		if n > 0 && n < len(feedback) {
			feedback = feedback[:n]
		}
		printArrayTable(cmd, feedback)
	},
}

var getLatestCmd = &cobra.Command{
	Use:   "latest",
	Short: "Get latest items",
	Run: func(cmd *cobra.Command, args []string) {
		categories := lo.Must(cmd.Flags().GetStringArray("category"))
		n := lo.Must(cmd.Flags().GetInt("n"))
		if n == 0 {
			n = 10
		}
		latest, err := newAdminClient(cmd).GetLatest(n, categories)
		if err != nil {
			fatalErr(cmd, "admin API request failed", err)
		}
		printArrayTable(cmd, latest)
	},
}

var getNonPersonalizedCmd = &cobra.Command{
	Use:   "non-personalized <name>",
	Short: "Get non-personalized recommendations",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		recommendations, err := newAdminClient(cmd).GetNonPersonalized(
			args[0],
			lo.Must(cmd.Flags().GetInt("n")),
			lo.Must(cmd.Flags().GetString("user-id")),
			lo.Must(cmd.Flags().GetStringArray("category")),
		)
		if err != nil {
			fatalErr(cmd, "admin API request failed", err)
		}
		printArrayTable(cmd, recommendations)
	},
}

var recommendUserCmd = &cobra.Command{
	Use:   "item-to-user <user-id> [recommender] [name]",
	Short: "Get recommendations for a user",
	Args:  cobra.RangeArgs(1, 3),
	Run: func(cmd *cobra.Command, args []string) {
		categories := lo.Must(cmd.Flags().GetStringArray("category"))
		n := lo.Must(cmd.Flags().GetInt("n"))
		if n == 0 {
			n = 10
		}
		var recommender, name string
		if len(args) > 1 {
			recommender = args[1]
		}
		if len(args) > 2 {
			name = args[2]
		}
		recommendations, err := newAdminClient(cmd).GetRecommend(args[0], recommender, name, n, categories)
		if err != nil {
			fatalErr(cmd, "admin API request failed", err)
		}
		printArrayTable(cmd, recommendations)
	},
}

var getItemToItemCmd = &cobra.Command{
	Use:   "item-to-item <name> <item-id>",
	Short: "Get item-to-item recommendations",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		recommendations, err := newAdminClient(cmd).GetItemToItem(
			args[0],
			args[1],
			lo.Must(cmd.Flags().GetInt("n")),
			lo.Must(cmd.Flags().GetStringArray("category")),
		)
		if err != nil {
			fatalErr(cmd, "admin API request failed", err)
		}
		printArrayTable(cmd, recommendations)
	},
}

var getUserToUserCmd = &cobra.Command{
	Use:   "user-to-user <name> <user-id>",
	Short: "Get user-to-user recommendations",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		recommendations, err := newAdminClient(cmd).GetUserToUser(args[0], args[1], lo.Must(cmd.Flags().GetInt("n")))
		if err != nil {
			fatalErr(cmd, "admin API request failed", err)
		}
		printArrayTable(cmd, recommendations)
	},
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
	rootCmd.AddCommand(getClusterCmd)
	rootCmd.AddCommand(getStatsCmd)
	getCmd.AddCommand(getCategoriesCmd)
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

	rootCmd.AddCommand(pipelineCmd)
	pipelineCmd.AddCommand(pipelineGetCmd)
	pipelineCmd.AddCommand(pipelineSchemaCmd)
	pipelineCmd.AddCommand(pipelinePatchCmd)
	pipelineCmd.AddCommand(pipelineResetCmd)
	rootCmd.AddCommand(psCmd)
	rootCmd.AddCommand(dumpCmd)
	rootCmd.AddCommand(restoreCmd)

	contextAddCmd.Flags().String("endpoint", "", "Gorse base URL (default: GORSE_ADMIN_ENDPOINT)")
	contextAddCmd.Flags().String("api-key", "", "Gorse admin API key (default: GORSE_ADMIN_API_KEY; prefer prompt to avoid shell history)")
	for _, cmd := range []*cobra.Command{
		getClusterCmd, getCategoriesCmd, psCmd,
		getStatsCmd, getUserCmd, getUsersCmd, getItemCmd, getItemsCmd, getFeedbackCmd, getLatestCmd,
		getNonPersonalizedCmd, recommendUserCmd, getItemToItemCmd, getUserToUserCmd,
		pipelineGetCmd, pipelineSchemaCmd, pipelinePatchCmd, pipelineResetCmd, dumpCmd, restoreCmd,
	} {
		cmd.Flags().String("endpoint", "", "Gorse base URL (default: selected context or GORSE_ADMIN_ENDPOINT)")
		cmd.Flags().String("api-key", "", "Gorse admin API key (default: selected context or GORSE_ADMIN_API_KEY)")
		cmd.Flags().String("context", "", "Gorse CLI context name")
	}

	getUsersCmd.Flags().IntP("n", "n", 0, "Number of returned users")
	getItemsCmd.Flags().IntP("n", "n", 0, "Number of returned items")
	getFeedbackCmd.Flags().IntP("n", "n", 0, "Number of returned feedback")
	getFeedbackCmd.Flags().String("type", "", "Filter by feedback type")
	getFeedbackCmd.Flags().String("user", "", "Filter by user ID")
	getFeedbackCmd.Flags().String("item", "", "Filter by item ID")
	getLatestCmd.Flags().IntP("n", "n", 0, "Number of returned records")
	getLatestCmd.Flags().StringArray("category", nil, "Filter by category, repeatable")
	getNonPersonalizedCmd.Flags().IntP("n", "n", 0, "Number of returned records")
	getNonPersonalizedCmd.Flags().StringArray("category", nil, "Filter by category, repeatable")
	getNonPersonalizedCmd.Flags().String("user-id", "", "Remove read items of a user")
	recommendUserCmd.Flags().IntP("n", "n", 0, "Number of returned items")
	recommendUserCmd.Flags().StringArray("category", nil, "Filter by category, repeatable")
	getItemToItemCmd.Flags().IntP("n", "n", 0, "Number of returned records")
	getItemToItemCmd.Flags().StringArray("category", nil, "Filter by category, repeatable")
	getUserToUserCmd.Flags().IntP("n", "n", 0, "Number of returned records")

}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
