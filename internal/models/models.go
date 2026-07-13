package models

import "time"

type DestinationType string

const (
	DestinationDiscord DestinationType = "discord"
	DestinationSlack   DestinationType = "slack"
	DestinationGeneric DestinationType = "generic"
)

type Destination struct {
	Type   DestinationType `json:"type"`
	URL    string          `json:"url"`
	Secret *string         `json:"secret,omitempty"`
}

type DispatchRequest struct {
	Payload      	map[string]any 	`json:"payload"`
	Destinations 	[]Destination  	`json:"destinations"`
	Secret      	*string        	`json:"secret,omitempty"`
	EventType   	*string        	`json:"event_type,omitempty"`
}

type AttemptStatus string

const (
	StatusSuccess AttemptStatus = "success"
	StatusFailure AttemptStatus = "failure"
	StatusPending AttemptStatus = "pending"
)

type DeliveryAttempt struct {
	AttemptNumber int           	`json:"attempt_number"`
	Timestamp     time.Time     	`json:"timestamp"`
	Status        AttemptStatus 	`json:"status"`
	HTTPStatus    *int          	`json:"http_status,omitempty"`
	Error         *string       	`json:"error,omitempty"`
}

type DestinationResult struct {
	DestinationType DestinationType   `json:"destination_type"`
	URL             string            `json:"url"`
	Status          AttemptStatus     `json:"status"`
	Attempts        []DeliveryAttempt `json:"attempts"`
}

type DispatchResponse struct {
	EventID string              `json:"event_id"`
	Results []DestinationResult `json:"results"`
}

type EventRecord struct {
	EventID   string              `json:"event_id"`
	EventType *string             `json:"event_type,omitempty"`
	Payload   map[string]any      `json:"payload"`
	CreatedAt time.Time           `json:"created_at"`
	Results   []DestinationResult `json:"results"`
}