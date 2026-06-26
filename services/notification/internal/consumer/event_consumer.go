package consumer

import (
	"context"
	"encoding/json"
	"log"

	"github.com/twmb/franz-go/pkg/kgo"
)

type EventConsumer struct {
	kafkaClient *kgo.Client
}

func NewEventConsumer(kafkaClient *kgo.Client) *EventConsumer {
	return &EventConsumer{
		kafkaClient: kafkaClient,
	}
}

func (ec *EventConsumer) Run(ctx context.Context) {
	log.Println("[Notification] Event consumer starting...")

	for {
		select {
		case <-ctx.Done():
			log.Println("[Notification] Event consumer stopping...")
			return
		default:
		}

		fetches := ec.kafkaClient.PollFetches(ctx)

		if fetches.IsClientClosed() {
			log.Println("[Notification] Kafka client closed")
			return
		}

		fetches.EachPartition(func(p kgo.FetchTopicPartition) {
			for _, record := range p.Records {
				ec.handleEvent(ctx, string(record.Topic), string(record.Value))
			}
		})
	}
}

func (ec *EventConsumer) handleEvent(ctx context.Context, topic, payload string) {
	switch topic {
	case "user.events":
		ec.handleUserEvent(payload)
	case "transfer.events", "wallet.events":
		ec.handleTransferEvent(payload)
	case "deposit.completed", "withdrawal.completed":
		ec.handlePaymentEvent(payload)
	default:
		log.Printf("[Notification] Unknown topic: %s", topic)
	}
}

func (ec *EventConsumer) handleUserEvent(payload string) {
	var event map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		log.Printf("[Notification] Failed to parse user event: %v", err)
		return
	}

	email, _ := event["email"].(string)
	userName, _ := event["user_name"].(string)

	log.Printf("[NOTIFICATION] Welcome email sent to %s (%s)", email, userName)
}

func (ec *EventConsumer) handleTransferEvent(payload string) {
	var event map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		log.Printf("[Notification] Failed to parse transfer event: %v", err)
		return
	}

	fromUserID, _ := event["from_user_id"].(string)
	toUserID, _ := event["to_user_id"].(string)
	amount, _ := event["amount"].(string)
	currency, _ := event["currency"].(string)

	log.Printf("[NOTIFICATION] Transfer of %s %s completed: %s → %s", amount, currency, fromUserID[:8], toUserID[:8])
}

func (ec *EventConsumer) handlePaymentEvent(payload string) {
	var event map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		log.Printf("[Notification] Failed to parse payment event: %v", err)
		return
	}

	userID, _ := event["user_id"].(string)
	amount, _ := event["amount"].(string)
	currency, _ := event["currency"].(string)
	paymentType, _ := event["type"].(string)
	log.Printf("[NOTIFICATION] Payment %s confirmed for %s: %s %s", paymentType, userID[:8], amount, currency)
}
