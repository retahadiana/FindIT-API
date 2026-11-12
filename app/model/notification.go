package model

import (
	"encoding/json"
	"time"
)

const (
	QueueCritical = "critical"
	QueueDefault  = "default"
	TaskEmailSend = "email:deliver"

	EventReputationRewarded = "reputation.rewarded"
	EventClaimSubmitted     = "claim.submitted"
	EventClaimDecided       = "claim.decided"
	EventItemReturned       = "item.returned"
)

type OutboxMessage struct {
	ID          int64
	EventType   string
	Payload     json.RawMessage
	CreatedAt   time.Time
	PublishedAt *time.Time
}

type ReputationRewardedEvent struct {
	UserID   int    `json:"user_id"`
	ClaimID  int    `json:"claim_id"`
	ItemID   int    `json:"item_id"`
	Points   int    `json:"points"`
	Reason   string `json:"reason"`
	Occurred string `json:"occurred_at"`
}

type ClaimEvent struct {
	ClaimID    int    `json:"claim_id"`
	ItemID     int    `json:"item_id"`
	ItemTitle  string `json:"item_title"`
	FinderID   int    `json:"finder_id"`
	ClaimantID int    `json:"claimant_id"`
	Status     string `json:"status"`
}

type EmailDeliveryPayload struct {
	OutboxID  int64  `json:"outbox_id"`
	EventType string `json:"event_type"`
	UserID    int    `json:"user_id"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
}

type Notification struct {
	ID       int64     `json:"id"`
	OutboxID int64     `json:"outbox_id"`
	UserID   int       `json:"user_id"`
	Channel  string    `json:"channel"`
	Subject  string    `json:"subject"`
	Body     string    `json:"body"`
	SentAt   time.Time `json:"sent_at"`
}

type OutboxStats struct {
	Pending   int64 `json:"pending"`
	Published int64 `json:"published"`
}

type QueueStats struct {
	Queue     string `json:"queue"`
	Pending   int    `json:"pending"`
	Active    int    `json:"active"`
	Scheduled int    `json:"scheduled"`
	Retry     int    `json:"retry"`
	Archived  int    `json:"archived"`
}

type QueueMonitorResponse struct {
	Outbox        OutboxStats  `json:"outbox"`
	Queues        []QueueStats `json:"queues"`
	Notifications int64        `json:"notifications_sent"`
}
