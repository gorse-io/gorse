package cache

import "errors"

func (suite *baseTestSuite) TestItemEmbeddingQueue() {
	ctx := suite.T().Context()
	req := suite.Require()
	const item = "item;/雪"
	req.NoError(EnqueueItemEmbeddings(ctx, suite.Database, []string{item, item, "other"}))
	fail := errors.New("refresh failed")
	req.ErrorIs(ProcessItemEmbeddings(ctx, suite.Database, 10, func(ids []string) error {
		req.ElementsMatch([]string{item, "other"}, ids)
		return fail
	}), fail)
	var first []string
	req.NoError(ProcessItemEmbeddings(ctx, suite.Database, 1, func(ids []string) error {
		req.Len(ids, 1)
		first = ids
		return EnqueueItemEmbeddings(ctx, suite.Database, ids)
	}))
	req.Len(first, 1)
	var remaining []string
	req.NoError(ProcessItemEmbeddings(ctx, suite.Database, 10, func(ids []string) error { remaining = ids; return nil }))
	req.ElementsMatch([]string{item, "other"}, remaining)
	req.NoError(ProcessItemEmbeddings(ctx, suite.Database, 10, func([]string) error { return fail }))
}
