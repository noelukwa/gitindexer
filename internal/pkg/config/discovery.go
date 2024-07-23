package config

import "time"

type DiscoveryConfig struct {
	RabbitMQURL          string        `split_words:"true" required:"true"`
	RedisURL             string        `split_words:"true" required:"true"`
	RabbitMQConsumeQueue string        `split_words:"true" required:"true"`
	RabbitMQPublishQueue string        `split_words:"true" required:"true"`
	BroadcastInterval    time.Duration `split_words:"true" required:"true"`
}
