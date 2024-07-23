package config

type MonitorConfig struct {
	RabbitMQURL          string `split_words:"true" required:"true"`
	RabbitMQConsumeQueue string `split_words:"true" required:"true"`
	RabbitMQPublishQueue string `split_words:"true" required:"true"`
	GitHubToken          string `split_words:"true" required:"true"`
	RedisAddr            string `split_words:"true" required:"true"`
}
