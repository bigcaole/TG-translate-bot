package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// UserSettings stores per-user runtime preferences.
type UserSettings struct {
	UserID         int64
	TargetLanguage string
	AutoMode       bool
	BotEnabled     bool
	UpdatedAt      time.Time
}

// Store wraps PostgreSQL access.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create pg pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping pg: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) InitSchema(ctx context.Context) error {
	const query = `
CREATE TABLE IF NOT EXISTS user_settings (
  user_id BIGINT PRIMARY KEY,
  target_language VARCHAR(8) NOT NULL DEFAULT 'en',
  auto_mode BOOLEAN NOT NULL DEFAULT TRUE,
  bot_enabled BOOLEAN NOT NULL DEFAULT TRUE,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`
	_, err := s.pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("init schema: %w", err)
	}
	return nil
}

func (s *Store) GetOrCreateUserSettings(ctx context.Context, userID int64, defaultTargetLang string) (*UserSettings, error) {
	const insert = `
INSERT INTO user_settings (user_id, target_language)
VALUES ($1, $2)
ON CONFLICT (user_id) DO NOTHING;
`
	_, err := s.pool.Exec(ctx, insert, userID, defaultTargetLang)
	if err != nil {
		return nil, fmt.Errorf("create user settings: %w", err)
	}

	const selectQuery = `
SELECT user_id, target_language, auto_mode, bot_enabled, updated_at
FROM user_settings
WHERE user_id = $1;
`
	var settings UserSettings
	err = s.pool.QueryRow(ctx, selectQuery, userID).
		Scan(&settings.UserID, &settings.TargetLanguage, &settings.AutoMode, &settings.BotEnabled, &settings.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user settings: %w", err)
	}
	return &settings, nil
}

func (s *Store) UpdateTargetLanguage(ctx context.Context, userID int64, lang string) error {
	const query = `
INSERT INTO user_settings (user_id, target_language, auto_mode, bot_enabled)
VALUES ($1, $2, FALSE, TRUE)
ON CONFLICT (user_id)
DO UPDATE SET target_language = EXCLUDED.target_language, auto_mode = FALSE, updated_at = NOW();
`
	_, err := s.pool.Exec(ctx, query, userID, lang)
	if err != nil {
		return fmt.Errorf("update target language: %w", err)
	}
	return nil
}

func (s *Store) SetAutoMode(ctx context.Context, userID int64, enabled bool, defaultTargetLang string) error {
	const query = `
INSERT INTO user_settings (user_id, target_language, auto_mode, bot_enabled)
VALUES ($1, $2, $3, TRUE)
ON CONFLICT (user_id)
DO UPDATE SET auto_mode = EXCLUDED.auto_mode, updated_at = NOW();
`
	_, err := s.pool.Exec(ctx, query, userID, defaultTargetLang, enabled)
	if err != nil {
		return fmt.Errorf("set auto mode: %w", err)
	}
	return nil
}

func (s *Store) SetBotEnabled(ctx context.Context, userID int64, enabled bool, defaultTargetLang string) error {
	const query = `
INSERT INTO user_settings (user_id, target_language, auto_mode, bot_enabled)
VALUES ($1, $2, TRUE, $3)
ON CONFLICT (user_id)
DO UPDATE SET bot_enabled = EXCLUDED.bot_enabled, updated_at = NOW();
`
	_, err := s.pool.Exec(ctx, query, userID, defaultTargetLang, enabled)
	if err != nil {
		return fmt.Errorf("set bot enabled: %w", err)
	}
	return nil
}

func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}
