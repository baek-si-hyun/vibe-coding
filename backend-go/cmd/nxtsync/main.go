package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"investment-news-go/internal/config"
	"investment-news-go/internal/nxt"
)

func main() {
	cfg := config.Load()
	service := nxt.NewService(cfg)

	tradingDate := ""
	if len(os.Args) > 1 {
		tradingDate = strings.TrimSpace(os.Args[1])
	}

	snapshot, err := service.CollectAndSave(tradingDate)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf(
		"collected trading_date=%s quotes=%d set_time=%s latest=%s\n",
		snapshot.TradingDate,
		snapshot.QuoteCount,
		snapshot.SetTime,
		service.LatestSnapshotPath(),
	)
}
