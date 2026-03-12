package ops

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"investment-news-go/internal/config"
	"investment-news-go/internal/krx"
	newssvc "investment-news-go/internal/news"
	nxtsvc "investment-news-go/internal/nxt"
)

const (
	checkpointQuantRequest  = "quant_request_sync"
	checkpointKRXDaily      = "krx_daily"
	checkpointNXTDelayed    = "nxt_delayed"
	checkpointNewsMidnight  = "news_midnight"
	checkpointNewsPreMarket = "news_pre_market"
	checkpointQuantModel    = "quant_model"

	scheduledSlotMidnight  = "midnight"
	scheduledSlotPreMarket = "pre_market"

	// NXT-inclusive Korea combined trading window.
	marketCloseHour     = 20
	marketCloseMinute   = 0
	preMarketOpenHour   = 8
	preMarketOpenMinute = 0
)

var (
	defaultNewsSyncSources = []string{"daum", "naver"}
	tradingCalendarDirs    = []string{"kospi_daily", "kosdaq_daily"}
)

type QuantSyncManager struct {
	cfg   config.Config
	store *StateStore
	krx   *krx.Service
	news  *newssvc.Service
	nxt   *nxtsvc.Service
	mu    sync.Mutex
}

type QuantSyncSnapshot struct {
	Session                string   `json:"session"`
	LatestAvailableDate    string   `json:"latest_available_date,omitempty"`
	CurrentTradingDate     string   `json:"current_trading_date,omitempty"`
	PreviousTradingDate    string   `json:"previous_trading_date,omitempty"`
	NextTradingDate        string   `json:"next_trading_date,omitempty"`
	KRXCollectDate         string   `json:"krx_collect_date,omitempty"`
	NXTTargetTradingDate   string   `json:"nxt_target_trading_date,omitempty"`
	NXTQuoteCount          int      `json:"nxt_quote_count,omitempty"`
	NewsTargetTradingDate  string   `json:"news_target_trading_date,omitempty"`
	ModelTargetTradingDate string   `json:"model_target_trading_date,omitempty"`
	ModelAsOf              string   `json:"model_as_of,omitempty"`
	Performed              []string `json:"performed,omitempty"`
	Skipped                []string `json:"skipped,omitempty"`
	Errors                 []string `json:"errors,omitempty"`
}

type quantSyncPlan struct {
	Session                string
	LatestAvailableDate    string
	CurrentTradingDate     string
	PreviousTradingDate    string
	NextTradingDate        string
	KRXCollectDate         string
	NXTTargetTradingDate   string
	NewsTargetTradingDate  string
	ModelTargetTradingDate string
	ModelAsOf              string
	NewsWindowStart        time.Time
	NewsWindowEnd          time.Time
	ShouldEnsureKRX        bool
	ShouldEnsureNXT        bool
	ShouldEnsureMidnight   bool
	ShouldEnsurePreMarket  bool
	ShouldEnsureModel      bool
}

type lstmSyncFile struct {
	PredictionAsOf string `json:"prediction_as_of"`
	ItemCount      int    `json:"item_count"`
	Items          []any  `json:"items"`
}

type lstmPredictionStatus struct {
	PredictionAsOf string
	ItemCount      int
}

func NewQuantSyncManager(cfg config.Config, store *StateStore, krxService *krx.Service, newsService *newssvc.Service, nxtService *nxtsvc.Service) *QuantSyncManager {
	return &QuantSyncManager{
		cfg:   cfg,
		store: store,
		krx:   krxService,
		news:  newsService,
		nxt:   nxtService,
	}
}

func (m *QuantSyncManager) EnsureQuantInputs(ctx context.Context) (QuantSyncSnapshot, error) {
	return m.ensureQuantInputs(ctx, true, "")
}

func (m *QuantSyncManager) EnsureQuantInputsForSlot(ctx context.Context, slot string) (QuantSyncSnapshot, error) {
	return m.ensureQuantInputs(ctx, true, slot)
}

func (m *QuantSyncManager) EnsureQuantInputsForRequest(ctx context.Context) (QuantSyncSnapshot, error) {
	return m.ensureQuantInputs(ctx, false, "request")
}

