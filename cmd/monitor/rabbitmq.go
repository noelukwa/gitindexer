package main

import (
	"encoding/json"

	"github.com/google/go-github/v63/github"
	amqp "github.com/rabbitmq/amqp091-go"
)

func publishCommit(ch *amqp.Channel, queueName string, commit *github.RepositoryCommit) error {
	body, err := json.Marshal(commit)
	if err != nil {
		return err
	}

	err = ch.Publish(
		"",        // exchange
		queueName, // routing key
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})
	return err
}
