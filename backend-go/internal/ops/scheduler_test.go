package ops

import (
	"testing"
	"time"

	"investment-news-go/internal/config"
)

func TestQuantAutoSchedulerNextRunSameDay(t *testing.T) {
	loc := seoulLocation()
	scheduler := NewQuantAutoScheduler(config.Config{
		AutoQuantSync:              true,
		AutoQuantSyncMidnightTime:  "00:00",
		AutoQuantSyncPreMarketTime: "07:50",
	}, nil)

	now := time.Date(2026, 3, 11, 7, 5, 0, 0, loc)
	nextRun, slot, err := scheduler.nextRun(now, loc)
	if err != nil {
		t.Fatalf("expected next run, got error: %v", err)
	}
	if slot != "pre_market" {
		t.Fatalf("expected pre_market slot, got %s", slot)
	}
	if nextRun.Format("20060102 15:04") != "20260311 07:50" {
		t.Fatalf("unexpected next run: %s", nextRun.Format("20060102 15:04"))
	}
}

func TestQuantAutoSchedulerNextRunSkipsWeekend(t *testing.T) {
	loc := seoulLocation()
	scheduler := NewQuantAutoScheduler(config.Config{
		AutoQuantSync:              true,
		AutoQuantSyncMidnightTime:  "00:00",
		AutoQuantSyncPreMarketTime: "07:50",
	}, nil)

	now := time.Date(2026, 3, 13, 20, 20, 0, 0, loc)
	nextRun, slot, err := scheduler.nextRun(now, loc)
	if err != nil {
		t.Fatalf("expected next run, got error: %v", err)
	}
	if slot != "midnight" {
		t.Fatalf("expected next slot to be Monday midnight, got %s", slot)
	}
	if nextRun.Format("20060102 15:04") != "20260316 00:00" {
		t.Fatalf("unexpected next run: %s", nextRun.Format("20060102 15:04"))
	}
}