func (m *QuantSyncManager) ensureQuantInputs(ctx context.Context, allowModel bool, scheduledSlot string) (QuantSyncSnapshot, error) {
	snapshot := QuantSyncSnapshot{}
	if m == nil || !m.cfg.AutoQuantSync {
		snapshot.Skipped = append(snapshot.Skipped, "auto_quant_sync_disabled")
		return snapshot, nil
	}

	if allowModel {
		m.mu.Lock()
		defer m.mu.Unlock()
	} else {
		if !m.mu.TryLock() {
			snapshot.Skipped = append(snapshot.Skipped, "quant_sync_busy")
			return snapshot, nil
		}
		defer m.mu.Unlock()
	}

	now := time.Now().In(seoulLocation())
	_ = m.updateCheckpoint(checkpointQuantRequest, func(checkpoint *SyncCheckpoint) {
		checkpoint.LastAttemptAt = now.Format(time.RFC3339)
		checkpoint.Status = "running"
		checkpoint.Note = fmt.Sprintf("quant sync started (%s)", firstNonEmpty(strings.TrimSpace(scheduledSlot), "request"))
	})

	tradingDates, latestAvailable, err := loadTradingDates(m.cfg.DataRootDir, now)
	if err != nil {
		mark := fmt.Sprintf("거래일 캘린더 로드 실패: %s", err)
		_ = m.updateCheckpoint(checkpointQuantRequest, func(checkpoint *SyncCheckpoint) {
			checkpoint.Status = "error"
			checkpoint.Note = mark
		})
		return snapshot, err
	}

	plan := buildQuantSyncPlan(tradingDates, latestAvailable, now, scheduledSlot)
	snapshot.Session = plan.Session
	snapshot.LatestAvailableDate = plan.LatestAvailableDate
	snapshot.CurrentTradingDate = plan.CurrentTradingDate
	snapshot.PreviousTradingDate = plan.PreviousTradingDate
	snapshot.NextTradingDate = plan.NextTradingDate
	snapshot.KRXCollectDate = plan.KRXCollectDate
	snapshot.NXTTargetTradingDate = plan.NXTTargetTradingDate
	snapshot.NewsTargetTradingDate = plan.NewsTargetTradingDate
	snapshot.ModelTargetTradingDate = plan.ModelTargetTradingDate
	snapshot.ModelAsOf = plan.ModelAsOf

	if plan.ShouldEnsureKRX {
		if err := m.ensureKRX(plan.KRXCollectDate, now, &snapshot); err != nil {
			snapshot.Errors = append(snapshot.Errors, err.Error())
		}
	} else {
		snapshot.Skipped = append(snapshot.Skipped, "krx")
	}

	if plan.ShouldEnsureNXT {
		if err := m.ensureNXT(plan.NXTTargetTradingDate, now, &snapshot); err != nil {
			snapshot.Errors = append(snapshot.Errors, err.Error())
		}
	} else {
		snapshot.Skipped = append(snapshot.Skipped, "nxt")
	}

	if plan.ShouldEnsureMidnight {
		if err := m.ensureNews(checkpointNewsMidnight, plan.NewsTargetTradingDate, plan.NewsWindowStart, plan.NewsWindowEnd, now, &snapshot); err != nil {
			snapshot.Errors = append(snapshot.Errors, err.Error())
		}
	} else {
		snapshot.Skipped = append(snapshot.Skipped, "news_midnight")
	}

	if plan.ShouldEnsurePreMarket {
		if err := m.ensureNews(checkpointNewsPreMarket, plan.NewsTargetTradingDate, plan.NewsWindowStart, plan.NewsWindowEnd, now, &snapshot); err != nil {
			snapshot.Errors = append(snapshot.Errors, err.Error())
		}
	} else {
		snapshot.Skipped = append(snapshot.Skipped, "news_pre_market")
	}

	if plan.ShouldEnsureModel && allowModel {
		if err := m.ensureModel(ctx, plan.ModelTargetTradingDate, plan.ModelAsOf, forceModelRun(scheduledSlot), now, &snapshot); err != nil {
			snapshot.Errors = append(snapshot.Errors, err.Error())
		}
	} else if plan.ShouldEnsureModel {
		snapshot.Skipped = append(snapshot.Skipped, "quant_model_deferred")
	} else {
		snapshot.Skipped = append(snapshot.Skipped, "quant_model")
	}

	status := "success"
	note := "quant request sync completed"
	if len(snapshot.Errors) > 0 {
		status = "warning"
		note = strings.Join(snapshot.Errors, " | ")
	}
	_ = m.updateCheckpoint(checkpointQuantRequest, func(checkpoint *SyncCheckpoint) {
		checkpoint.LastSuccessAt = now.Format(time.RFC3339)
		checkpoint.LastTradingDate = firstNonEmpty(plan.ModelTargetTradingDate, plan.NewsTargetTradingDate, plan.KRXCollectDate, plan.CurrentTradingDate)
		checkpoint.LastAsOf = plan.ModelAsOf
		checkpoint.Status = status
		checkpoint.Note = note
		checkpoint.Extra["session"] = plan.Session
		checkpoint.Extra["performed"] = strings.Join(snapshot.Performed, ",")
		checkpoint.Extra["skipped"] = strings.Join(snapshot.Skipped, ",")
	})

	if len(snapshot.Errors) > 0 {
		return snapshot, fmt.Errorf(strings.Join(snapshot.Errors, "; "))
	}
	return snapshot, nil
}

