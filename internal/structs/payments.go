package structs

import (
	"time"
)

var (
	DefaultProcessor  string = "default"
	FallbackProcessor string = "fallback"
)

type PaymentsPayload struct {
	CorrelationID string    `json:"correlationId"`
	Amount        float64   `json:"amount"`
	RequestedAt   time.Time `json:"requestedAt"`
}

type ConsolidatePayment struct {
	Tag          string
	ProcessorURL string
	Payload      *PaymentsPayload
}

type ServiceHealthPayload struct {
	Failing         bool `json:"failing"`
	MinResponseTime int  `json:"minResponseTime"`
}
