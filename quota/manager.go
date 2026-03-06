package quota

import (
	"context"
	"time"

	"tg-translate-bot/cache"
)

const (
	FreeQuota        int64 = 500000
	WarnThreshold    int64 = 400000
	CircuitThreshold int64 = 450000
)

// Manager handles monthly quota accounting and circuit breaking.
type Manager struct {
	cache *cache.Client
}

func NewManager(cacheClient *cache.Client) *Manager {
	return &Manager{cache: cacheClient}
}

func (m *Manager) Consume(ctx context.Context, chars int64, now time.Time) (usage int64, warnTriggered bool, circuitTriggered bool, circuitOpen bool, err error) {
	month := monthKey(now)
	ttl := monthTTL(now)

	usage, err = m.cache.IncrMonthlyChars(ctx, month, chars, ttl)
	if err != nil {
		return 0, false, false, false, err
	}

	if usage >= WarnThreshold {
		warnTriggered, err = m.cache.SetMonthFlagNX(ctx, "warned", month, ttl)
		if err != nil {
			return usage, false, false, false, err
		}
	}

	if usage >= CircuitThreshold {
		circuitTriggered, err = m.cache.SetMonthFlagNX(ctx, "service_off", month, ttl)
		if err != nil {
			return usage, warnTriggered, false, false, err
		}
	}

	return usage, warnTriggered, circuitTriggered, usage >= CircuitThreshold, nil
}

func (m *Manager) Usage(ctx context.Context, now time.Time) (int64, error) {
	return m.cache.GetMonthlyChars(ctx, monthKey(now))
}

func (m *Manager) IsCircuitOpen(ctx context.Context, now time.Time) (bool, error) {
	return m.cache.HasMonthFlag(ctx, "service_off", monthKey(now))
}

func monthKey(now time.Time) string {
	return now.Format("200601")
}

func monthTTL(now time.Time) time.Duration {
	location := now.Location()
	firstOfCurrentMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location)
	firstOfNextMonth := firstOfCurrentMonth.AddDate(0, 1, 0)
	ttl := time.Until(firstOfNextMonth) + time.Hour
	if ttl <= 0 {
		return 24 * time.Hour
	}
	return ttl
}
