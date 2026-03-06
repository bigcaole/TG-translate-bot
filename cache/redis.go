package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps Redis operations used by bot services.
type Client struct {
	rdb *redis.Client
}

func NewClient(ctx context.Context, addr, password string, db int) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return &Client{rdb: rdb}, nil
}

func (c *Client) GetTranslation(ctx context.Context, key string) (string, bool, error) {
	val, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get translation cache: %w", err)
	}
	return val, true, nil
}

func (c *Client) SetTranslation(ctx context.Context, key, value string, ttl time.Duration) error {
	if err := c.rdb.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("set translation cache: %w", err)
	}
	return nil
}

func (c *Client) IncrMonthlyChars(ctx context.Context, month string, delta int64, ttl time.Duration) (int64, error) {
	key := c.monthlyCharsKey(month)
	pipe := c.rdb.TxPipeline()
	countCmd := pipe.IncrBy(ctx, key, delta)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("incr monthly chars: %w", err)
	}
	return countCmd.Val(), nil
}

func (c *Client) GetMonthlyChars(ctx context.Context, month string) (int64, error) {
	key := c.monthlyCharsKey(month)
	count, err := c.rdb.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get monthly chars: %w", err)
	}
	return count, nil
}

func (c *Client) SetMonthFlagNX(ctx context.Context, flag, month string, ttl time.Duration) (bool, error) {
	key := c.monthFlagKey(flag, month)
	ok, err := c.rdb.SetNX(ctx, key, "1", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("set month flag nx: %w", err)
	}
	return ok, nil
}

func (c *Client) HasMonthFlag(ctx context.Context, flag, month string) (bool, error) {
	key := c.monthFlagKey(flag, month)
	exists, err := c.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("has month flag: %w", err)
	}
	return exists > 0, nil
}

func (c *Client) monthlyCharsKey(month string) string {
	return fmt.Sprintf("quota:chars:%s", month)
}

func (c *Client) monthFlagKey(flag, month string) string {
	return fmt.Sprintf("quota:%s:%s", flag, month)
}

func (c *Client) Close() error {
	if c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}