func (m *QuantSyncManager) ensureKRX(targetDate string, now time.Time, snapshot *QuantSyncSnapshot) error {
	if strings.TrimSpace(targetDate) == "" {
		snapshot.Skipped = append(snapshot.Skipped, "krx")
		return nil
	}

	latestDate, err := latestCollectedKRXDate(m.cfg.DataRootDir)
	if err == nil && latestDate >= targetDate {
		_ = m.updateCheckpoint(checkpointKRXDaily, func(checkpoint *SyncCheckpoint) {
			checkpoint.LastSuccessAt = now.Format(time.RFC3339)
			checkpoint.LastTradingDate = targetDate
			checkpoint.LastAsOf = latestDate
			checkpoint.Status = "success"
			checkpoint.Note = "existing KRX data reused"
		})
		snapshot.Skipped = append(snapshot.Skipped, "krx_existing")
		return nil
	}

	_ = m.updateCheckpoint(checkpointKRXDaily, func(checkpoint *SyncCheckpoint) {
		checkpoint.LastAttemptAt = now.Format(time.RFC3339)
		checkpoint.LastTradingDate = targetDate
		checkpoint.Status = "running"
		checkpoint.Note = "collecting KRX daily data"
	})

	if _, err := m.krx.CollectAndSave(targetDate, nil); err != nil {
		_ = m.updateCheckpoint(checkpointKRXDaily, func(checkpoint *SyncCheckpoint) {
			checkpoint.Status = "error"
			checkpoint.Note = err.Error()
		})
		log.Printf("quant sync: KRX collect failed for %s: %v", targetDate, err)
		return fmt.Errorf("KRX 자동수집 실패(%s): %w", targetDate, err)
	}

	latestDate, latestErr := latestCollectedKRXDate(m.cfg.DataRootDir)
	if latestErr != nil || latestDate < targetDate {
		note := "KRX 자동수집 완료 후 최신 거래일을 검증하지 못했습니다"
		if latestErr == nil && latestDate != "" {
			note = fmt.Sprintf("KRX 자동수집 후 최신 거래일이 %s 로 남았습니다", latestDate)
		}
		_ = m.updateCheckpoint(checkpointKRXDaily, func(checkpoint *SyncCheckpoint) {
			checkpoint.Status = "warning"
			checkpoint.Note = note
		})
		snapshot.Errors = append(snapshot.Errors, note)
		return nil
	}

	_ = m.updateCheckpoint(checkpointKRXDaily, func(checkpoint *SyncCheckpoint) {
		checkpoint.LastSuccessAt = time.Now().Format(time.RFC3339)
		checkpoint.LastTradingDate = targetDate
		checkpoint.LastAsOf = latestDate
		checkpoint.Status = "success"
		checkpoint.Note = "KRX daily data refreshed"
	})
	snapshot.Performed = append(snapshot.Performed, "krx")
	return nil
}

