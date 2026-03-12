package ops

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"investment-news-go/internal/config"
)

const checkpointQuantScheduler = "quant_scheduler"

type QuantAutoScheduler struct {
	cfg     config.Config
	manager *QuantSyncManager
}

func NewQuantAutoScheduler(cfg config.Config, manager *QuantSyncManager) *QuantAutoScheduler {
	return &QuantAutoScheduler{
		cfg:     cfg,
		manager: manager,
	}
}

func (s *QuantAutoScheduler) Start(ctx context.Context) {
	if s == nil || s.manager == nil || !s.cfg.AutoQuantSync {
		return
	}

	loc := seoulLocation()
	log.Printf("quant scheduler: enabled (midnight %s KST, pre-market %s KST)", s.cfg.AutoQuantSyncMidnightTime, s.cfg.AutoQuantSyncPreMarketTime)
	if s.cfg.AutoQuantSyncStartup {
		go s.runOnce(ctx, "startup", time.Now().In(loc))
	}

	for {
		nextRun, slot, err := s.nextRun(time.Now().In(loc), loc)
		if err != nil {
			log.Printf("quant scheduler: next run calculation failed: %v", err)
			timer := time.NewTimer(time.Minute)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			continue
		}

		wait := time.Until(nextRun)
		if wait < 0 {
			wait = 0
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			s.runOnce(ctx, slot, nextRun)
		}
	}
}

func (s *QuantAutoScheduler) runOnce(ctx context.Context, slot string, scheduledAt time.Time) {
	if s == nil || s.manager == nil {
		return
	}

	now := time.Now().In(seoulLocation())
	_ = s.manager.updateCheckpoint(checkpointQuantScheduler, func(cp *SyncCheckpoint) {
		cp.LastAttemptAt = now.Format(time.RFC3339)
		cp.Status = "running"
		cp.Note = fmt.Sprintf("scheduled %s sync started", slot)
		cp.Extra["slot"] = slot
		cp.Extra["scheduled_at"] = scheduledAt.Format(time.RFC3339)
	})

	runSlot := slot
	if runSlot == "startup" {
		runSlot = ""
	}
	snapshot, err := s.manager.EnsureQuantInputsForSlot(ctx, runSlot)
	if err != nil {
		note := err.Error()
		_ = s.manager.updateCheckpoint(checkpointQuantScheduler, func(cp *SyncCheckpoint) {
			cp.Status = "warning"
			cp.Note = truncateNote(note)
			cp.LastTradingDate = firstNonEmpty(snapshot.ModelTargetTradingDate, snapshot.NewsTargetTradingDate, snapshot.KRXCollectDate)
			cp.LastAsOf = snapshot.ModelAsOf
			cp.Extra["slot"] = slot
			cp.Extra["session"] = snapshot.Session
			cp.Extra["performed"] = strings.Join(snapshot.Performed, ",")
			cp.Extra["skipped"] = strings.Join(snapshot.Skipped, ",")
		})
		log.Printf("quant scheduler: %s sync finished with warning: %v", slot, err)
		return
	}

	_ = s.manager.updateCheckpoint(checkpointQuantScheduler, func(cp *SyncCheckpoint) {
		cp.LastSuccessAt = time.Now().Format(time.RFC3339)
		cp.LastTradingDate = firstNonEmpty(snapshot.ModelTargetTradingDate, snapshot.NewsTargetTradingDate, snapshot.KRXCollectDate)
		cp.LastAsOf = snapshot.ModelAsOf
		cp.Status = "success"
		cp.Note = fmt.Sprintf("scheduled %s sync completed", slot)
		cp.Extra["slot"] = slot
		cp.Extra["session"] = snapshot.Session
		cp.Extra["performed"] = strings.Join(snapshot.Performed, ",")
		cp.Extra["skipped"] = strings.Join(snapshot.Skipped, ",")
	})
	log.Printf("quant scheduler: %s sync completed", slot)
}

func (s *QuantAutoScheduler) nextRun(now time.Time, loc *time.Location) (time.Time, string, error) {
	candidates := []struct {
		slot string
		when time.Time
	}{}

	for dayOffset := 0; dayOffset <= 7; dayOffset++ {
		day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, dayOffset)
		if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
			continue
		}

		midnight, err := combineClock(day, s.cfg.AutoQuantSyncMidnightTime, loc)
		if err != nil {
			return time.Time{}, "", err
		}
		preMarket, err := combineClock(day, s.cfg.AutoQuantSyncPreMarketTime, loc)
		if err != nil {
			return time.Time{}, "", err
		}

		candidates = append(candidates,
			struct {
				slot string
				when time.Time
			}{slot: scheduledSlotMidnight, when: midnight},
			struct {
				slot string
				when time.Time
			}{slot: scheduledSlotPreMarket, when: preMarket},
		)
	}

	for _, candidate := range candidates {
		if candidate.when.After(now) {
			return candidate.when, candidate.slot, nil
		}
	}

	return time.Time{}, "", fmt.Errorf("next scheduler run could not be resolved")
}

func combineClock(day time.Time, hhmm string, loc *time.Location) (time.Time, error) {
	parsed, err := time.ParseInLocation("15:04", strings.TrimSpace(hhmm), loc)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(day.Year(), day.Month(), day.Day(), parsed.Hour(), parsed.Minute(), 0, 0, loc), nil
}
