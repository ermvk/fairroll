package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "fairroll/services/currency/api"
	"fairroll/services/transfer/internal/repository"
)

var (
	ErrInvalidAmount        = errors.New("amount must be positive")
	ErrInsufficientFunds    = errors.New("insufficient funds")
	ErrIdempotencyConflict  = errors.New("transfer already processed")
	ErrSameSender           = errors.New("sender and receiver must be different")
	ErrCurrencyNotSupported = errors.New("currency not supported")
	ErrCurrencyServiceError = errors.New("currency service error")
)

type TransferService struct {
	repo                  *repository.TransferRepository
	currencyServiceClient pb.CurrencyServiceClient
	currencyServiceConn   *grpc.ClientConn
	eventPublisher        *EventPublisher
}

func NewTransferService(repo *repository.TransferRepository, publisher *EventPublisher, currencyServiceAddr string) (*TransferService, error) {
	conn, err := grpc.NewClient(
		currencyServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	client := pb.NewCurrencyServiceClient(conn)

	return &TransferService{
		repo:                  repo,
		currencyServiceClient: client,
		currencyServiceConn:   conn,
		eventPublisher:        publisher,
	}, nil
}

func (s *TransferService) Close() error {
	if s.currencyServiceConn != nil {
		return s.currencyServiceConn.Close()
	}
	return nil
}

type SendTransferRequest struct {
	FromUserID     uuid.UUID
	ToUserID       uuid.UUID
	Amount         decimal.Decimal
	Currency       string
	IdempotencyKey string
}

func (s *TransferService) SendTransfer(ctx context.Context, req SendTransferRequest) (uuid.UUID, bool, error) {
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return uuid.Nil, false, ErrInvalidAmount
	}
	if req.FromUserID == req.ToUserID {
		return uuid.Nil, false, ErrSameSender
	}

	existingTransfer, err := s.repo.GetTransferByIdempotencyKey(ctx, req.IdempotencyKey)
	if err == nil {
		return existingTransfer.ID, true, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return uuid.Nil, false, err
	}

	senderBalance, err := s.repo.GetBalance(ctx, req.FromUserID, req.Currency)
	if err != nil {
		return uuid.Nil, false, err
	}
	if senderBalance.LessThan(req.Amount) {
		return uuid.Nil, false, ErrInsufficientFunds
	}

	transfer := &repository.Transfer{
		ID:             uuid.New(),
		FromUserID:     req.FromUserID,
		ToUserID:       req.ToUserID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Status:         "completed",
		Type:           "p2p",
		IdempotencyKey: req.IdempotencyKey,
		CreatedAt:      time.Now(),
	}

	if err := s.repo.CreateTransfer(ctx, transfer); err != nil {
		return uuid.Nil, false, err
	}

	payload := map[string]interface{}{
		"transfer_id":  transfer.ID,
		"from_user_id": transfer.FromUserID,
		"to_user_id":   transfer.ToUserID,
		"amount":       transfer.Amount.String(),
		"currency":     transfer.Currency,
	}
	payloadBytes, _ := json.Marshal(payload)
	s.eventPublisher.PublishEvent(ctx, "transfer.completed", transfer.ID, payloadBytes)

	return transfer.ID, false, nil
}

type InitiateDepositRequest struct {
	UserID         uuid.UUID
	Amount         decimal.Decimal
	Currency       string
	Source         string
	IdempotencyKey string
}

func (s *TransferService) InitiateDeposit(ctx context.Context, req InitiateDepositRequest) (uuid.UUID, bool, error) {
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return uuid.Nil, false, ErrInvalidAmount
	}

	existingPayment, err := s.repo.GetPaymentByIdempotencyKey(ctx, req.IdempotencyKey)
	if err == nil {
		return existingPayment.ID, true, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return uuid.Nil, false, err
	}

	payment := &repository.Payment{
		ID:             uuid.New(),
		UserID:         req.UserID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Type:           "deposit",
		Status:         "completed",
		Source:         req.Source,
		IdempotencyKey: req.IdempotencyKey,
		CreatedAt:      time.Now(),
	}

	if err := s.repo.CreatePayment(ctx, payment); err != nil {
		return uuid.Nil, false, err
	}

	payload := map[string]interface{}{
		"payment_id": payment.ID,
		"user_id":    payment.UserID,
		"amount":     payment.Amount.String(),
		"currency":   payment.Currency,
		"type":       "deposit",
	}
	payloadBytes, _ := json.Marshal(payload)
	s.eventPublisher.PublishEvent(ctx, "deposit.completed", payment.ID, payloadBytes)

	return payment.ID, false, nil
}