func (m *QuantSyncManager) ensureNXT(targetDate string, now time.Time, snapshot *QuantSyncSnapshot) error {
	if strings.TrimSpace(targetDate) == "" || m.nxt == nil {
		snapshot.Skipped = append(snapshot.Skipped, "nxt")
		return nil
	}

	existing, err := m.nxt.LoadSnapshot(targetDate)
	if err == nil && existing.TradingDate == targetDate && existing.QuoteCount > 0 {
		_ = m.updateCheckpoint(checkpointNXTDelayed, func(cp *SyncCheckpoint) {
			cp.LastSuccessAt = now.Format(time.RFC3339)
			cp.LastTradingDate = targetDate
			cp.Status = "success"
			cp.Note = "existing NXT delayed snapshot reused"
			cp.Extra["quote_count"] = fmt.Sprintf("%d", existing.QuoteCount)
			cp.Extra["set_time"] = existing.SetTime
		})
		snapshot.NXTQuoteCount = existing.QuoteCount
		snapshot.Skipped = append(snapshot.Skipped, "nxt_existing")
		return nil
	}

	_ = m.updateCheckpoint(checkpointNXTDelayed, func(cp *SyncCheckpoint) {
		cp.LastAttemptAt = now.Format(time.RFC3339)
		cp.LastTradingDate = targetDate
		cp.Status = "running"
		cp.Note = "collecting NXT delayed snapshot"
	})

	collected, err := m.nxt.CollectAndSave(targetDate)
	if err != nil {
		_ = m.updateCheckpoint(checkpointNXTDelayed, func(cp *SyncCheckpoint) {
			cp.Status = "error"
			cp.Note = err.Error()
		})
		return fmt.Errorf("NXT 지연시세 수집 실패(%s): %w", targetDate, err)
	}

	_ = m.updateCheckpoint(checkpointNXTDelayed, func(cp *SyncCheckpoint) {
		cp.LastSuccessAt = time.Now().Format(time.RFC3339)
		cp.LastTradingDate = targetDate
		cp.Status = "success"
		cp.Note = "NXT delayed snapshot refreshed"
		cp.Extra["quote_count"] = fmt.Sprintf("%d", collected.QuoteCount)
		cp.Extra["set_time"] = collected.SetTime
	})
	snapshot.NXTQuoteCount = collected.QuoteCount
	snapshot.Performed = append(snapshot.Performed, "nxt")
	return nil
}

func (m *QuantSyncManager) ensureNews(checkpointKey, targetTradingDate string, windowStart, windowEnd, now time.Time, snapshot *QuantSyncSnapshot) error {
	if strings.TrimSpace(targetTradingDate) == "" {
		snapshot.Skipped = append(snapshot.Skipped, checkpointKey)
		return nil
	}

	checkpoint, err := m.store.GetCheckpoint(checkpointKey)
	if err == nil && checkpoint.Status == "success" && checkpoint.LastTradingDate == targetTradingDate {
		snapshot.Skipped = append(snapshot.Skipped, checkpointKey+"_existing")
		return nil
	}

	_ = m.updateCheckpoint(checkpointKey, func(cp *SyncCheckpoint) {
		cp.LastAttemptAt = now.Format(time.RFC3339)
		cp.LastTradingDate = targetTradingDate
		cp.WindowStart = formatTimeOrEmpty(windowStart)
		cp.WindowEnd = formatTimeOrEmpty(windowEnd)
		cp.Status = "running"
		cp.Note = "crawling news sources"
	})

	result := m.news.BackfillRecentTradingDays(targetTradingDate, m.cfg.NewsBackfillTradingDays, defaultNewsSyncSources)
	if rawErr := strings.TrimSpace(toString(result["error"])); rawErr != "" {
		_ = m.updateCheckpoint(checkpointKey, func(cp *SyncCheckpoint) {
			cp.Status = "error"
			cp.Note = rawErr
		})
		log.Printf("quant sync: news crawl failed for %s: %s", targetTradingDate, rawErr)
		return fmt.Errorf("%s 자동수집 실패(%s): %s", checkpointKey, targetTradingDate, rawErr)
	}

	message := strings.TrimSpace(toString(result["message"]))
	if message == "" {
		message = "news crawl completed"
	}
	_ = m.updateCheckpoint(checkpointKey, func(cp *SyncCheckpoint) {
		cp.LastSuccessAt = time.Now().Format(time.RFC3339)
		cp.LastTradingDate = targetTradingDate
		cp.WindowStart = formatTimeOrEmpty(windowStart)
		cp.WindowEnd = formatTimeOrEmpty(windowEnd)
		cp.Status = "success"
		cp.Note = message
		cp.Extra["trading_days"] = fmt.Sprint(result["trading_days"])
		cp.Extra["min_date"] = toString(result["min_date"])
	})
	snapshot.Performed = append(snapshot.Performed, checkpointKey)
	return nil
}

