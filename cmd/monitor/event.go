package main

import (
	"encoding/json"

	"github.com/noelukwa/indexer/internal/events"
)

func parseEvent(data []byte) (*events.NewIntent, error) {
	var event events.NewIntent
	err := json.Unmarshal(data, &event)
	if err != nil {
		return nil, err
	}
	return &event, nil
}
