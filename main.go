package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"tg-translate-bot/bot"
	"tg-translate-bot/cache"
	"tg-translate-bot/config"
	"tg-translate-bot/database"
	"tg-translate-bot/quota"
	"tg-translate-bot/translator"
)

func main() {
	logger := log.New(os.Stdout, "[tg-translate-bot] ", log.LstdFlags|log.Lshortfile)

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("load config failed: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, redisClient, translatorClient, botAPI, err := bootstrap(ctx, cfg)
	if err != nil {
		logger.Fatalf("bootstrap failed: %v", err)
	}
	defer gracefulClose(logger, store, redisClient, translatorClient)

	quotaManager := quota.NewManager(redisClient)
	handler := bot.NewHandler(botAPI, cfg, store, redisClient, quotaManager, translatorClient, logger)

	errCh := make(chan error, 1)
	go func() {
		errCh <- handler.Run(ctx)
	}()

	logger.Println("机器人已启动，正在监听消息...")

	select {
	case <-ctx.Done():
		logger.Println("收到退出信号，正在关闭服务...")
	case runErr := <-errCh:
		if runErr != nil && !errors.Is(runErr, context.Canceled) {
			logger.Printf("bot run failed: %v", runErr)
		}
	}
}

func bootstrap(ctx context.Context, cfg *config.Config) (*database.Store, *cache.Client, *translator.Client, *tgbotapi.BotAPI, error) {
	initCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	store, err := database.NewStore(initCtx, cfg.PostgresDSN)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if err := store.InitSchema(initCtx); err != nil {
		store.Close()
		return nil, nil, nil, nil, err
	}

	redisClient, err := cache.NewClient(initCtx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		store.Close()
		return nil, nil, nil, nil, err
	}

	translatorClient, err := translator.NewClient(initCtx, cfg.GoogleProjectID, cfg.GoogleLocation)
	if err != nil {
		store.Close()
		_ = redisClient.Close()
		return nil, nil, nil, nil, err
	}

	botAPI, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		store.Close()
		_ = redisClient.Close()
		_ = translatorClient.Close()
		return nil, nil, nil, nil, err
	}

	return store, redisClient, translatorClient, botAPI, nil
}

func gracefulClose(logger *log.Logger, store *database.Store, redisClient *cache.Client, translatorClient *translator.Client) {
	if translatorClient != nil {
		if err := translatorClient.Close(); err != nil {
			logger.Printf("close translator failed: %v", err)
		}
	}
	if redisClient != nil {
		if err := redisClient.Close(); err != nil {
			logger.Printf("close redis failed: %v", err)
		}
	}
	if store != nil {
		store.Close()
	}
}