func forceModelRun(slot string) bool {
	switch strings.ToLower(strings.TrimSpace(slot)) {
	case scheduledSlotMidnight, scheduledSlotPreMarket:
		return true
	default:
		return false
	}
}

func (m *QuantSyncManager) ensureModel(ctx context.Context, targetTradingDate, asOf string, force bool, now time.Time, snapshot *QuantSyncSnapshot) error {
	if strings.TrimSpace(targetTradingDate) == "" || strings.TrimSpace(asOf) == "" {
		snapshot.Skipped = append(snapshot.Skipped, "quant_model")
		return nil
	}

	predictionStatus, err := readPredictionStatus(m.cfg.LSTMPredictionsPath)
	if err == nil && predictionStatus.PredictionAsOf == asOf && predictionStatus.ItemCount >= m.cfg.LSTMMinPredictionCount && !force {
		_ = m.updateCheckpoint(checkpointQuantModel, func(cp *SyncCheckpoint) {
			cp.LastSuccessAt = now.Format(time.RFC3339)
			cp.LastTradingDate = targetTradingDate
			cp.LastAsOf = asOf
			cp.Status = "success"
			cp.Note = fmt.Sprintf("existing LSTM prediction reused (count=%d)", predictionStatus.ItemCount)
		})
		snapshot.Skipped = append(snapshot.Skipped, "quant_model_existing")
		return nil
	}
	if err == nil && predictionStatus.PredictionAsOf == asOf && predictionStatus.ItemCount >= m.cfg.LSTMMinPredictionCount && force {
		log.Printf(
			"quant sync: forcing scheduled LSTM regeneration: asOf=%s count=%d",
			predictionStatus.PredictionAsOf,
			predictionStatus.ItemCount,
		)
	}
	if err == nil && predictionStatus.PredictionAsOf == asOf && predictionStatus.ItemCount < m.cfg.LSTMMinPredictionCount {
		log.Printf(
			"quant sync: existing LSTM prediction is sparse and will be regenerated: asOf=%s count=%d min=%d",
			predictionStatus.PredictionAsOf,
			predictionStatus.ItemCount,
			m.cfg.LSTMMinPredictionCount,
		)
	}

	scriptPath := strings.TrimSpace(m.cfg.LSTMBatchScriptPath)
	if scriptPath == "" {
		_ = m.updateCheckpoint(checkpointQuantModel, func(cp *SyncCheckpoint) {
			cp.Status = "error"
			cp.Note = "LSTM batch script path is empty"
		})
		return fmt.Errorf("LSTM 배치 스크립트 경로가 비어 있습니다")
	}
	if _, statErr := os.Stat(scriptPath); statErr != nil {
		_ = m.updateCheckpoint(checkpointQuantModel, func(cp *SyncCheckpoint) {
			cp.Status = "error"
			cp.Note = statErr.Error()
		})
		return fmt.Errorf("LSTM 배치 스크립트를 찾지 못했습니다: %s", scriptPath)
	}

	_ = m.updateCheckpoint(checkpointQuantModel, func(cp *SyncCheckpoint) {
		cp.LastAttemptAt = now.Format(time.RFC3339)
		cp.LastTradingDate = targetTradingDate
		cp.LastAsOf = asOf
		cp.Status = "running"
		cp.Note = "running LSTM export batch"
	})

	runCtx := ctx
	cancel := func() {}
	if runCtx == nil {
		runCtx = context.Background()
	}
	runCtx, cancel = context.WithTimeout(runCtx, 2*time.Hour)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "bash", scriptPath)
	cmd.Dir = firstNonEmpty(m.cfg.ProjectRootDir, filepath.Dir(m.cfg.BackendDir))
	output, runErr := cmd.CombinedOutput()
	outputNote := truncateNote(strings.TrimSpace(string(output)))
	if runErr != nil {
		note := strings.TrimSpace(firstNonEmpty(outputNote, runErr.Error()))
		_ = m.updateCheckpoint(checkpointQuantModel, func(cp *SyncCheckpoint) {
			cp.Status = "error"
			cp.Note = note
		})
		log.Printf("quant sync: LSTM batch failed for %s/%s: %v", targetTradingDate, asOf, runErr)
		return fmt.Errorf("LSTM 자동학습 실패(%s/%s): %s", targetTradingDate, asOf, note)
	}

	predictionStatus, err = readPredictionStatus(m.cfg.LSTMPredictionsPath)
	if err != nil {
		_ = m.updateCheckpoint(checkpointQuantModel, func(cp *SyncCheckpoint) {
			cp.Status = "error"
			cp.Note = err.Error()
		})
		return fmt.Errorf("LSTM 예측 파일 확인 실패: %w", err)
	}
	if predictionStatus.PredictionAsOf != asOf {
		note := fmt.Sprintf("LSTM 예측 기준일 mismatch expected=%s actual=%s", asOf, predictionStatus.PredictionAsOf)
		_ = m.updateCheckpoint(checkpointQuantModel, func(cp *SyncCheckpoint) {
			cp.Status = "error"
			cp.Note = note
		})
		return fmt.Errorf("LSTM 예측 기준일이 기대값과 다릅니다. expected=%s actual=%s", asOf, predictionStatus.PredictionAsOf)
	}
	if predictionStatus.ItemCount < m.cfg.LSTMMinPredictionCount {
		note := fmt.Sprintf(
			"LSTM prediction coverage is too low: actual=%d minimum=%d",
			predictionStatus.ItemCount,
			m.cfg.LSTMMinPredictionCount,
		)
		_ = m.updateCheckpoint(checkpointQuantModel, func(cp *SyncCheckpoint) {
			cp.Status = "error"
			cp.Note = note
		})
		return fmt.Errorf("LSTM 예측 커버리지가 부족합니다. actual=%d minimum=%d", predictionStatus.ItemCount, m.cfg.LSTMMinPredictionCount)
	}

	note := firstNonEmpty(outputNote, fmt.Sprintf("LSTM export batch completed (count=%d)", predictionStatus.ItemCount))
	_ = m.updateCheckpoint(checkpointQuantModel, func(cp *SyncCheckpoint) {
		cp.LastSuccessAt = time.Now().Format(time.RFC3339)
		cp.LastTradingDate = targetTradingDate
		cp.LastAsOf = asOf
		cp.Status = "success"
		cp.Note = note
	})
	snapshot.Performed = append(snapshot.Performed, "quant_model")
	return nil
}

