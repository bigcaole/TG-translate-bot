package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	BotToken              string
	AllowedUsers          map[int64]struct{}
	AdminUsers            []int64
	PostgresDSN           string
	RedisAddr             string
	RedisPassword         string
	RedisDB               int
	GoogleProjectID       string
	GoogleLocation        string
	DefaultTargetLanguage string
	RequestTimeout        time.Duration
	CacheTTL              time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{
		BotToken:              strings.TrimSpace(os.Getenv("BOT_TOKEN")),
		PostgresDSN:           strings.TrimSpace(os.Getenv("POSTGRES_DSN")),
		RedisAddr:             strings.TrimSpace(os.Getenv("REDIS_ADDR")),
		RedisPassword:         os.Getenv("REDIS_PASSWORD"),
		GoogleProjectID:       strings.TrimSpace(firstNonEmpty(os.Getenv("GOOGLE_PROJECT_ID"), os.Getenv("GOOGLE_CLOUD_PROJECT"))),
		GoogleLocation:        strings.TrimSpace(firstNonEmpty(os.Getenv("GOOGLE_LOCATION"), "global")),
		DefaultTargetLanguage: strings.TrimSpace(firstNonEmpty(os.Getenv("DEFAULT_TARGET_LANGUAGE"), "en")),
	}

	requestTimeout, err := parseDurationEnv("REQUEST_TIMEOUT", "10s")
	if err != nil {
		return nil, err
	}
	cfg.RequestTimeout = requestTimeout

	cacheTTL, err := parseDurationEnv("CACHE_TTL", "720h")
	if err != nil {
		return nil, err
	}
	cfg.CacheTTL = cacheTTL

	if cfg.BotToken == "" {
		return nil, fmt.Errorf("BOT_TOKEN is required")
	}
	if cfg.PostgresDSN == "" {
		return nil, fmt.Errorf("POSTGRES_DSN is required")
	}
	if cfg.RedisAddr == "" {
		return nil, fmt.Errorf("REDIS_ADDR is required")
	}
	if cfg.GoogleProjectID == "" {
		return nil, fmt.Errorf("GOOGLE_PROJECT_ID or GOOGLE_CLOUD_PROJECT is required")
	}

	redisDB := strings.TrimSpace(firstNonEmpty(os.Getenv("REDIS_DB"), "0"))
	db, err := strconv.Atoi(redisDB)
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_DB: %w", err)
	}
	cfg.RedisDB = db

	allowedUsers, err := parseUsers(strings.TrimSpace(os.Getenv("ALLOWED_USERS")))
	if err != nil {
		return nil, fmt.Errorf("invalid ALLOWED_USERS: %w", err)
	}
	if len(allowedUsers) == 0 {
		return nil, fmt.Errorf("ALLOWED_USERS must contain at least one user id")
	}
	cfg.AllowedUsers = allowedUsers

	admins, err := parseUsersSlice(strings.TrimSpace(os.Getenv("ADMIN_USERS")))
	if err != nil {
		return nil, fmt.Errorf("invalid ADMIN_USERS: %w", err)
	}
	if len(admins) == 0 {
		for id := range cfg.AllowedUsers {
			admins = append(admins, id)
		}
	}
	cfg.AdminUsers = admins

	return cfg, nil
}

func parseUsers(value string) (map[int64]struct{}, error) {
	ids, err := parseUsersSlice(value)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		result[id] = struct{}{}
	}
	return result, nil
}

func parseUsersSlice(value string) ([]int64, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	ids := make([]int64, 0, len(parts))
	seen := make(map[int64]struct{}, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		id, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not a valid int64", trimmed)
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func parseDurationEnv(key, defaultValue string) (time.Duration, error) {
	value := strings.TrimSpace(firstNonEmpty(os.Getenv(key), defaultValue))
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return d, nil
}
