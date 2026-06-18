package selector

import (
	"sync"

	"github.com/lHumaNl/gomock/internal/domain/mapping"
)

type randomSelector struct {
	mu       sync.Mutex
	nextInt  intnFunc
	variants []mapping.Response
}

func NewRandom(variants []mapping.Response) Selector {
	return newRandomSelector(variants, newSeededIntn())
}

func newRandomSelector(variants []mapping.Response, nextInt intnFunc) Selector {
	return &randomSelector{variants: copyResponses(variants), nextInt: nextInt}
}

func (s *randomSelector) Select() mapping.Response {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.variants[s.nextInt(len(s.variants))]
}
