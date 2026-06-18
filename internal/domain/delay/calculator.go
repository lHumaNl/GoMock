package delay

import (
	"math/rand"
	"sync"
	"time"

	"github.com/lHumaNl/gomock/internal/domain/mapping"
)

type Calculator struct {
	mu  sync.Mutex
	rng *rand.Rand
}

func NewCalculator() *Calculator {
	return NewCalculatorWithSeed(time.Now().UnixNano())
}

func NewCalculatorWithSeed(seed int64) *Calculator {
	return &Calculator{rng: rand.New(rand.NewSource(seed))}
}

func (c *Calculator) Duration(config *mapping.Delay) time.Duration {
	if config == nil {
		return 0
	}
	if config.Type == mapping.DelayTypeRandom {
		return c.randomDuration(config.Min, config.Max)
	}
	return config.Value
}

func (c *Calculator) randomDuration(minimum time.Duration, maximum time.Duration) time.Duration {
	if minimum >= maximum {
		return minimum
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return minimum + time.Duration(c.rng.Int63n(int64(maximum-minimum)))
}
