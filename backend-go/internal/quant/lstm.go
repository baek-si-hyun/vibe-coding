package quant

import (
	"encoding/json"
	"math"
	"os"
	"strings"
	"time"
)

const defaultLSTMModelVersion = "lstm_signal_v1"

type lstmPredictionEntry struct {
	Market               string  `json:"market"`
	Code                 string  `json:"code"`
	Name                 string  `json:"name"`
	AsOf                 string  `json:"as_of"`
	PredReturn1D         float64 `json:"pred_return_1d"`
	PredReturn5D         float64 `json:"pred_return_5d"`
	PredReturn20D        float64 `json:"pred_return_20d"`
	ProbUp               float64 `json:"prob_up"`
	Confidence           float64 `json:"confidence"`
	ValidationAccuracy1D float64 `json:"validation_accuracy_1d,omitempty"`
	ValidationBrier1D    float64 `json:"validation_brier_1d,omitempty"`
	ModelVersion         string  `json:"model_version,omitempty"`
	TrainSamples         int     `json:"train_samples,omitempty"`
	ValidationSize       int     `json:"validation_size,omitempty"`
}

type lstmPredictionFile struct {
	GeneratedAt    string                `json:"generated_at"`
	ModelVersion   string                `json:"model_version"`
	PredictionAsOf string                `json:"prediction_as_of"`
	LookbackDays   int                   `json:"lookback_days,omitempty"`
	Horizon1D      int                   `json:"horizon_1d,omitempty"`
	Horizon5D      int                   `json:"horizon_5d,omitempty"`
	Horizon20D     int                   `json:"horizon_20d,omitempty"`
	ItemCount      int                   `json:"item_count,omitempty"`
	Items          []lstmPredictionEntry `json:"items"`
}

type lstmPredictionSnapshot struct {
	Enabled            bool
	CoverageSufficient bool
	GeneratedAt        string
	ModelVersion       string
	PredictionAsOf     string
	LookbackDays       int
	Horizon1D          int
	Horizon5D          int
	Horizon20D         int
	ItemCount          int
	Weight             float64
	Index              map[string]lstmPredictionEntry
}

type lstmPredictionCache struct {
	path     string
	modTime  time.Time
	snapshot lstmPredictionSnapshot
}

func normalizePredictionMarket(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "KOSPI":
		return "KOSPI"
	case "KOSDAQ":
		return "KOSDAQ"
	default:
		return ""
	}
}

func makeLSTMKey(market, code string) string {
	normalizedCode := strings.TrimSpace(code)
	normalizedMarket := normalizePredictionMarket(market)
	if normalizedMarket == "" {
		return normalizedCode
	}
	return normalizedMarket + ":" + normalizedCode
}

