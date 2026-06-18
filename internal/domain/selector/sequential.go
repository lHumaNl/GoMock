package selector

import (
	"sync"

	"github.com/lHumaNl/gomock/internal/domain/mapping"
)

type sequentialSelector struct {
	mu       sync.Mutex
	next     int
	variants []mapping.Response
}

func NewSequential(variants []mapping.Response) Selector {
	return &sequentialSelector{variants: copyResponses(variants)}
}

func (s *sequentialSelector) Select() mapping.Response {
	s.mu.Lock()
	defer s.mu.Unlock()
	response := s.variants[s.next]
	s.next = (s.next + 1) % len(s.variants)
	return response
}

func copyResponses(responses []mapping.Response) []mapping.Response {
	copied := make([]mapping.Response, len(responses))
	copy(copied, responses)
	return copied
}
