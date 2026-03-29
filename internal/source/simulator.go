package source

import (
	"context"

	"prune/internal/simulator"
)

// SimulatorSource wraps the built-in deterministic simulator.
type SimulatorSource struct {
	ctrl *simulator.Controller
}

func NewSimulatorSource(seed int64) *SimulatorSource {
	return &SimulatorSource{ctrl: simulator.NewController(seed)}
}

func (s *SimulatorSource) Start(ctx context.Context) error {
	s.ctrl.Start(ctx)
	return nil
}

func (s *SimulatorSource) Samples() <-chan simulator.SensorSample {
	return s.ctrl.Samples()
}

func (s *SimulatorSource) Name() string {
	return "simulator"
}
