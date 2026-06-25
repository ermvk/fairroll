// Consumer for register User

package consumer

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"

	"fairroll/pkg/logger"
	"fairroll/services/wallet/internal/service"
)

type UserRegisteredEvent struct {
	Email    string `json:"email"`
	UserName string `json:"user_name"`
}

type UserEventsConsumer struct {
	client        *kgo.Client
	walletService *service.WalletService
	logger        *zap.Logger
}

func NewUserEventsConsumer(client *kgo.Client, walletService *service.WalletService) *UserEventsConsumer {
	return &UserEventsConsumer{
		client:        client,
		walletService: walletService,
		logger:        logger.GetZap(),
	}
}

func (c *UserEventsConsumer) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}

		fetches := c.client.PollFetches(ctx)

		if fetches.IsClientClosed() {
			return
		}

		fetches.EachError(func(_ string, _ int32, err error) {
			c.logger.Error("kafka fetch error", zap.Error(err))
		})

		fetches.EachRecord(func(record *kgo.Record) {
			c.handleRecord(ctx, record)
		})
	}
}

func (c *UserEventsConsumer) handleRecord(ctx context.Context, record *kgo.Record) {
	eventType := headerValue(record, "event_type")

	if eventType != "user.registered" {
		return
	}

	var evt UserRegisteredEvent

	if err := json.Unmarshal(record.Value, &evt); err != nil {
		c.logger.Error("Failed to unmarshal user.registered payload", zap.Error(err))
		return
	}

	userID, err := uuid.Parse(string(record.Key))
	if err != nil {
		c.logger.Error("Invalid aggregate_id in record key", zap.Error(err), zap.String("key",
			string(record.Key)))
		return
	}

	if _, err := c.walletService.CreateAccountForUser(ctx, userID, "USD"); err != nil {
		c.logger.Error("Failed to create wallet account", zap.Error(err), zap.String("user_id",
			userID.String(),
		))
		return
	}

	c.logger.Info("Wallet account created from user.registered", zap.String("user_id", userID.String()))
}

func headerValue(record *kgo.Record, key string) string {
	for _, h := range record.Headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}
