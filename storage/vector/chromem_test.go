package vector

import (
	"testing"

	"github.com/philippgille/chromem-go"
	"github.com/stretchr/testify/suite"
)

type ChrommemTestSuite struct {
	baseTestSuite
}

func (suite *ChrommemTestSuite) SetupSuite() {
	c := &Chromem{}
	c.db = chromem.NewDB()
	suite.Database = c
}

func TestChrommem(t *testing.T) {
	suite.Run(t, new(ChrommemTestSuite))
}
