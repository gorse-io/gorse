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
	"os"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
	"golang.org/x/term"
)

const (
	keyringService           = "gorse-cli"
	keyringCurrentContextKey = "current-context"
	keyringContextsKey       = "contexts"
)

var (
	keyringGet    = keyring.Get
	keyringSet    = keyring.Set
	keyringDelete = keyring.Delete
)

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
	endpoint = lo.Must(cmd.Flags().GetString("endpoint"))
	apiKey = lo.Must(cmd.Flags().GetString("api-key"))

	if endpoint == "" || apiKey == "" {
		contextName := lo.Must(cmd.Flags().GetString("context"))
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
	value := lo.Must(cmd.Flags().GetString(flagName))
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
