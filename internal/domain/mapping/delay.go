package mapping

import "time"

type DelayType string

const (
	DelayTypeFixed  DelayType = "fixed"
	DelayTypeRandom DelayType = "random"
)

type Delay struct {
	Type  DelayType
	Value time.Duration
	Min   time.Duration
	Max   time.Duration
}
