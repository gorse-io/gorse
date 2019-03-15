package cmd_data

import (
	"bytes"
	"testing"
)

func TestData(t *testing.T) {
	// TODO: Need test
	cmd := CmdData
	buf := new(bytes.Buffer)
	cmd.SetOutput(buf)
	cmd.SetArgs([]string{})
	_ = cmd.Execute()
}
