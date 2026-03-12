package ops

import (
	"testing"
	"time"
)

func TestBuildQuantSyncPlanPreMarket(t *testing.T) {
	loc := seoulLocation()
	now := time.Date(2026, 3, 11, 7, 10, 0, 0, loc)
	dates := []string{"20260309", "20260310", "20260311", "20260312"}

	plan := buildQuantSyncPlan(dates, "20260310", now, "")

	if plan.Session != "pre_market" {
		t.Fatalf("expected pre_market session, got %s", plan.Session)
	}
	if plan.KRXCollectDate != "20260310" {
		t.Fatalf("expected KRX collect date 20260310, got %s", plan.KRXCollectDate)
	}
	if plan.NewsTargetTradingDate != "20260311" {
		t.Fatalf("expected news target 20260311, got %s", plan.NewsTargetTradingDate)
	}
	if plan.ModelTargetTradingDate != "20260311" || plan.ModelAsOf != "20260310" {
		t.Fatalf("unexpected model sync target/as_of: %s / %s", plan.ModelTargetTradingDate, plan.ModelAsOf)
	}
	if !plan.ShouldEnsurePreMarket || !plan.ShouldEnsureModel || !plan.ShouldEnsureKRX || !plan.ShouldEnsureNXT {
		t.Fatalf("expected pre-market, model, KRX, and NXT sync to be required")
	}
}

func TestBuildQuantSyncPlanPostClose(t *testing.T) {
	loc := seoulLocation()
	now := time.Date(2026, 3, 11, 20, 5, 0, 0, loc)
	dates := []string{"20260309", "20260310", "20260311", "20260312"}

	plan := buildQuantSyncPlan(dates, "20260310", now, "")

	if plan.Session != "post_close" {
		t.Fatalf("expected post_close session, got %s", plan.Session)
	}
	if plan.KRXCollectDate != "20260311" {
		t.Fatalf("expected KRX collect date 20260311, got %s", plan.KRXCollectDate)
	}
	if plan.NewsTargetTradingDate != "20260312" {
		t.Fatalf("expected news target 20260312, got %s", plan.NewsTargetTradingDate)
	}
	if !plan.ShouldEnsureMidnight || !plan.ShouldEnsureNXT || plan.ShouldEnsureModel {
		t.Fatalf("expected post-close news and NXT sync to be required")
	}
}

func TestBuildQuantSyncPlanMidnightScheduled(t *testing.T) {
	loc := seoulLocation()
	now := time.Date(2026, 3, 11, 0, 0, 0, 0, loc)
	dates := []string{"20260309", "20260310", "20260311", "20260312"}

	plan := buildQuantSyncPlan(dates, "20260310", now, "midnight")

	if plan.Session != "midnight" {
		t.Fatalf("expected midnight session, got %s", plan.Session)
	}
	if plan.KRXCollectDate != "20260310" || plan.NXTTargetTradingDate != "20260310" {
		t.Fatalf("expected previous trading date inputs, got %s / %s", plan.KRXCollectDate, plan.NXTTargetTradingDate)
	}
	if plan.NewsTargetTradingDate != "20260311" {
		t.Fatalf("expected news target 20260311, got %s", plan.NewsTargetTradingDate)
	}
	if !plan.ShouldEnsureMidnight || !plan.ShouldEnsureModel {
		t.Fatalf("expected midnight plan to refresh news/model")
	}
}

func TestBuildQuantSyncPlanRegularDuringNXTSession(t *testing.T) {
	loc := seoulLocation()
	now := time.Date(2026, 3, 11, 18, 0, 0, 0, loc)
	dates := []string{"20260309", "20260310", "20260311", "20260312"}

	plan := buildQuantSyncPlan(dates, "20260310", now, "")

	if plan.Session != "regular" {
		t.Fatalf("expected regular session during NXT trading hours, got %s", plan.Session)
	}
	if !plan.ShouldEnsurePreMarket || !plan.ShouldEnsureModel || !plan.ShouldEnsureKRX || !plan.ShouldEnsureNXT {
		t.Fatalf("expected regular-session plan to keep current-day model inputs active")
	}
	if plan.ShouldEnsureMidnight {
		t.Fatalf("did not expect post-close sync during NXT trading hours")
	}
}

func TestBuildQuantSyncPlanWeekendClosed(t *testing.T) {
	loc := seoulLocation()
	now := time.Date(2026, 3, 14, 12, 0, 0, 0, loc)
	dates := []string{"20260312", "20260313", "20260316", "20260317"}

	plan := buildQuantSyncPlan(dates, "20260313", now, "")

	if plan.Session != "closed" {
		t.Fatalf("expected closed session, got %s", plan.Session)
	}
	if plan.KRXCollectDate != "20260313" {
		t.Fatalf("expected KRX collect date 20260313, got %s", plan.KRXCollectDate)
	}
	if plan.NewsTargetTradingDate != "20260316" {
		t.Fatalf("expected Monday news target, got %s", plan.NewsTargetTradingDate)
	}
	if !plan.ShouldEnsureMidnight || !plan.ShouldEnsureKRX || !plan.ShouldEnsureNXT {
		t.Fatalf("expected weekend recovery sync to be required")
	}
}
