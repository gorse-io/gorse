package cmd_cv

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"log"
	"regexp"
	"strconv"
	"testing"
)

const (
	ratingEpsilon = 0.005
)

func TestCmdTest(t *testing.T) {
	// Test rating
	cmd := CmdTest
	buf := bytes.NewBuffer(nil)
	log.SetOutput(buf)
	cmd.SetArgs([]string{
		"svd",
		"--load-builtin", "ml-100k",
		"--eval-rmse",
		"--eval-mae",
		"--set-n-epochs", "100",
		"--set-reg", "0.1",
		"--set-lr", "0.01",
		"--set-n-factors", "50",
		"--set-init-mean", "0",
		"--set-init-std", "0.001",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	scores, err := ParseOutput(buf.String())
	if err != nil {
		t.Fatal(err)
	}
	// Test RMSE
	assert.True(t, (scores[0]-0.90728) < ratingEpsilon, "low accuracy in RMSE")
	// Test MAE
	assert.True(t, (scores[1]-0.71508) < ratingEpsilon, "low accuracy in MAE")
}

func ParseOutput(output string) ([]float64, error) {
	r, err := regexp.Compile("\\d+\\.\\d+\\(")
	if err != nil {
		return nil, err
	}
	match := r.FindAllString(output, -1)
	scores := make([]float64, len(match))
	for i, str := range match {
		if scores[i], err = strconv.ParseFloat(str[:len(str)-1], 10); err != nil {
			return nil, err
		}
	}
	return scores, nil
}
