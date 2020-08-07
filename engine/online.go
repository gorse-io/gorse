package engine

type RecommendedItems []RecommendedItem

func (items RecommendedItems) Len() int {
	return len(items)
}

func (items RecommendedItems) Less(i, j int) bool {
	return items[i].Score > items[j].Score
}

func (items RecommendedItems) Swap(i, j int) {
	items[i], items[j] = items[j], items[i]
}
