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
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/samber/lo"
	"github.com/zhenghaoz/gorse/protocol"
)

var expressionPattern = regexp.MustCompile(`^(?P<feedback_type>[a-zA-Z][a-zA-Z0-9_]*)(?P<expr_type><=|>=|<|>|=)?(?P<value>[0-9]*\.?[0-9]*)$`)

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

type FeedbackTypeExpression struct {
	FeedbackType string
	ExprType     ExprType
	Value        float64
}

func (f *FeedbackTypeExpression) String() string {
	if f.ExprType == None {
		return f.FeedbackType
	} else {
		return fmt.Sprintf("%s%v%v", f.FeedbackType, f.ExprType, f.Value)
	}
}

func (f *FeedbackTypeExpression) FromString(data string) error {
	groupNames := expressionPattern.SubexpNames()
	subMatches := expressionPattern.FindStringSubmatch(data)
	if len(subMatches) == 0 {
		return errors.New("invalid expression format, expected format: <feedback_type>[<operator><value>]")
	}
	for i, match := range subMatches {
		switch groupNames[i] {
		case "feedback_type":
			f.FeedbackType = match
		case "expr_type":
			switch match {
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
				f.Value, err = strconv.ParseFloat(match, 64)
				if err != nil {
					return fmt.Errorf("invalid value: %w", err)
				}
			}
		}
	}
	return nil
}

func (f *FeedbackTypeExpression) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.String())
}

func (f *FeedbackTypeExpression) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("unmarshal FeedbackTypeExpression: %w", err)
	}
	if err := f.FromString(s); err != nil {
		return fmt.Errorf("unmarshal FeedbackTypeExpression: %w", err)
	}
	return nil
}

func (f *FeedbackTypeExpression) ToPB() *protocol.FeedbackTypeExpression {
	pb := &protocol.FeedbackTypeExpression{}
	pb.FeedbackType = f.FeedbackType
	switch f.ExprType {
	case None:
		pb.ExpressionType = protocol.ExpressionType_None
	case Less:
		pb.ExpressionType = protocol.ExpressionType_Less
	case LessOrEqual:
		pb.ExpressionType = protocol.ExpressionType_LessOrEqual
	case Greater:
		pb.ExpressionType = protocol.ExpressionType_Greater
	case GreaterOrEqual:
		pb.ExpressionType = protocol.ExpressionType_GreaterOrEqual
	}
	pb.Value = f.Value
	return pb
}

func (f *FeedbackTypeExpression) FromPB(pb *protocol.FeedbackTypeExpression) {
	f.FeedbackType = pb.FeedbackType
	switch pb.ExpressionType {
	case protocol.ExpressionType_None:
		f.ExprType = None
	case protocol.ExpressionType_Less:
		f.ExprType = Less
	case protocol.ExpressionType_LessOrEqual:
		f.ExprType = LessOrEqual
	case protocol.ExpressionType_Greater:
		f.ExprType = Greater
	case protocol.ExpressionType_GreaterOrEqual:
		f.ExprType = GreaterOrEqual
	}
	f.Value = pb.Value
}

func (f *FeedbackTypeExpression) Match(feedbackType string, value float64) bool {
	if f.FeedbackType != feedbackType {
		return false
	}
	switch f.ExprType {
	case None:
		return true
	case Less:
		return value < f.Value
	case LessOrEqual:
		return value <= f.Value
	case Greater:
		return value > f.Value
	case GreaterOrEqual:
		return value >= f.Value
	default:
		return false
	}
}

func MatchFeedbackTypeExpressions(exprs []FeedbackTypeExpression, feedbackType string, value float64) bool {
	for _, expr := range exprs {
		if expr.Match(feedbackType, value) {
			return true
		}
	}
	return false
}

func MustParseFeedbackTypeExpression(s string) FeedbackTypeExpression {
	var expr FeedbackTypeExpression
	lo.Must0(expr.FromString(s))
	return expr
}
