package service

import (
	"context"
	"errors"
	"time"

	"fairroll/services/wallet/internal/repository"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var (
	ErrInvalidAmount       = errors.New("amount must be positive")
	ErrIdempotencyConflict = errors.New("transaction already processed")
)

type AccountType string

const (
	SystemHouseAccountType AccountType = "system_house"
	UserMainAccountType    AccountType = "user_main"
)

type Direction string

const (
	DirectionDebit  Direction = "debit"
	DirectionCredit Direction = "credit"
)

type TxType string

const (
	TxTypeDeposit    TxType = "deposit"
	TxTypeWithdrawal TxType = "withdrawal"
	TxTypeTransfer   TxType = "transfer"
)

type TxStatus string

const (
	TxStatusCompleted TxStatus = "completed"
)

type WalletService struct {
	repo *repository.WalletRepository
}

func NewWalletService(repo *repository.WalletRepository) *WalletService {
	return &WalletService{
		repo: repo,
	}
}

type DepositRequest struct {
	UserID         int64
	Amount         decimal.Decimal
	Currency       string
	Source         string
	IdempotencyKey string
}

type WithdrawRequest struct {
	UserID         int64
	Amount         decimal.Decimal
	Currency       string
	IdempotencyKey string
}

func (s *WalletService) GetBalance(ctx context.Context, userID int64, currency string) (decimal.Decimal, error) {
	acc, err := s.repo.GetAccountByUserID(ctx, userID, currency)
	if err != nil {
		return decimal.Zero, err
	}
	return s.repo.GetBalance(ctx, acc.ID)
}

func (s *WalletService) Deposit(ctx context.Context, req DepositRequest) (*repository.Transaction, bool, error) {
	existingTx, err := s.repo.GetTransactionByIdempotencyKey(ctx, req.IdempotencyKey)
	if err == nil {
		return existingTx, true, nil
	}

	if !errors.Is(err, repository.ErrNotFound) {
		return nil, false, err
	}

	userAcc, err := s.repo.GetOrCreateAccount(ctx, req.UserID, req.Currency, string(UserMainAccountType))
	if err != nil {
		return nil, false, err
	}

	houseAcc, err := s.repo.GetOrCreateAccount(ctx, 0, req.Currency, string(SystemHouseAccountType))
	if err != nil {
		return nil, false, err
	}

	now := time.Now()
	tx := &repository.Transaction{
		ID:             uuid.New(),
		IdempotencyKey: req.IdempotencyKey,
		Type:           string(TxTypeDeposit),
		Status:         string(TxStatusCompleted),
		CreatedAt:      now,
	}

	entries := []repository.LedgerEntry{
		{
			TransactionID: tx.ID,
			AccountID:     houseAcc.ID,
			Direction:     string(DirectionDebit),
			Amount:        req.Amount,
			CreatedAt:     now,
		},
		{
			TransactionID: tx.ID,
			AccountID:     userAcc.ID,
			Direction:     string(DirectionCredit),
			Amount:        req.Amount,
			CreatedAt:     now,
		},
	}

	if err := s.repo.CreateTransactionWithEntries(ctx, tx, entries); err != nil {
		return nil, false, err

	}

	return tx, false, nil
}

func (s *WalletService) WithDraw(ctx context.Context, req WithdrawRequest) (*repository.Transaction, bool, error) {
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return nil, false, ErrInvalidAmount
	}

	existingTx, err := s.repo.GetTransactionByIdempotencyKey(ctx, req.IdempotencyKey)
	if err == nil {
		return existingTx, true, nil
	}

	if !errors.Is(err, repository.ErrNotFound) {
		return nil, false, err
	}

	userAcc, err := s.repo.GetAccountByUserID(ctx, req.UserID, req.Currency)
	if err != nil {
		return nil, false, err
	}

	balance, err := s.repo.GetBalance(ctx, userAcc.ID)
	if err != nil {
		return nil, false, err
	}

	if balance.LessThan(req.Amount) {
		return nil, false, repository.ErrInsufficientFunds
	}

	houseAcc, err := s.repo.GetOrCreateAccount(ctx, 0, req.Currency, string(SystemHouseAccountType))
	if err != nil {
		return nil, false, err
	}

	now := time.Now()

	tx := &repository.Transaction{
		ID:             uuid.New(),
		IdempotencyKey: req.IdempotencyKey,
		Type:           string(TxTypeWithdrawal),
		Status:         string(TxStatusCompleted),
		CreatedAt:      now,
	}

	entries := []repository.LedgerEntry{
		{
			TransactionID: tx.ID,
			AccountID:     userAcc.ID,
			Direction:     string(DirectionDebit),
			Amount:        req.Amount,
			CreatedAt:     now,
		},
		{
			TransactionID: tx.ID,
			AccountID:     houseAcc.ID,
			Direction:     string(DirectionCredit),
			Amount:        req.Amount,
			CreatedAt:     now,
		},
	}

	if err := s.repo.CreateTransactionWithEntries(ctx, tx, entries); err != nil {
		return nil, false, err
	}

	return tx, false, nil
}

func (s *WalletService) ListTransactions(ctx context.Context, userID int64, limit, offset int) ([]repository.Transaction, error) {
	return s.repo.ListTransactionsByUserID(ctx, userID, limit, offset)
}
