package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type EventPublisher struct {
	db *pgx.Conn
}

func NewEventPublisher(db *pgx.Conn) *EventPublisher {
	return &EventPublisher{db: db}
}

func (ep *EventPublisher) PublishEvent(
	ctx context.Context,
	eventType string,
	aggregateID uuid.UUID,
	payload []byte,
) error {
	_, err := ep.db.Exec(ctx,
		`INSERT INTO outbox_events (aggregate_id, event_type, payload)
         VALUES ($1, $2, $3)`,
		aggregateID, eventType, payload,
	)
	return err
}
