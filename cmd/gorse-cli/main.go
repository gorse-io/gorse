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
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gorse-io/gorse/cmd/version"
	"github.com/gorse-io/gorse/common/log"
	"github.com/olekukonko/tablewriter"
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
			fmt.Println(version.BuildInfo())
			return
		}
		_ = cmd.Help()
	},
}

// Admin API configuration from environment variables
var (
	adminAPIKey   = os.Getenv("GORSE_ADMIN_API_KEY")
	adminEndpoint = os.Getenv("GORSE_ADMIN_ENDPOINT")
)

// newAdminClient creates a new resty client for admin API
func newAdminClient(endpoint, apiKey string) *resty.Client {
	client := resty.New()
	client.SetBaseURL(endpoint)
	client.SetHeader("X-Api-Key", apiKey)
	return client
}

// getEndpointAndKey returns the endpoint and API key from flags or environment
func getEndpointAndKey(cmd *cobra.Command) (endpoint, apiKey string) {
	endpoint, _ = cmd.Flags().GetString("endpoint")
	apiKey, _ = cmd.Flags().GetString("api-key")
	if endpoint == "" {
		endpoint = adminEndpoint
	}
	if apiKey == "" {
		apiKey = adminAPIKey
	}
	return
}

// ClusterNode represents a node in the cluster
type ClusterNode struct {
	UUID       string    `json:"uuid"`
	Hostname   string    `json:"hostname"`
	Type       string    `json:"type"`
	Version    string    `json:"version"`
	UpdateTime time.Time `json:"update_time"`
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
		endpoint, apiKey := getEndpointAndKey(cmd)
		if endpoint == "" {
			log.Logger().Fatal("GORSE_ADMIN_ENDPOINT or --endpoint is required")
		}

		client := newAdminClient(endpoint, apiKey)
		resp, err := client.R().Get("/dashboard/cluster")
		if err != nil {
			log.Logger().Fatal("failed to send request", zap.Error(err))
		}

		if resp.IsError() {
			log.Logger().Fatal("API request failed",
				zap.Int("status", resp.StatusCode()),
				zap.String("body", resp.String()))
		}

		// Parse and format output
		var nodes []ClusterNode
		if err := json.Unmarshal(resp.Body(), &nodes); err != nil {
			log.Logger().Fatal("failed to parse response", zap.Error(err))
		}

		// Format as table
		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"Type", "UUID", "Hostname", "Version", "Update Time"})
		data := make([][]string, len(nodes))
		for i, node := range nodes {
			data[i] = []string{
				node.Type,
				node.UUID,
				node.Hostname,
				node.Version,
				node.UpdateTime.Format("2006-01-02 15:04:05"),
			}
		}
		lo.Must0(table.Bulk(data))
		lo.Must0(table.Render())
	},
}

// getTasksCmd gets tasks from the admin API
var getTasksCmd = &cobra.Command{
	Use:   "tasks",
	Short: "Get task progress from Gorse admin API",
	Run: func(cmd *cobra.Command, args []string) {
		endpoint, apiKey := getEndpointAndKey(cmd)
		if endpoint == "" {
			log.Logger().Fatal("GORSE_ADMIN_ENDPOINT or --endpoint is required")
		}

		client := newAdminClient(endpoint, apiKey)
		resp, err := client.R().Get("/dashboard/tasks")
		if err != nil {
			log.Logger().Fatal("failed to send request", zap.Error(err))
		}

		if resp.IsError() {
			log.Logger().Fatal("API request failed",
				zap.Int("status", resp.StatusCode()),
				zap.String("body", resp.String()))
		}

		fmt.Println(resp.String())
	},
}

// getConfigCmd gets configuration from the admin API
var getConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Get configuration from Gorse admin API",
	Run: func(cmd *cobra.Command, args []string) {
		endpoint, apiKey := getEndpointAndKey(cmd)
		if endpoint == "" {
			log.Logger().Fatal("GORSE_ADMIN_ENDPOINT or --endpoint is required")
		}

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

		fmt.Println(resp.String())
	},
}

// setCmd is the parent command for set operations
var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Set resources in Gorse admin API",
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
		endpoint, apiKey := getEndpointAndKey(cmd)
		if endpoint == "" {
			log.Logger().Fatal("GORSE_ADMIN_ENDPOINT or --endpoint is required")
		}

		// Build config patch from arguments
		configPatch := make(map[string]interface{})
		for _, arg := range args {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) != 2 {
				log.Logger().Fatal("invalid config format, expected key=value", zap.String("arg", arg))
			}
			key := parts[0]
			value := parts[1]

			// Parse the value
			configPatch[key] = parseConfigValue(value)
		}

		client := newAdminClient(endpoint, apiKey)
		resp, err := client.R().
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

		fmt.Println(resp.String())
	},
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
	rootCmd.AddCommand(getCmd)
	getCmd.AddCommand(getClusterCmd)
	getCmd.AddCommand(getTasksCmd)
	getCmd.AddCommand(getConfigCmd)

	rootCmd.AddCommand(setCmd)
	setCmd.AddCommand(setConfigCmd)

	// Add flags for endpoint and api-key
	getClusterCmd.Flags().String("endpoint", "", "Gorse admin API endpoint (default: GORSE_ADMIN_ENDPOINT)")
	getClusterCmd.Flags().String("api-key", "", "Gorse admin API key (default: GORSE_ADMIN_API_KEY)")
	getTasksCmd.Flags().String("endpoint", "", "Gorse admin API endpoint (default: GORSE_ADMIN_ENDPOINT)")
	getTasksCmd.Flags().String("api-key", "", "Gorse admin API key (default: GORSE_ADMIN_API_KEY)")
	getConfigCmd.Flags().String("endpoint", "", "Gorse admin API endpoint (default: GORSE_ADMIN_ENDPOINT)")
	getConfigCmd.Flags().String("api-key", "", "Gorse admin API key (default: GORSE_ADMIN_API_KEY)")
	setConfigCmd.Flags().String("endpoint", "", "Gorse admin API endpoint (default: GORSE_ADMIN_ENDPOINT)")
	setConfigCmd.Flags().String("api-key", "", "Gorse admin API key (default: GORSE_ADMIN_API_KEY)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Logger().Fatal("failed to execute command", zap.Error(err))
	}
}
