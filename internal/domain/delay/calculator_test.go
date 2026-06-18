package delay

import (
	"testing"
	"time"

	"github.com/lHumaNl/gomock/internal/domain/mapping"
)

func TestCalculatorReturnsFixedDelay(t *testing.T) {
	calculator := NewCalculatorWithSeed(1)
	config := &mapping.Delay{Type: mapping.DelayTypeFixed, Value: 25 * time.Millisecond}

	duration := calculator.Duration(config)

	if duration != 25*time.Millisecond {
		t.Fatalf("expected fixed delay, got %s", duration)
	}
}

func TestCalculatorReturnsRandomDelayWithinRange(t *testing.T) {
	calculator := NewCalculatorWithSeed(1)
	config := &mapping.Delay{Type: mapping.DelayTypeRandom, Min: 10 * time.Millisecond, Max: 20 * time.Millisecond}

	for range 100 {
		duration := calculator.Duration(config)
		if duration < config.Min || duration > config.Max {
			t.Fatalf("expected random delay in range, got %s", duration)
		}
	}
}

func TestCalculatorReturnsRandomMinimumWhenRangeIsEmpty(t *testing.T) {
	calculator := NewCalculatorWithSeed(1)
	config := &mapping.Delay{Type: mapping.DelayTypeRandom, Min: 15 * time.Millisecond, Max: 15 * time.Millisecond}

	duration := calculator.Duration(config)

	if duration != config.Min {
		t.Fatalf("expected minimum delay, got %s", duration)
	}
}
