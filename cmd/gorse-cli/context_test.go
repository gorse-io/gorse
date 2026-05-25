package main

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

func TestContextAddCmd(t *testing.T) {
	dev := testContextName(t, "dev")
	withIsolatedRealKeyring(t, dev)

	out, err := executeRawCommand(rootCmd, "context", "add", dev, "--endpoint", "http://localhost:8088", "--api-key", "secret")
	require.NoError(t, err)
	require.Contains(t, out, fmt.Sprintf("Context %q saved", dev))
	ctx, err := loadContext(dev)
	require.NoError(t, err)
	require.Equal(t, "http://localhost:8088", ctx.Endpoint)
	require.Equal(t, "secret", ctx.APIKey)
	current, err := getCurrentContextName()
	require.NoError(t, err)
	require.Equal(t, dev, current)

	names, err := getContextNames()
	require.NoError(t, err)
	require.Equal(t, []string{dev}, names)

	out, err = executeRawCommand(rootCmd, "context", "add", dev, "--endpoint", "http://localhost:9090", "--api-key", "updated")
	require.NoError(t, err)
	require.Contains(t, out, fmt.Sprintf("Context %q saved", dev))
	ctx, err = loadContext(dev)
	require.NoError(t, err)
	require.Equal(t, "http://localhost:9090", ctx.Endpoint)
	require.Equal(t, "updated", ctx.APIKey)

	names, err = getContextNames()
	require.NoError(t, err)
	require.Equal(t, []string{dev}, names)
}

func TestContextCommands(t *testing.T) {
	prod := testContextName(t, "prod")
	dev := testContextName(t, "dev")
	withIsolatedRealKeyring(t, prod, dev)

	_, err := executeRawCommand(rootCmd, "context", "add", prod, "--endpoint", "http://prod:8088", "--api-key", "prod-secret")
	require.NoError(t, err)
	_, err = executeRawCommand(rootCmd, "context", "add", dev, "--endpoint", "http://dev:8088", "--api-key", "dev-secret")
	require.NoError(t, err)

	listOut, err := executeRawCommand(rootCmd, "context", "list")
	require.NoError(t, err)
	require.Contains(t, listOut, dev)
	require.Contains(t, listOut, prod)
	require.Contains(t, listOut, "http://dev:8088")
	require.NotContains(t, listOut, "dev-secret")

	currentOut, err := executeRawCommand(rootCmd, "context", "current")
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("Name:\t\t%s\nEndpoint:\thttp://dev:8088\n", dev), currentOut)
	require.NotContains(t, currentOut, "dev-secret")

	useOut, err := executeRawCommand(rootCmd, "context", "use", prod)
	require.NoError(t, err)
	require.Contains(t, useOut, fmt.Sprintf("Switched to context %q", prod))
	current, err := getCurrentContextName()
	require.NoError(t, err)
	require.Equal(t, prod, current)

	deleteOut, err := executeRawCommand(rootCmd, "context", "delete", prod)
	require.NoError(t, err)
	require.Contains(t, deleteOut, fmt.Sprintf("Context %q deleted", prod))
	_, err = loadContext(prod)
	require.Error(t, err)
	current, err = getCurrentContextName()
	require.NoError(t, err)
	require.Equal(t, dev, current)

	deleteOut, err = executeRawCommand(rootCmd, "context", "delete", dev)
	require.NoError(t, err)
	require.Contains(t, deleteOut, fmt.Sprintf("Context %q deleted", dev))

	currentOut, err = executeRawCommand(rootCmd, "context", "current")
	require.NoError(t, err)
	require.Contains(t, currentOut, "No current context.")
}

type savedKeyringValue struct {
	value  string
	exists bool
}

func testContextName(t *testing.T, suffix string) string {
	t.Helper()
	return fmt.Sprintf("gorse-cli-test-%s-%d", suffix, time.Now().UnixNano())
}

func withIsolatedRealKeyring(t *testing.T, contextNames ...string) {
	t.Helper()

	savedContexts := readKeyringValue(t, keyringContextsKey)
	savedCurrent := readKeyringValue(t, keyringCurrentContextKey)
	t.Cleanup(func() {
		for _, name := range contextNames {
			deleteKeyringValue(t, contextEndpointKey(name))
			deleteKeyringValue(t, contextAPIKeyKey(name))
		}
		restoreKeyringValue(t, keyringContextsKey, savedContexts)
		restoreKeyringValue(t, keyringCurrentContextKey, savedCurrent)
	})

	deleteKeyringValue(t, keyringContextsKey)
	deleteKeyringValue(t, keyringCurrentContextKey)
	for _, name := range contextNames {
		deleteKeyringValue(t, contextEndpointKey(name))
		deleteKeyringValue(t, contextAPIKeyKey(name))
	}
}

func readKeyringValue(t *testing.T, key string) savedKeyringValue {
	t.Helper()
	value, err := keyring.Get(keyringService, key)
	if errors.Is(err, keyring.ErrNotFound) {
		return savedKeyringValue{}
	}
	require.NoError(t, err)
	return savedKeyringValue{value: value, exists: true}
}

func restoreKeyringValue(t *testing.T, key string, saved savedKeyringValue) {
	t.Helper()
	if !saved.exists {
		deleteKeyringValue(t, key)
		return
	}
	require.NoError(t, keyring.Set(keyringService, key, saved.value))
}

func deleteKeyringValue(t *testing.T, key string) {
	t.Helper()
	err := keyring.Delete(keyringService, key)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		require.NoError(t, err)
	}
}
