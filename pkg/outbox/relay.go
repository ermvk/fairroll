package outbox

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"

	"fairroll/pkg/logger"
)

type Event struct {
	ID          string
	AggregateID string
	EventType   string
	Payload     []byte
}

type Relay struct {
	db       *pgx.Conn
	producer *kgo.Client
	topic    string
	interval time.Duration
	batch    int
	logger   *zap.Logger
}

func NewRelay(db *pgx.Conn, producer *kgo.Client, topic string) *Relay {
	return &Relay{
		db:       db,
		producer: producer,
		topic:    topic,
		interval: time.Second,
		batch:    50,
		logger:   logger.GetZap(),
	}
}

func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.tick(ctx); err != nil {
				r.logger.Error("outbox relay tick failed", zap.Error(err))
			}
		}
	}
}

func (r *Relay) tick(ctx context.Context) error {
	events, err := r.fetchPending(ctx)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}

	for _, e := range events {
		if err := r.publish(ctx, e); err != nil {
			r.logger.Error("failed to publish outbox event", zap.Error(err), zap.String("event_id", e.ID))
			continue
		}
		if err := r.markSent(ctx, e.ID); err != nil {
			r.logger.Error("failed to mark outbox event as sent", zap.Error(err), zap.String("event_id", e.ID))
		}
	}

	return nil
}

func (r *Relay) fetchPending(ctx context.Context) ([]Event, error) {
	query := `SELECT id, aggregate_id, event_type, payload
		FROM outbox_events
		WHERE status = 'pending'
		ORDER BY created_at
		LIMIT $1`

	rows, err := r.db.Query(ctx, query, r.batch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.AggregateID, &e.EventType, &e.Payload); err != nil {
			return nil, err
		}
		events = append(events, e)
	}

	return events, rows.Err()
}

func (r *Relay) publish(ctx context.Context, e Event) error {
	record := &kgo.Record{
		Topic: r.topic,
		Key:   []byte(e.AggregateID),
		Value: e.Payload,
		Headers: []kgo.RecordHeader{
			{Key: "event_type", Value: []byte(e.EventType)},
		},
	}

	result := r.producer.ProduceSync(ctx, record)
	return result.FirstErr()
}

func (r *Relay) markSent(ctx context.Context, eventID string) error {
	query := `UPDATE outbox_events SET status = 'sent', sent_at = now() WHERE id = $1`
	_, err := r.db.Exec(ctx, query, eventID)
	return err
}