type InitiateWithdrawalRequest struct {
	UserID         uuid.UUID
	Amount         decimal.Decimal
	Currency       string
	Destination    string
	IdempotencyKey string
}

func (s *TransferService) InitiateWithdrawal(ctx context.Context, req InitiateWithdrawalRequest) (uuid.UUID, bool, error) {
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return uuid.Nil, false, ErrInvalidAmount
	}

	existingPayment, err := s.repo.GetPaymentByIdempotencyKey(ctx, req.IdempotencyKey)
	if err == nil {
		return existingPayment.ID, true, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return uuid.Nil, false, err
	}

	payment := &repository.Payment{
		ID:             uuid.New(),
		UserID:         req.UserID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Type:           "withdrawal",
		Status:         "completed",
		Destination:    req.Destination,
		IdempotencyKey: req.IdempotencyKey,
		CreatedAt:      time.Now(),
	}

	if err := s.repo.CreatePayment(ctx, payment); err != nil {
		return uuid.Nil, false, err
	}

	payload := map[string]interface{}{
		"payment_id": payment.ID,
		"user_id":    payment.UserID,
		"amount":     payment.Amount.String(),
		"currency":   payment.Currency,
		"type":       "withdrawal",
	}
	payloadBytes, _ := json.Marshal(payload)
	s.eventPublisher.PublishEvent(ctx, "withdrawal.completed", payment.ID, payloadBytes)

	return payment.ID, false, nil
}

type ConvertCurrencyRequest struct {
	Amount       decimal.Decimal
	FromCurrency string
	ToCurrency   string
}

func (s *TransferService) ConvertCurrency(ctx context.Context, req ConvertCurrencyRequest) (decimal.Decimal, error) {
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, ErrInvalidAmount
	}
	if req.FromCurrency == req.ToCurrency {
		return req.Amount, nil
	}

	grpcReq := &pb.ConvertRequest{
		Amount:       req.Amount.String(),
		FromCurrency: req.FromCurrency,
		ToCurrency:   req.ToCurrency,
	}

	grpcResp, err := s.currencyServiceClient.Convert(ctx, grpcReq)
	if err != nil {
		return decimal.Zero, ErrCurrencyServiceError
	}

	if grpcResp.Error != "" {
		return decimal.Zero, errors.New(grpcResp.Error)
	}

	convertedAmount, err := decimal.NewFromString(grpcResp.ConvertedAmount)
	if err != nil {
		return decimal.Zero, err
	}

	eventPayload := map[string]interface{}{
		"from_currency":    req.FromCurrency,
		"to_currency":      req.ToCurrency,
		"original_amount":  req.Amount.String(),
		"converted_amount": grpcResp.ConvertedAmount,
		"rate":             grpcResp.Rate,
	}

	payloadBytes, _ := json.Marshal(eventPayload)

	s.eventPublisher.PublishEvent(ctx, "conversion.completed", uuid.New(), payloadBytes)

	return convertedAmount, nil
}

func (s *TransferService) GetSupportedCurrencies(ctx context.Context) ([]string, error) {
	resp, err := s.currencyServiceClient.GetSupportedCurrencies(ctx, &pb.Empty{})
	if err != nil {
		return nil, ErrCurrencyServiceError
	}

	return resp.Currencies, nil
}

func (s *TransferService) GetExchangeRates(ctx context.Context, baseCurrency string) (map[string]decimal.Decimal, error) {
	resp, err := s.currencyServiceClient.GetRates(ctx, &pb.GetRatesRequest{
		BaseCurrency: baseCurrency,
	})
	if err != nil {
		return nil, ErrCurrencyServiceError
	}

	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}

	result := make(map[string]decimal.Decimal)
	for currency, rateStr := range resp.Rates {
		rate, err := decimal.NewFromString(rateStr)
		if err != nil {
			continue
		}
		result[currency] = rate
	}

	return result, nil
}

func (s *TransferService) ListUserTransfers(ctx context.Context, userID uuid.UUID, limit, offset int) ([]repository.Transfer, error) {
	return s.repo.ListTransfersByUserID(ctx, userID, limit, offset)
}

func (s *TransferService) GetTransfer(ctx context.Context, transferID uuid.UUID) (*repository.Transfer, error) {
	return s.repo.GetTransferByID(ctx, transferID)
}
