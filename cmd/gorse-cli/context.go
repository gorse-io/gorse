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
	"regexp"
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
	contextNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	contexts           = NewContexts(keyringService)
)

type cliContext struct {
	Name     string
	Endpoint string
	APIKey   string
}

type Contexts struct {
	keyringService string
}

func NewContexts(keyringService string) Contexts {
	return Contexts{keyringService: keyringService}
}

func (c Contexts) contextEndpointKey(name string) string {
	return "context:" + name + ":admin-endpoint"
}

func (c Contexts) contextAPIKeyKey(name string) string {
	return "context:" + name + ":admin-api-key"
}

func (c Contexts) Names() ([]string, error) {
	raw, err := keyring.Get(c.keyringService, keyringContextsKey)
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

func (c Contexts) saveNames(names []string) error {
	names = lo.Uniq(names)
	sort.Strings(names)
	raw, err := json.Marshal(names)
	if err != nil {
		return err
	}
	return keyring.Set(c.keyringService, keyringContextsKey, string(raw))
}

func (c Contexts) addName(name string) error {
	names, err := c.Names()
	if err != nil {
		return err
	}
	if !lo.Contains(names, name) {
		names = append(names, name)
	}
	return c.saveNames(names)
}

func (c Contexts) removeName(name string) ([]string, error) {
	names, err := c.Names()
	if err != nil {
		return nil, err
	}
	names = lo.Filter(names, func(candidate string, _ int) bool {
		return candidate != name
	})
	if err = c.saveNames(names); err != nil {
		return nil, err
	}
	return names, nil
}

func (c Contexts) CurrentName() (string, error) {
	name, err := keyring.Get(c.keyringService, keyringCurrentContextKey)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	return name, err
}

func (c Contexts) SetCurrentName(name string) error {
	return keyring.Set(c.keyringService, keyringCurrentContextKey, name)
}

func (c Contexts) ClearCurrentName() error {
	if err := keyring.Delete(c.keyringService, keyringCurrentContextKey); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return err
	}
	return nil
}

func (c Contexts) Load(name string) (cliContext, error) {
	if !contextNamePattern.MatchString(name) {
		return cliContext{}, fmt.Errorf("context name %q must match %q", name, contextNamePattern.String())
	}
	endpoint, endpointErr := keyring.Get(c.keyringService, c.contextEndpointKey(name))
	apiKey, apiKeyErr := keyring.Get(c.keyringService, c.contextAPIKeyKey(name))
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

func (c Contexts) Save(name, endpoint, apiKey string) error {
	if !contextNamePattern.MatchString(name) {
		return fmt.Errorf("context name %q must match %q", name, contextNamePattern.String())
	}
	if endpoint == "" {
		return fmt.Errorf("GORSE_ADMIN_ENDPOINT or --endpoint is required")
	}
	if apiKey == "" {
		return fmt.Errorf("GORSE_ADMIN_API_KEY or --api-key is required")
	}
	if err := keyring.Set(c.keyringService, c.contextEndpointKey(name), endpoint); err != nil {
		return err
	}
	if err := keyring.Set(c.keyringService, c.contextAPIKeyKey(name), apiKey); err != nil {
		return err
	}
	if err := c.addName(name); err != nil {
		return err
	}
	return c.SetCurrentName(name)
}

func (c Contexts) Delete(name string) error {
	if _, err := c.Load(name); err != nil {
		return err
	}
	if err := keyring.Delete(c.keyringService, c.contextEndpointKey(name)); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return err
	}
	if err := keyring.Delete(c.keyringService, c.contextAPIKeyKey(name)); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return err
	}
	names, err := c.removeName(name)
	if err != nil {
		return err
	}
	current, err := c.CurrentName()
	if err != nil {
		return err
	}
	if current != name {
		return nil
	}
	if len(names) == 0 {
		return c.ClearCurrentName()
	}
	return c.SetCurrentName(names[0])
}

// getEndpointAndKey returns the base URL and API key from flags, context, or environment variables.
func getEndpointAndKey(cmd *cobra.Command) (endpoint, apiKey string) {
	endpoint = getFlagOrEnv(cmd, "endpoint", "GORSE_ADMIN_ENDPOINT")
	apiKey = getFlagOrEnv(cmd, "api-key", "GORSE_ADMIN_API_KEY")

	if endpoint == "" || apiKey == "" {
		contextName := lo.Must(cmd.Flags().GetString("context"))
		if contextName != "" {
			ctx, err := contexts.Load(contextName)
			if err != nil {
				fatal(cmd,
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

	if endpoint == "" || apiKey == "" {
		contextName, err := contexts.CurrentName()
		if err != nil {
			fatal(cmd, "failed to read the current context from the system keyring: "+err.Error())
		}
		if contextName != "" {
			ctx, err := contexts.Load(contextName)
			if err != nil {
				fatal(cmd,
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

func fatal(cmd *cobra.Command, message string, suggestions ...string) {
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

func fatalErr(cmd *cobra.Command, message string, err error) {
	if err != nil {
		message += ": " + err.Error()
	}
	fatal(cmd, message)
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
				fatal(cmd, "failed to read API key: "+err.Error())
			}
		}
		if err := contexts.Save(name, endpoint, apiKey); err != nil {
			fatal(cmd,
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
		names, err := contexts.Names()
		if err != nil {
			fatal(cmd, "failed to read contexts from the system keyring: "+err.Error())
		}
		if len(names) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No contexts configured.")
			return
		}
		current, err := contexts.CurrentName()
		if err != nil {
			fatal(cmd, "failed to read the current context from the system keyring: "+err.Error())
		}
		rows := make([][]string, 0, len(names))
		for _, name := range names {
			ctx, err := contexts.Load(name)
			if err != nil {
				fatal(cmd, "context "+strconv.Quote(name)+" is invalid or incomplete: "+err.Error())
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
		if _, err := contexts.Load(name); err != nil {
			fatal(cmd,
				"context "+strconv.Quote(name)+" was not found.",
				"List available contexts:",
				"  gorse-cli context list",
			)
		}
		if err := contexts.SetCurrentName(name); err != nil {
			fatal(cmd, "failed to save the current context in the system keyring: "+err.Error())
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
		if err := contexts.Delete(name); err != nil {
			fatal(cmd,
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
		name, err := contexts.CurrentName()
		if err != nil {
			fatal(cmd, "failed to read the current context from the system keyring: "+err.Error())
		}
		if name == "" {
			fmt.Fprintln(cmd.OutOrStdout(), "No current context.")
			return
		}
		ctx, err := contexts.Load(name)
		if err != nil {
			fatal(cmd,
				"current context "+strconv.Quote(name)+" is invalid or incomplete.",
				"Choose another context:",
				"  gorse-cli context use <name>",
				"Or recreate it:",
				"  gorse-cli context add "+name+" --endpoint http://localhost:8088",
			)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Name:\t\t%s\nEndpoint:\t%s\n", ctx.Name, ctx.Endpoint)
	},
}
