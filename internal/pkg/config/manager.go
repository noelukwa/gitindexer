package config

type ManagerConfig struct {
	DatabaseURL      string `split_words:"true" required:"true"`
	RabbitMQURL      string `split_words:"true" required:"true"`
	IntentsQueueName string `split_words:"true" required:"true"`
	CommitsQueueName string `split_words:"true" required:"true"`
	ServerPort       int    `split_words:"true" required:"true"`
}
