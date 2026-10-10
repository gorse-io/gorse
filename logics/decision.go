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

package logics

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/gorse-io/gorse/common/decision"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/nikolalohinski/gonja/v2"
	"github.com/nikolalohinski/gonja/v2/exec"
)

// DecisionReranker scores candidates against a user query using the decisions API.
type DecisionReranker struct {
	queryTemplate *exec.Template
	docTemplate   *exec.Template
	client        *decision.Client
}

func NewDecisionReranker(cfg config.RerankerAPIConfig, queryTemplate, docTemplate string) (*DecisionReranker, error) {
	qTpl, err := gonja.FromString(queryTemplate)
	if err != nil {
		return nil, err
	}
	dTpl, err := gonja.FromString(docTemplate)
	if err != nil {
		return nil, err
	}
	options := []decision.Option{}
	if cfg.URL != "" {
		options = append(options, decision.WithEndpoint(cfg.URL))
	}
	if cfg.Model != "" {
		options = append(options, decision.WithModel(cfg.Model))
	}
	return &DecisionReranker{queryTemplate: qTpl, docTemplate: dTpl, client: decision.NewClient(cfg.AuthToken, options...)}, nil
}

func (r *DecisionReranker) Rank(ctx context.Context, user *data.User, feedback []*FeedbackItem, items []*data.Item) ([]cache.Score, error) {
	if len(items) == 0 {
		return nil, nil
	}
	var query strings.Builder
	if err := r.queryTemplate.Execute(&query, exec.NewContext(map[string]any{"user": user, "feedback": feedback})); err != nil {
		return nil, err
	}
	criteria := []any{"Not relevant", "Slightly relevant", "Moderately relevant", "Highly relevant", "Extremely relevant"}
	questions := make(decision.Questions, len(items))
	for _, item := range items {
		var document strings.Builder
		if err := r.docTemplate.Execute(&document, exec.NewContext(map[string]any{"item": item})); err != nil {
			return nil, err
		}
		questions[item.ItemId] = decision.ScoreQuestion{
			Instructions: "Rate the relevance of the following candidate to the user query and preferences in the state:\n" + document.String(),
			Criteria:     criteria,
		}
	}
	response, err := r.client.Evaluate(ctx, query.String(), questions)
	if err != nil {
		return nil, err
	}
	scores := make([]cache.Score, 0, len(items))
	for _, item := range items {
		answer, exists := response.Answers[item.ItemId]
		if !exists || answer.Type != "score" || answer.Score == nil {
			return nil, fmt.Errorf("decision response is missing a score for item %q", item.ItemId)
		}
		score := *answer.Score
		if math.IsNaN(score) || math.IsInf(score, 0) || score < 0 || score > float64(len(criteria)-1) {
			return nil, fmt.Errorf("decision response has an invalid score for item %q: %g", item.ItemId, score)
		}
		scores = append(scores, cache.Score{Id: item.ItemId, Score: score})
	}
	cache.SortDocuments(scores)
	return scores, nil
}
