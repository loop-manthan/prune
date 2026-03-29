package source

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sync"
	"time"

	"prune/internal/simulator"
	"prune/pkg/telemetry"
)

const defaultOpenSkyURL = "https://opensky-network.org/api/states/all"

type openSkyResponse struct {
	Time   int64           `json:"time"`
	States [][]interface{} `json:"states"`
}

// OpenSkySource fetches live aircraft state and adapts it to project schema.
type OpenSkySource struct {
	url        string
	pollEvery  time.Duration
	httpClient *http.Client

	samples chan simulator.SensorSample

	mu      sync.RWMutex
	latest  simulator.SensorSample
	hasData bool
	seq     uint16
}

func NewOpenSkySource(pollEvery time.Duration) *OpenSkySource {
	if pollEvery <= 0 {
		pollEvery = time.Second
	}

	return &OpenSkySource{
		url:       defaultOpenSkyURL,
		pollEvery: pollEvery,
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
		},
		samples: make(chan simulator.SensorSample, 4),
	}
}

func (s *OpenSkySource) Start(ctx context.Context) error {
	if err := s.pollOnce(ctx); err != nil {
		return err
	}

	go s.pollLoop(ctx)
	go s.emitLoop(ctx)
	return nil
}

func (s *OpenSkySource) Samples() <-chan simulator.SensorSample {
	return s.samples
}

func (s *OpenSkySource) Name() string {
	return "opensky"
}

func (s *OpenSkySource) pollLoop(ctx context.Context) {
	ticker := time.NewTicker(s.pollEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.pollOnce(ctx)
		}
	}
}

func (s *OpenSkySource) emitLoop(ctx context.Context) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	defer close(s.samples)

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.mu.RLock()
			snapshot := s.latest
			hasData := s.hasData
			s.mu.RUnlock()

			if !hasData {
				continue
			}

			snapshot.Timestamp = now
			snapshot.SequenceNum = s.nextSeq()
			select {
			case s.samples <- snapshot:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (s *OpenSkySource) pollOnce(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("opensky status: %s", resp.Status)
	}

	var data openSkyResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}

	sample, ok := adaptOpenSky(data)
	if !ok {
		return fmt.Errorf("opensky: no valid state rows")
	}

	s.mu.Lock()
	s.latest = sample
	s.hasData = true
	s.mu.Unlock()
	return nil
}

func (s *OpenSkySource) nextSeq() uint16 {
	s.mu.Lock()
	defer s.mu.Unlock()
	seq := s.seq
	s.seq++
	return seq
}

func adaptOpenSky(data openSkyResponse) (simulator.SensorSample, bool) {
	for _, row := range data.States {
		alt, okAlt := toFloat(row, 7)      // baro_altitude
		vRate, okVRate := toFloat(row, 11) // vertical_rate
		speed, _ := toFloat(row, 9)        // velocity
		if !okAlt || !okVRate {
			continue
		}

		clampedAlt := clamp(alt, telemetry.AltitudeMin, telemetry.AltitudeMax)
		imuZ := clamp(vRate/2.0, telemetry.IMUMin, telemetry.IMUMax)
		imuX := clamp(speed/40.0, telemetry.IMUMin, telemetry.IMUMax)

		flags := telemetry.FlagCommsReady
		battery := 13.2
		if battery >= 12.5 {
			flags |= telemetry.FlagBatteryOk
		}
		flags |= telemetry.FlagIMUValid

		return simulator.SensorSample{
			Timestamp: time.Unix(data.Time, 0),
			Battery:   battery,
			Altitude:  clampedAlt,
			IMU: [3]float64{
				imuX,
				0,
				imuZ,
			},
			Flags: flags,
		}, true
	}

	return simulator.SensorSample{}, false
}

func toFloat(row []interface{}, idx int) (float64, bool) {
	if idx < 0 || idx >= len(row) || row[idx] == nil {
		return 0, false
	}
	v, ok := row[idx].(float64)
	return v, ok
}

func clamp(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}
