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

package logics

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"

	"github.com/gorse-io/gorse/common/decision"
	"github.com/gorse-io/gorse/common/log"
	"github.com/gorse-io/gorse/common/reranker"
	"github.com/gorse-io/gorse/config"
	"github.com/gorse-io/gorse/storage/cache"
	"github.com/gorse-io/gorse/storage/data"
	"github.com/nikolalohinski/gonja/v2"
	"github.com/nikolalohinski/gonja/v2/exec"
	"go.uber.org/zap"
)

type FeedbackItem struct {
	FeedbackType string
	data.Item
}

type rankerTemplates struct {
	queryTemplate *exec.Template
	docTemplate   *exec.Template
}

func newRankerTemplates(queryTemplate, docTemplate string) (rankerTemplates, error) {
	qTpl, err := gonja.FromString(queryTemplate)
	if err != nil {
		return rankerTemplates{}, err
	}
	dTpl, err := gonja.FromString(docTemplate)
	if err != nil {
		return rankerTemplates{}, err
	}
	return rankerTemplates{queryTemplate: qTpl, docTemplate: dTpl}, nil
}

func (r *rankerTemplates) render(user *data.User, feedback []*FeedbackItem, items []*data.Item) (string, []string, error) {
	var query strings.Builder
	if err := r.queryTemplate.Execute(&query, exec.NewContext(map[string]any{
		"user":     user,
		"feedback": feedback,
	})); err != nil {
		return "", nil, err
	}
	documents := make([]string, len(items))
	for i, item := range items {
		var document strings.Builder
		if err := r.docTemplate.Execute(&document, exec.NewContext(map[string]any{"item": item})); err != nil {
			return "", nil, err
		}
		documents[i] = document.String()
	}
	return query.String(), documents, nil
}

type Reranker struct {
	rankerTemplates
	client *reranker.Client
	model  string
}

func NewReranker(cfg config.RerankerAPIConfig) (*Reranker, error) {
	templates, err := newRankerTemplates(cfg.QueryTemplate, cfg.DocumentTemplate)
	if err != nil {
		return nil, err
	}
	return &Reranker{
		rankerTemplates: templates,
		client:          reranker.NewClient(cfg.AuthToken, cfg.URL),
		model:           cfg.Model,
	}, nil
}

func (r *Reranker) Rank(ctx context.Context, user *data.User, feedback []*FeedbackItem, items []*data.Item) ([]cache.Score, error) {
	query, documents, err := r.render(user, feedback, items)
	if err != nil {
		return nil, err
	}
	// rerank
	resp, err := rankWithRetry(ctx, func() (*reranker.RerankResponse, error) {
		return r.client.Rerank(ctx, reranker.RerankRequest{
			Model:     r.model,
			Query:     query,
			Documents: documents,
		})
	})
	if err != nil {
		return nil, err
	}
	// sort items
	result := make([]cache.Score, len(resp.Results))
	for i, rerankResult := range resp.Results {
		result[i].Id = items[rerankResult.Index].ItemId
		result[i].Score = rerankResult.RelevanceScore
	}
	return result, nil
}

// Decision scores candidates against a user query using the decisions API.
type Decision struct {
	rankerTemplates
	client *decision.Client
}

func NewDecision(cfg config.DecisionAPIConfig) (*Decision, error) {
	templates, err := newRankerTemplates(cfg.StateTemplate, cfg.InstructionsTemplate)
	if err != nil {
		return nil, err
	}
	return &Decision{rankerTemplates: templates, client: decision.NewClient(cfg.AuthToken, cfg.URL, cfg.Model)}, nil
}

func (r *Decision) Rank(ctx context.Context, user *data.User, feedback []*FeedbackItem, items []*data.Item) ([]cache.Score, error) {
	if len(items) == 0 {
		return nil, nil
	}
	query, documents, err := r.render(user, feedback, items)
	if err != nil {
		return nil, err
	}
	questions := make(decision.Questions, len(items))
	for i, item := range items {
		questions[item.ItemId] = decision.NoulQuestion{
			Instructions: documents[i],
		}
	}
	response, err := rankWithRetry(ctx, func() (*decision.Response, error) {
		return r.client.Evaluate(ctx, query, questions)
	})
	if err != nil {
		return nil, err
	}
	scores := make([]cache.Score, 0, len(items))
	for _, item := range items {
		answer, exists := response.Answers[item.ItemId]
		if !exists || answer.Type != "noul" || answer.Noul == nil {
			return nil, fmt.Errorf("decision response is missing a noul probability for item %q", item.ItemId)
		}
		score := *answer.Noul
		if math.IsNaN(score) || math.IsInf(score, 0) || score < 0 || score > 1 {
			return nil, fmt.Errorf("decision response has an invalid noul probability for item %q: %g", item.ItemId, score)
		}
		scores = append(scores, cache.Score{Id: item.ItemId, Score: score})
	}
	cache.SortDocuments(scores)
	return scores, nil
}

func rankWithRetry[T any](ctx context.Context, rank func() (T, error)) (T, error) {
	const maxAttempts = 3
	var zero T
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		result, err := rank()
		if !isRetryableRankError(err) || attempt == maxAttempts {
			return result, err
		}
		delay := time.Second << (attempt - 1)
		log.Logger().Warn("ranker request failed, retrying",
			zap.Int("attempt", attempt),
			zap.Int("max_attempts", maxAttempts),
			zap.Duration("retry_delay", delay),
			zap.Error(err))
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return zero, ctx.Err()
		case <-timer.C:
		}
	}
}

func isRetryableRankError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var rerankErr *reranker.APIError
	var decisionErr *decision.APIError
	var statusCode int
	if errors.As(err, &rerankErr) {
		statusCode = rerankErr.StatusCode
	} else if errors.As(err, &decisionErr) {
		statusCode = decisionErr.StatusCode
	}
	if statusCode != 0 {
		switch statusCode {
		case http.StatusRequestTimeout, http.StatusTooManyRequests,
			http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout,
			520, 521, 522, 523, 524:
			return true
		default:
			return false
		}
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}
	return errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNABORTED) ||
		errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ETIMEDOUT)
}
