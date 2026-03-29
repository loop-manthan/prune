package source

import (
	"context"

	"prune/internal/simulator"
)

// Source provides telemetry samples to the processing pipeline.
type Source interface {
	Start(ctx context.Context) error
	Samples() <-chan simulator.SensorSample
	Name() string
}