func unitClamp(v float64) float64 {
	if !isFinite(v) {
		return math.NaN()
	}
	if v > 1 && v <= 100 {
		v /= 100
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func zeroToNaN(v float64) float64 {
	if !isFinite(v) {
		return math.NaN()
	}
	return v
}

func shouldReplaceLSTMPrediction(existing, candidate lstmPredictionEntry) bool {
	if candidate.AsOf > existing.AsOf {
		return true
	}
	if candidate.AsOf < existing.AsOf {
		return false
	}
	if candidate.Confidence > existing.Confidence {
		return true
	}
	if candidate.Confidence < existing.Confidence {
		return false
	}
	return candidate.ModelVersion != "" && existing.ModelVersion == ""
}

func (s *Service) loadLSTMPredictions() (lstmPredictionSnapshot, error) {
	snapshot := lstmPredictionSnapshot{
		Weight: s.cfg.LSTMWeight,
	}
	path := strings.TrimSpace(s.cfg.LSTMPredictionsPath)
	if path == "" {
		return snapshot, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return snapshot, nil
		}
		return snapshot, err
	}

	s.mu.RLock()
	cached := s.lstmCache
	s.mu.RUnlock()
	if cached.path == path && info.ModTime().Equal(cached.modTime) {
		return cached.snapshot, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return snapshot, err
	}

	var payload lstmPredictionFile
	if err := json.Unmarshal(content, &payload); err != nil {
		return snapshot, err
	}

	modelVersion := strings.TrimSpace(payload.ModelVersion)
	if modelVersion == "" {
		modelVersion = defaultLSTMModelVersion
	}

	index := make(map[string]lstmPredictionEntry, len(payload.Items)*2)
	uniqueCount := 0
	for _, raw := range payload.Items {
		code := strings.TrimSpace(raw.Code)
		if code == "" {
			continue
		}

		entry := lstmPredictionEntry{
			Market:               normalizePredictionMarket(raw.Market),
			Code:                 code,
			Name:                 strings.TrimSpace(raw.Name),
			AsOf:                 strings.TrimSpace(raw.AsOf),
			PredReturn1D:         zeroToNaN(raw.PredReturn1D),
			PredReturn5D:         zeroToNaN(raw.PredReturn5D),
			PredReturn20D:        zeroToNaN(raw.PredReturn20D),
			ProbUp:               unitClamp(raw.ProbUp),
			Confidence:           unitClamp(raw.Confidence),
			ValidationAccuracy1D: unitClamp(raw.ValidationAccuracy1D),
			ValidationBrier1D:    unitClamp(raw.ValidationBrier1D / 0.25),
			ModelVersion:         strings.TrimSpace(raw.ModelVersion),
			TrainSamples:         raw.TrainSamples,
			ValidationSize:       raw.ValidationSize,
		}
		if entry.AsOf == "" {
			entry.AsOf = strings.TrimSpace(payload.PredictionAsOf)
		}
		if entry.ModelVersion == "" {
			entry.ModelVersion = modelVersion
		}
		if !isFinite(entry.PredReturn1D) || !isFinite(entry.ProbUp) || !isFinite(entry.Confidence) {
			continue
		}

		primaryKey := makeLSTMKey(entry.Market, entry.Code)
		if _, ok := index[primaryKey]; !ok {
			uniqueCount++
		}
		if existing, ok := index[primaryKey]; !ok || shouldReplaceLSTMPrediction(existing, entry) {
			index[primaryKey] = entry
		}
		codeOnlyKey := makeLSTMKey("", entry.Code)
		if existing, ok := index[codeOnlyKey]; !ok || shouldReplaceLSTMPrediction(existing, entry) {
			index[codeOnlyKey] = entry
		}
	}

	if len(index) == 0 {
		snapshot.ModelVersion = modelVersion
		snapshot.GeneratedAt = strings.TrimSpace(payload.GeneratedAt)
		snapshot.PredictionAsOf = strings.TrimSpace(payload.PredictionAsOf)
		snapshot.LookbackDays = payload.LookbackDays
		snapshot.Horizon1D = payload.Horizon1D
		snapshot.Horizon5D = payload.Horizon5D
		snapshot.Horizon20D = payload.Horizon20D
		snapshot.ItemCount = 0
	} else {
		snapshot.GeneratedAt = strings.TrimSpace(payload.GeneratedAt)
		snapshot.ModelVersion = modelVersion
		snapshot.PredictionAsOf = strings.TrimSpace(payload.PredictionAsOf)
		snapshot.LookbackDays = payload.LookbackDays
		snapshot.Horizon1D = payload.Horizon1D
		snapshot.Horizon5D = payload.Horizon5D
		snapshot.Horizon20D = payload.Horizon20D
		snapshot.ItemCount = uniqueCount
		snapshot.Index = index
		snapshot.Enabled = uniqueCount > 0
		snapshot.CoverageSufficient = uniqueCount >= s.cfg.LSTMMinPredictionCount
	}

	s.mu.Lock()
	s.lstmCache = lstmPredictionCache{
		path:     path,
		modTime:  info.ModTime(),
		snapshot: snapshot,
	}
	s.mu.Unlock()

	return snapshot, nil
}

func (snapshot lstmPredictionSnapshot) find(market, code, asOf string) (lstmPredictionEntry, bool) {
	if !snapshot.Enabled || strings.TrimSpace(code) == "" {
		return lstmPredictionEntry{}, false
	}

	keys := []string{
		makeLSTMKey(market, code),
		makeLSTMKey("", code),
	}
	for _, key := range keys {
		entry, ok := snapshot.Index[key]
		if !ok {
			continue
		}
		predictionAsOf := strings.TrimSpace(entry.AsOf)
		if predictionAsOf == "" {
			predictionAsOf = strings.TrimSpace(snapshot.PredictionAsOf)
		}
		if predictionAsOf != "" && strings.TrimSpace(asOf) != "" && predictionAsOf != strings.TrimSpace(asOf) {
			continue
		}
		return entry, true
	}

	return lstmPredictionEntry{}, false
}