func (m *QuantSyncManager) updateCheckpoint(key string, update func(*SyncCheckpoint)) error {
	if m == nil || m.store == nil {
		return nil
	}
	_, err := m.store.UpdateCheckpoint(key, update)
	return err
}

func buildQuantSyncPlan(tradingDates []string, latestAvailable string, now time.Time, scheduledSlot string) quantSyncPlan {
	plan := quantSyncPlan{
		LatestAvailableDate: latestAvailable,
	}
	if len(tradingDates) == 0 {
		return plan
	}

	today := now.Format("20060102")
	currentIndex := indexAtOrBefore(tradingDates, today)
	if currentIndex >= 0 {
		plan.CurrentTradingDate = tradingDates[currentIndex]
	}
	if currentIndex > 0 {
		plan.PreviousTradingDate = tradingDates[currentIndex-1]
	}
	for _, candidate := range tradingDates {
		if candidate > today {
			plan.NextTradingDate = candidate
			break
		}
	}

	slot := strings.ToLower(strings.TrimSpace(scheduledSlot))
	switch slot {
	case scheduledSlotMidnight, scheduledSlotPreMarket:
		plan.Session = slot
	default:
		isTodayTrading := plan.CurrentTradingDate == today
		switch {
		case isTodayTrading && beforePreMarketOpen(now):
			plan.Session = "pre_market"
		case isTodayTrading && atOrAfterMarketClose(now):
			plan.Session = "post_close"
		case isTodayTrading:
			plan.Session = "regular"
		default:
			plan.Session = "closed"
		}
	}

	switch plan.Session {
	case scheduledSlotMidnight:
		plan.KRXCollectDate = plan.PreviousTradingDate
		plan.NXTTargetTradingDate = plan.PreviousTradingDate
		plan.NewsTargetTradingDate = plan.CurrentTradingDate
		plan.ModelTargetTradingDate = plan.CurrentTradingDate
		plan.ModelAsOf = plan.PreviousTradingDate
		plan.ShouldEnsureKRX = plan.KRXCollectDate != ""
		plan.ShouldEnsureNXT = plan.NXTTargetTradingDate != ""
		plan.ShouldEnsureMidnight = plan.NewsTargetTradingDate != ""
		plan.ShouldEnsureModel = plan.ModelTargetTradingDate != "" && plan.ModelAsOf != ""
		plan.NewsWindowStart, plan.NewsWindowEnd = newsWindowBounds(plan.PreviousTradingDate, plan.CurrentTradingDate, now.Location())
	case "pre_market", "regular":
		plan.KRXCollectDate = plan.PreviousTradingDate
		plan.NXTTargetTradingDate = plan.PreviousTradingDate
		plan.NewsTargetTradingDate = plan.CurrentTradingDate
		plan.ModelTargetTradingDate = plan.CurrentTradingDate
		plan.ModelAsOf = plan.PreviousTradingDate
		plan.ShouldEnsureKRX = plan.KRXCollectDate != ""
		plan.ShouldEnsureNXT = plan.NXTTargetTradingDate != ""
		plan.ShouldEnsurePreMarket = plan.NewsTargetTradingDate != ""
		plan.ShouldEnsureModel = plan.ModelTargetTradingDate != "" && plan.ModelAsOf != ""
		plan.NewsWindowStart, plan.NewsWindowEnd = newsWindowBounds(plan.PreviousTradingDate, plan.CurrentTradingDate, now.Location())
	case "post_close":
		plan.KRXCollectDate = plan.CurrentTradingDate
		plan.NXTTargetTradingDate = plan.CurrentTradingDate
		plan.NewsTargetTradingDate = plan.NextTradingDate
		plan.ShouldEnsureKRX = plan.KRXCollectDate != ""
		plan.ShouldEnsureNXT = plan.NXTTargetTradingDate != ""
		plan.ShouldEnsureMidnight = plan.NewsTargetTradingDate != ""
		plan.NewsWindowStart, plan.NewsWindowEnd = newsWindowBounds(plan.CurrentTradingDate, plan.NextTradingDate, now.Location())
	case "closed":
		plan.KRXCollectDate = plan.CurrentTradingDate
		plan.NXTTargetTradingDate = plan.CurrentTradingDate
		plan.NewsTargetTradingDate = plan.NextTradingDate
		plan.ShouldEnsureKRX = plan.KRXCollectDate != ""
		plan.ShouldEnsureNXT = plan.NXTTargetTradingDate != ""
		plan.ShouldEnsureMidnight = plan.NewsTargetTradingDate != ""
		plan.NewsWindowStart, plan.NewsWindowEnd = newsWindowBounds(plan.CurrentTradingDate, plan.NextTradingDate, now.Location())
	}

	return plan
}

