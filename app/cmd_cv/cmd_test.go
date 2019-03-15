package cmd_cv

import (
	"bytes"
	"testing"
)

func TestCV(t *testing.T) {
	// TODO: Need test
	cmd := CmdTest
	buf := new(bytes.Buffer)
	cmd.SetOutput(buf)
	cmd.SetArgs([]string{})
	_ = cmd.Execute()
}
