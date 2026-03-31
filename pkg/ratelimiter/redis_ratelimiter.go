package ratelimiter

import (
	"context"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

type RedisRateLimiter struct {
	client     *redis.Client
	ipLimit    int
	tokenLimit int
	window     time.Duration
	blockTime  time.Duration
	context    context.Context
}

func NewRedisLimiter(client *redis.Client, ipLimit, tokenLimit int, window, block time.Duration) *RedisRateLimiter {
	return &RedisRateLimiter{
		client:     client,
		ipLimit:    ipLimit,
		tokenLimit: tokenLimit,
		window:     window,
		blockTime:  block,
		context:    context.Background(),
	}

}

func (rl *RedisRateLimiter) Allow(key, kind string) bool {
	var limit int
	switch kind {
	case "token":
		limit = rl.tokenLimit
	default:
		limit = rl.ipLimit
	}

	blockKey := "blocked:" + key

	blocked, err := rl.client.Exists(rl.context, blockKey).Result()
	if err != nil {
		log.Print("erro ao verificar bloqueio:", err)
		return false
	}
	if blocked > 0 {
		return false
	}

	pipe := rl.client.TxPipeline()
	icmd := pipe.Incr(rl.context, key)
	pipe.Expire(rl.context, key, rl.window)
	_, err = pipe.Exec(rl.context)
	if err != nil {
		log.Print("erro no Allow:", err)
		return false
	}

	count := icmd.Val()
	if count > int64(limit) {
		err := rl.client.Set(rl.context, blockKey, "1", rl.blockTime).Err()
		if err != nil {
			log.Print("erro ao definir bloqueio:", err)
		}
		return false
	}

	return true
}