func loadTradingDates(dataRoot string, now time.Time) ([]string, string, error) {
	calendarPath, err := findTradingCalendarCSV(dataRoot)
	if err != nil {
		return nil, "", err
	}

	file, err := os.Open(calendarPath)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	header, err := reader.Read()
	if err != nil {
		return nil, "", err
	}

	dateIndex := -1
	for i, col := range header {
		if strings.TrimPrefix(strings.TrimSpace(col), "\ufeff") == "BAS_DD" {
			dateIndex = i
			break
		}
	}
	if dateIndex < 0 {
		return nil, "", fmt.Errorf("거래일 캘린더 CSV에 BAS_DD 컬럼이 없습니다")
	}

	seen := map[string]struct{}{}
	dates := make([]string, 0, 512)
	for {
		row, readErr := reader.Read()
		if readErr != nil {
			break
		}
		if dateIndex >= len(row) {
			continue
		}
		value := normalizeTradingDate(row[dateIndex])
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		dates = append(dates, value)
	}

	sort.Strings(dates)
	if len(dates) == 0 {
		return nil, "", fmt.Errorf("거래일 캘린더 데이터가 비어 있습니다")
	}
	latestAvailable := dates[len(dates)-1]
	return extendTradingDates(dates, now, seoulLocation()), latestAvailable, nil
}

func latestCollectedKRXDate(dataRoot string) (string, error) {
	_, latestAvailable, err := loadTradingDates(dataRoot, time.Now().In(seoulLocation()))
	if err != nil {
		return "", err
	}
	return latestAvailable, nil
}

func findTradingCalendarCSV(dataRoot string) (string, error) {
	bestPath := ""
	bestSize := int64(-1)
	for _, dirName := range tradingCalendarDirs {
		pattern := filepath.Join(dataRoot, dirName, "*.csv")
		paths, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		sort.Strings(paths)
		for _, path := range paths {
			info, statErr := os.Stat(path)
			if statErr != nil || info.IsDir() {
				continue
			}
			if info.Size() > bestSize {
				bestPath = path
				bestSize = info.Size()
			}
		}
	}
	if bestPath == "" {
		return "", fmt.Errorf("거래일 캘린더용 KRX CSV를 찾지 못했습니다")
	}
	return bestPath, nil
}

