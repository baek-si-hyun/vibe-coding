package quant

import (
	"encoding/json"
	"os"
	"strings"
	"time"
)

type lstmTuningWindow struct {
	SnapshotCount          int     `json:"snapshot_count"`
	EvaluatedCount         int     `json:"evaluated_count"`
	DirectionHitProbRate   float64 `json:"direction_hit_prob_rate"`
	DirectionHitReturnRate float64 `json:"direction_hit_return_rate"`
	AvgPredReturn1D        float64 `json:"avg_pred_return_1d"`
	AvgActualReturn1D      float64 `json:"avg_actual_return_1d"`
	AvgAbsError1D          float64 `json:"avg_abs_error_1d"`
	BrierScore1D           float64 `json:"brier_score_1d"`
	BullishHitRate         float64 `json:"bullish_hit_rate"`
	TopK                   int     `json:"top_k"`
	TopKCount              int     `json:"top_k_count"`
	TopKHitRate            float64 `json:"top_k_hit_rate"`
	TopKAvgActualReturn1D  float64 `json:"top_k_avg_actual_return_1d"`
}

type lstmTuningProfileFile struct {
	GeneratedAt   string           `json:"generated_at"`
	SnapshotCount int              `json:"snapshot_count"`
	Recent5       lstmTuningWindow `json:"recent_5"`
	Recent20      lstmTuningWindow `json:"recent_20"`
	Profile       struct {
		Mode                   string   `json:"mode"`
		WeightMultiplier       float64  `json:"weight_multiplier"`
		StrictProbUpDelta      float64  `json:"strict_prob_up_delta"`
		StrictConfidenceDelta  float64  `json:"strict_confidence_delta"`
		StrictPredReturnDelta  float64  `json:"strict_pred_return_delta"`
		RelaxedProbUpDelta     float64  `json:"relaxed_prob_up_delta"`
		RelaxedConfidenceDelta float64  `json:"relaxed_confidence_delta"`
		RelaxedPredReturnDelta float64  `json:"relaxed_pred_return_delta"`
		SafetyProbUpDelta      float64  `json:"safety_prob_up_delta"`
		SafetyConfidenceDelta  float64  `json:"safety_confidence_delta"`
		SafetyPredReturnDelta  float64  `json:"safety_pred_return_delta"`
		Reasoning              []string `json:"reasoning"`
	} `json:"profile"`
}

type lstmTuningProfile struct {
	Enabled                bool
	GeneratedAt            string
	SnapshotCount          int
	Mode                   string
	WeightMultiplier       float64
	StrictProbUpDelta      float64
	StrictConfidenceDelta  float64
	StrictPredReturnDelta  float64
	RelaxedProbUpDelta     float64
	RelaxedConfidenceDelta float64
	RelaxedPredReturnDelta float64
	SafetyProbUpDelta      float64
	SafetyConfidenceDelta  float64
	SafetyPredReturnDelta  float64
	Recent5                lstmTuningWindow
	Recent20               lstmTuningWindow
	Reasoning              []string
}

type lstmTuningCache struct {
	path    string
	modTime time.Time
	profile lstmTuningProfile
}

func normalizeTuningMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "aggressive":
		return "aggressive"
	case "conservative":
		return "conservative"
	case "warmup":
		return "warmup"
	default:
		return "balanced"
	}
}

func (s *Service) loadLSTMTuning() (lstmTuningProfile, error) {
	path := strings.TrimSpace(s.cfg.LSTMTuningPath)
	if path == "" {
		return lstmTuningProfile{}, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return lstmTuningProfile{}, nil
		}
		return lstmTuningProfile{}, err
	}

	s.mu.RLock()
	cached := s.lstmTuning
	s.mu.RUnlock()
	if cached.path == path && info.ModTime().Equal(cached.modTime) {
		return cached.profile, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return lstmTuningProfile{}, err
	}

	var payload lstmTuningProfileFile
	if err := json.Unmarshal(content, &payload); err != nil {
		return lstmTuningProfile{}, err
	}

	profile := lstmTuningProfile{
		GeneratedAt:            strings.TrimSpace(payload.GeneratedAt),
		SnapshotCount:          payload.SnapshotCount,
		Mode:                   normalizeTuningMode(payload.Profile.Mode),
		WeightMultiplier:       clampFloat(payload.Profile.WeightMultiplier, 0.35, 1.35),
		StrictProbUpDelta:      clampFloat(payload.Profile.StrictProbUpDelta, -6, 8),
		StrictConfidenceDelta:  clampFloat(payload.Profile.StrictConfidenceDelta, -6, 8),
		StrictPredReturnDelta:  clampFloat(payload.Profile.StrictPredReturnDelta, -0.12, 0.12),
		RelaxedProbUpDelta:     clampFloat(payload.Profile.RelaxedProbUpDelta, -6, 8),
		RelaxedConfidenceDelta: clampFloat(payload.Profile.RelaxedConfidenceDelta, -6, 8),
		RelaxedPredReturnDelta: clampFloat(payload.Profile.RelaxedPredReturnDelta, -0.12, 0.12),
		SafetyProbUpDelta:      clampFloat(payload.Profile.SafetyProbUpDelta, -6, 8),
		SafetyConfidenceDelta:  clampFloat(payload.Profile.SafetyConfidenceDelta, -6, 8),
		SafetyPredReturnDelta:  clampFloat(payload.Profile.SafetyPredReturnDelta, -0.12, 0.12),
		Recent5:                payload.Recent5,
		Recent20:               payload.Recent20,
		Reasoning:              payload.Profile.Reasoning,
	}
	profile.Enabled = profile.SnapshotCount > 0 && profile.Recent20.EvaluatedCount > 0
	if profile.WeightMultiplier == 0 {
		profile.WeightMultiplier = 1
	}

	s.mu.Lock()
	s.lstmTuning = lstmTuningCache{
		path:    path,
		modTime: info.ModTime(),
		profile: profile,
	}
	s.mu.Unlock()

	return profile, nil
}
