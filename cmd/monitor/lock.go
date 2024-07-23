package main

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

func acquireLock(client *redis.Client, key string, ttl time.Duration) (bool, error) {
	ok, err := client.SetNX(context.Background(), key, "locked", ttl).Result()
	return ok, err
}

func releaseLock(client *redis.Client, key string) error {
	_, err := client.Del(context.Background(), key).Result()
	return err
}
