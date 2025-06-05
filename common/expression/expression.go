// Copyright 2025 gorse Project Authors
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

package expression

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
)

var expressionPattern = regexp.MustCompile("^(?P<feedback_type>[a-zA-Z][a-zA-Z0-9_]*)(?P<expr_type><=|>=|<|>|=)?(?P<value>[0-9]*\\.?[0-9]*)$")

type ExprType int

const (
	None ExprType = iota
	Less
	LessOrEqual
	Greater
	GreaterOrEqual
)

func (typ ExprType) String() string {
	switch typ {
	case Less:
		return "<"
	case LessOrEqual:
		return "<="
	case Greater:
		return ">"
	case GreaterOrEqual:
		return ">="
	default:
		return ""
	}
}

func (typ ExprType) FromString(s string) ExprType {
	switch s {
	case "<":
		return Less
	case "<=":
		return LessOrEqual
	case ">":
		return Greater
	case ">=":
		return GreaterOrEqual
	default:
		return None
	}
}

type FeedbackTypeExpression struct {
	FeedbackType string
	ExprType     ExprType
	Value        float64
}

func (f *FeedbackTypeExpression) MarshalJSON() ([]byte, error) {
	if f.ExprType == None {
		return []byte(f.FeedbackType), nil
	} else {
		return []byte(fmt.Sprintf("%s%v%v", f.FeedbackType, f.ExprType, f.Value)), nil
	}
}

func (f *FeedbackTypeExpression) UnmarshalJSON(data []byte) error {
	groupNames := expressionPattern.SubexpNames()
	subMatches := expressionPattern.FindSubmatch(data)
	if len(subMatches) == 0 {
		return errors.New("invalid expression format, expected format: <feedback_type>[<operator><value>]")
	}
	for i, match := range subMatches {
		switch groupNames[i] {
		case "feedback_type":
			f.FeedbackType = string(match)
		case "expr_type":
			switch string(match) {
			case "<":
				f.ExprType = Less
			case "<=":
				f.ExprType = LessOrEqual
			case ">":
				f.ExprType = Greater
			case ">=":
				f.ExprType = GreaterOrEqual
			default:
				f.ExprType = None
			}
		case "value":
			if len(match) > 0 {
				var err error
				f.Value, err = strconv.ParseFloat(string(match), 64)
				if err != nil {
					return fmt.Errorf("invalid value: %w", err)
				}
			}
		}
	}
	return nil
}
