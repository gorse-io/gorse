package engine

import (
	"math"
	"sort"
)

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

// Ranking ranks items by popularity, timestamps and collaborative filtering scores..
func Ranking(items []RecommendedItem, n int, p, t, c float64) []RecommendedItem {
	if len(items) == 0 {
		return items
	}
	// copy
	cpy := make([]RecommendedItem, len(items))
	copy(cpy, items)
	// find the minimal and maximal
	pMin := items[0].Popularity
	pMax := pMin
	tMin := float64(items[0].Timestamp.Unix())
	tMax := tMin
	cMin := items[0].Score
	cMax := cMin
	for _, item := range items {
		pMin = math.Min(pMin, item.Popularity)
		pMax = math.Max(pMax, item.Popularity)
		tMin = math.Min(tMin, float64(item.Timestamp.Unix()))
		tMax = math.Max(tMax, float64(item.Timestamp.Unix()))
		cMin = math.Min(cMin, item.Score)
		cMax = math.Max(cMax, item.Score)
	}
	// normalize
	popularity, timestamps, collaborative := make([]float64, len(items)), make([]float64, len(items)), make([]float64, len(items))
	for i := range popularity {
		if pMin != pMax {
			popularity[i] = (items[i].Popularity - pMin) / (pMax - pMin)
		}
		if tMin != tMax {
			timestamps[i] = (float64(items[i].Timestamp.Unix()) - tMin) / (tMax - tMin)
		}
		if cMin != cMax {
			collaborative[i] = (items[i].Score - cMin) / (cMax - cMin)
		}
	}
	// scoring
	for i := range cpy {
		cpy[i].Score = p*popularity[i] + t*timestamps[i] + c*collaborative[i]
	}
	// sorting
	sort.Sort(RecommendedItems(cpy))
	// truncate
	if n > 0 && n < len(cpy) {
		cpy = cpy[:n]
	}
	return cpy
}
