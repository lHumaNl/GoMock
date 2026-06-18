package selector

import (
	"errors"
	"math/rand"
	"time"

	"github.com/lHumaNl/gomock/internal/domain/mapping"
)

var (
	ErrNoResponseVariants = errors.New("response selector requires at least one variant")
	ErrUnsupportedMode    = errors.New("unsupported response selector mode")
)

type Selector interface {
	Select() mapping.Response
}

func NewSingle(response mapping.Response) Selector {
	return singleSelector{response: response}
}

func NewSet(set mapping.ResponseSet) (Selector, error) {
	if len(set.Variants) == 0 {
		return nil, ErrNoResponseVariants
	}
	switch set.Mode {
	case mapping.ResponseModeSequential:
		return NewSequential(set.Variants), nil
	case mapping.ResponseModeRandom:
		return NewRandom(set.Variants), nil
	case mapping.ResponseModeWeighted:
		return NewWeighted(set.Variants), nil
	default:
		return nil, ErrUnsupportedMode
	}
}

type intnFunc func(int) int

func newSeededIntn() intnFunc {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return rng.Intn
}
