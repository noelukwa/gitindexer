package main

type Config struct {
	RabbitMQURL          string `envconfig:"RABBITMQ_URL"`
	RabbitMQConsumeQueue string `envconfig:"RABBITMQ_CONSUME_QUEUE"`
	RabbitMQPublishQueue string `envconfig:"RABBITMQ_PUBLISH_QUEUE"`
	GitHubToken          string `envconfig:"GITHUB_TOKEN"`
	RedisAddr            string `envconfig:"REDIS_ADDR"`
}