func extendTradingDates(dates []string, now time.Time, loc *time.Location) []string {
	if len(dates) == 0 {
		return []string{}
	}
	out := append([]string{}, dates...)
	latest := out[len(out)-1]
	limit := now.In(loc).AddDate(0, 0, 7).Format("20060102")
	seen := map[string]struct{}{}
	for _, value := range out {
		seen[value] = struct{}{}
	}
	for i := 0; i < 7; i++ {
		next := nextWeekdayTradingDate(latest, loc)
		if next == "" || next > limit {
			break
		}
		if _, ok := seen[next]; ok {
			latest = next
			continue
		}
		out = append(out, next)
		seen[next] = struct{}{}
		latest = next
	}
	return out
}

func readPredictionStatus(path string) (lstmPredictionStatus, error) {
	status := lstmPredictionStatus{}
	if strings.TrimSpace(path) == "" {
		return status, fmt.Errorf("prediction path is empty")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return status, err
	}
	payload := lstmSyncFile{}
	if err := json.Unmarshal(content, &payload); err != nil {
		return status, err
	}
	status.PredictionAsOf = strings.TrimSpace(payload.PredictionAsOf)
	status.ItemCount = payload.ItemCount
	if status.ItemCount < len(payload.Items) {
		status.ItemCount = len(payload.Items)
	}
	return status, nil
}

func newsWindowBounds(previousDate, targetDate string, loc *time.Location) (time.Time, time.Time) {
	if previousDate == "" || targetDate == "" {
		return time.Time{}, time.Time{}
	}
	start, err := tradingDateTime(previousDate, marketCloseHour, marketCloseMinute, loc)
	if err != nil {
		return time.Time{}, time.Time{}
	}
	end, err := tradingDateTime(targetDate, preMarketOpenHour, preMarketOpenMinute, loc)
	if err != nil {
		return time.Time{}, time.Time{}
	}
	return start, end
}

func beforePreMarketOpen(now time.Time) bool {
	hour, minute, _ := now.Clock()
	return hour < preMarketOpenHour || (hour == preMarketOpenHour && minute < preMarketOpenMinute)
}

func atOrAfterMarketClose(now time.Time) bool {
	hour, minute, _ := now.Clock()
	return hour > marketCloseHour || (hour == marketCloseHour && minute >= marketCloseMinute)
}

func indexAtOrBefore(values []string, target string) int {
	index := -1
	for i, value := range values {
		if value > target {
			break
		}
		index = i
	}
	return index
}

func seoulLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err == nil {
		return loc
	}
	return time.FixedZone("KST", 9*60*60)
}

func normalizeTradingDate(raw string) string {
	value := strings.TrimSpace(strings.ReplaceAll(raw, "-", ""))
	if len(value) != 8 {
		return ""
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return ""
		}
	}
	return value
}

func nextWeekdayTradingDate(date string, loc *time.Location) string {
	base, err := tradingDateTime(date, 0, 0, loc)
	if err != nil {
		return ""
	}
	for i := 1; i <= 7; i++ {
		candidate := base.AddDate(0, 0, i)
		if candidate.Weekday() == time.Saturday || candidate.Weekday() == time.Sunday {
			continue
		}
		return candidate.Format("20060102")
	}
	return ""
}

func tradingDateTime(date string, hour, minute int, loc *time.Location) (time.Time, error) {
	normalized := normalizeTradingDate(date)
	if normalized == "" {
		return time.Time{}, fmt.Errorf("잘못된 거래일 형식: %s", date)
	}
	return time.Date(
		mustAtoi(normalized[0:4]),
		time.Month(mustAtoi(normalized[4:6])),
		mustAtoi(normalized[6:8]),
		hour,
		minute,
		0,
		0,
		loc,
	), nil
}

func mustAtoi(raw string) int {
	value := 0
	for _, ch := range raw {
		value = value*10 + int(ch-'0')
	}
	return value
}

func truncateNote(note string) string {
	note = strings.TrimSpace(note)
	if len(note) <= 500 {
		return note
	}
	return note[:500]
}

func formatTimeOrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func toString(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(value)
	}
}
