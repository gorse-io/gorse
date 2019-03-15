package cmd_init

import (
	"bytes"
	"testing"
)

func TestInit(t *testing.T) {
	// TODO: Need test
	cmd := CmdInit
	buf := new(bytes.Buffer)
	cmd.SetOutput(buf)
	cmd.SetArgs([]string{})
	_ = cmd.Execute()
}
