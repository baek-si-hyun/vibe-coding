package app

import (
	"investment-news-go/internal/bithumb"
	"investment-news-go/internal/config"
	"investment-news-go/internal/dart"
	"investment-news-go/internal/krx"
	"investment-news-go/internal/news"
	"investment-news-go/internal/quant"
	"investment-news-go/internal/telegram"
)

type App struct {
	Config   config.Config
	News     *news.Service
	KRX      *krx.Service
	Bithumb  *bithumb.Service
	Telegram *telegram.Service
	Quant    *quant.Service
	Dart     *dart.Service
}

func New(cfg config.Config) *App {
	return &App{
		Config:   cfg,
		News:     news.NewService(cfg),
		KRX:      krx.NewService(cfg),
		Bithumb:  bithumb.NewService(cfg),
		Telegram: telegram.NewService(cfg),
		Quant:    quant.NewService(cfg),
		Dart:     dart.NewService(cfg),
	}
}
