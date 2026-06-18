package selector

import (
	"sync"

	"github.com/lHumaNl/gomock/internal/domain/mapping"
)

type weightedSelector struct {
	mu         sync.Mutex
	nextInt    intnFunc
	total      int
	cumulative []int
	variants   []mapping.Response
}

func NewWeighted(variants []mapping.Response) Selector {
	return newWeightedSelector(variants, newSeededIntn())
}

func newWeightedSelector(variants []mapping.Response, nextInt intnFunc) Selector {
	selector := &weightedSelector{variants: copyResponses(variants), nextInt: nextInt}
	selector.buildCumulative()
	return selector
}

func (s *weightedSelector) Select() mapping.Response {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.variants[s.index(s.nextInt(s.total))]
}

func (s *weightedSelector) buildCumulative() {
	s.cumulative = make([]int, 0, len(s.variants))
	for _, variant := range s.variants {
		s.total += variant.Weight
		s.cumulative = append(s.cumulative, s.total)
	}
}

func (s *weightedSelector) index(point int) int {
	for index, threshold := range s.cumulative {
		if point < threshold {
			return index
		}
	}
	return len(s.variants) - 1
}
