package consumer

import (
	"context"
	"encoding/json"
	"log"

	"github.com/shopspring/decimal"
	"github.com/twmb/franz-go/pkg/kgo"

	"fairroll/services/currency/internal/service"
)

type ConversionConsumer struct {
	kafkaConsumer   *kgo.Client
	kafkaProducer   *kgo.Client
	currencyService *service.CurrencyService
}

func NewConversionConsumer(
	consumer *kgo.Client,
	producer *kgo.Client,
	currencyService *service.CurrencyService,
) *ConversionConsumer {
	return &ConversionConsumer{
		kafkaConsumer:   consumer,
		kafkaProducer:   producer,
		currencyService: currencyService,
	}
}

func (cc *ConversionConsumer) Run(ctx context.Context) {
	log.Println("[Currency] Conversion consumer starting...")

	for {
		if ctx.Err() != nil {
			log.Println("[Currency] Conversion consumer stopping...")
			return
		}

		fetches := cc.kafkaConsumer.PollFetches(ctx)
		if fetches.IsClientClosed() {
			log.Println("[Currency] Kafka client closed")
			return
		}

		fetches.EachError(func(t string, p int32, err error) {
			log.Printf("[Currency] fetch error topic=%s partition=%d: %v", t, p, err)
		})

		fetches.EachRecord(func(r *kgo.Record) {
			cc.handleConversionRequest(ctx, string(r.Value))
		})
	}
}

func (cc *ConversionConsumer) handleConversionRequest(ctx context.Context, payload string) {
	var request struct {
		ConversionID string `json:"conversion_id"`
		FromCurrency string `json:"from_currency"`
		ToCurrency   string `json:"to_currency"`
		Amount       string `json:"amount"`
	}

	if err := json.Unmarshal([]byte(payload), &request); err != nil {
		log.Printf("[Currency] Failed to parse conversion request: %v", err)
		return
	}

	// Parse amount
	amount, err := decimal.NewFromString(request.Amount)
	if err != nil {
		cc.publishConversionFailed(ctx, request.ConversionID, "invalid amount format")
		return
	}

	convertedAmount, err := cc.currencyService.Convert(
		request.FromCurrency,
		request.ToCurrency,
		amount,
	)
	if err != nil {
		cc.publishConversionFailed(ctx, request.ConversionID, err.Error())
		return
	}

	cc.publishConversionCompleted(ctx, request.ConversionID, request.FromCurrency, request.ToCurrency, amount, convertedAmount)
}

func (cc *ConversionConsumer) publishConversionCompleted(
	ctx context.Context,
	conversionID string,
	fromCurrency string,
	toCurrency string,
	originalAmount decimal.Decimal,
	convertedAmount decimal.Decimal,
) {
	response := map[string]interface{}{
		"conversion_id":    conversionID,
		"from_currency":    fromCurrency,
		"to_currency":      toCurrency,
		"original_amount":  originalAmount.String(),
		"converted_amount": convertedAmount.String(),
		"status":           "completed",
	}

	payload, err := json.Marshal(response)
	if err != nil {
		log.Printf("[Currency] Failed to marshal conversion result: %v", err)
		return
	}

	cc.kafkaProducer.ProduceSync(
		ctx,
		&kgo.Record{
			Topic: "conversion.completed",
			Value: payload,
		},
	)

	log.Printf("[Currency] Conversion completed: %s %s → %s (%s)", originalAmount.String(), fromCurrency, toCurrency, convertedAmount.String())
}

func (cc *ConversionConsumer) publishConversionFailed(ctx context.Context, conversionID, reason string) {
	response := map[string]interface{}{
		"conversion_id": conversionID,
		"status":        "failed",
		"reason":        reason,
	}

	payload, err := json.Marshal(response)
	if err != nil {
		log.Printf("[Currency] Failed to marshal error response: %v", err)
		return
	}

	cc.kafkaProducer.ProduceSync(
		ctx,
		&kgo.Record{
			Topic: "conversion.completed",
			Value: payload,
		},
	)

	log.Printf("[Currency] Conversion failed: %s - %s", conversionID, reason)
}
