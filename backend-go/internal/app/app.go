package app

import (
	"investment-news-go/internal/bithumb"
	"investment-news-go/internal/config"
	"investment-news-go/internal/dart"
	"investment-news-go/internal/krx"
	"investment-news-go/internal/news"
	"investment-news-go/internal/nxt"
	"investment-news-go/internal/ops"
	"investment-news-go/internal/quant"
	"investment-news-go/internal/telegram"
	"investment-news-go/internal/upbit"
)

type App struct {
	Config   config.Config
	News     *news.Service
	NXT      *nxt.Service
	KRX      *krx.Service
	Bithumb  *bithumb.Service
	Upbit    *upbit.Service
	Telegram *telegram.Service
	Quant    *quant.Service
	Dart     *dart.Service
	Ops      *ops.StateStore
	Sync     *ops.QuantSyncManager
}

func New(cfg config.Config) *App {
	application := &App{
		Config:   cfg,
		News:     news.NewService(cfg),
		NXT:      nxt.NewService(cfg),
		KRX:      krx.NewService(cfg),
		Bithumb:  bithumb.NewService(cfg),
		Upbit:    upbit.NewService(cfg),
		Telegram: telegram.NewService(cfg),
		Quant:    quant.NewService(cfg),
		Dart:     dart.NewService(cfg),
		Ops:      ops.NewStateStore(cfg.SyncStatePath),
	}
	application.Sync = ops.NewQuantSyncManager(cfg, application.Ops, application.KRX, application.News, application.NXT)
	return application
}
