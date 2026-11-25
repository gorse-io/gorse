package cache

import "errors"

func (s *baseTestSuite) TestItemEmbeddingQueue() {
	ctx := s.T().Context()
	req := s.Require()
	const item = "item;/雪"
	req.NoError(EnqueueItemEmbeddings(ctx, s.Database, []string{item, item, "other"}))
	fail := errors.New("refresh failed")
	req.ErrorIs(ProcessItemEmbeddings(ctx, s.Database, 10, func(ids []string) error {
		req.ElementsMatch([]string{item, "other"}, ids)
		return fail
	}), fail)
	var first []string
	req.NoError(ProcessItemEmbeddings(ctx, s.Database, 1, func(ids []string) error {
		req.Len(ids, 1)
		first = ids
		return EnqueueItemEmbeddings(ctx, s.Database, ids)
	}))
	req.Len(first, 1)
	var remaining []string
	req.NoError(ProcessItemEmbeddings(ctx, s.Database, 10, func(ids []string) error { remaining = ids; return nil }))
	req.ElementsMatch([]string{item, "other"}, remaining)
	req.NoError(ProcessItemEmbeddings(ctx, s.Database, 10, func([]string) error { return fail }))
}
