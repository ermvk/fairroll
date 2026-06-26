package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"
)

var (
	ErrNotFound            = errors.New("not found")
	ErrIdempotencyConflict = errors.New("transfer already processed")
)

type Transfer struct {
	ID             uuid.UUID
	FromUserID     uuid.UUID
	ToUserID       uuid.UUID
	Amount         decimal.Decimal
	Currency       string
	Type           string
	Status         string
	IdempotencyKey string
	CreatedAt      time.Time
}

type Payment struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	Amount         decimal.Decimal
	Currency       string
	Type           string
	Status         string
	Source         string
	Destination    string
	IdempotencyKey string
	CreatedAt      time.Time
}

type TransferRepository struct {
	db *pgx.Conn
}

func NewTransferRepository(db *pgx.Conn) *TransferRepository {
	return &TransferRepository{db: db}
}

func (r *TransferRepository) CreateTransfer(ctx context.Context, transfer *Transfer) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Get sender and receiver account IDs from wallet
	senderAccID, err := r.getAccountID(ctx, tx, transfer.FromUserID, transfer.Currency)
	if err != nil {
		return err
	}
	receiverAccID, err := r.getAccountID(ctx, tx, transfer.ToUserID, transfer.Currency)
	if err != nil {
		return err
	}

	txID := uuid.New()
	createTxQuery := `INSERT INTO transactions (id, idempotency_key, type, status, created_at)
		VALUES ($1, $2, $3, $4, $5)`
	_, err = tx.Exec(ctx, createTxQuery, txID, transfer.IdempotencyKey, "transfer", "completed", transfer.CreatedAt)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `INSERT INTO ledger_entries (transaction_id, account_id, direction, amount, created_at)
		VALUES ($1, $2, $3, $4, $5)`,
		txID, senderAccID, "debit", transfer.Amount, transfer.CreatedAt)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `INSERT INTO ledger_entries (transaction_id, account_id, direction, amount, created_at)
		VALUES ($1, $2, $3, $4, $5)`,
		txID, receiverAccID, "credit", transfer.Amount, transfer.CreatedAt)
	if err != nil {
		return err
	}

	insertTransferQuery := `INSERT INTO transfers (id, from_user_id, to_user_id, amount, currency, status, idempotency_key, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err = tx.Exec(ctx, insertTransferQuery,
		transfer.ID, transfer.FromUserID, transfer.ToUserID, transfer.Amount,
		transfer.Currency, transfer.Status, transfer.IdempotencyKey, transfer.CreatedAt)
	if err != nil {
		return err
	}

	err = tx.Commit(ctx)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// уже есть такой idempotency_key — отдадим существующий
			return ErrIdempotencyConflict
		}
		return err
	}

	return nil
}

func (r *TransferRepository) GetBalance(ctx context.Context, userID uuid.UUID, currency string) (decimal.Decimal, error) {
	var accID int64
	query := `SELECT id FROM accounts WHERE user_id = $1 AND currency = $2`
	err := r.db.QueryRow(ctx, query, userID, currency).Scan(&accID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return decimal.Zero, ErrNotFound
		}
		return decimal.Zero, err
	}
	var balance decimal.Decimal
	balanceQuery := `SELECT COALESCE(SUM(CASE WHEN direction = 'credit' THEN amount ELSE -amount END), 0)
		FROM ledger_entries
		WHERE account_id = $1`
	err = r.db.QueryRow(ctx, balanceQuery, accID).Scan(&balance)
	if err != nil {
		return decimal.Zero, err
	}
	return balance, nil
}

func (r *TransferRepository) GetTransferByIdempotencyKey(ctx context.Context, key string) (*Transfer, error) {
	var transfer Transfer
	query := `SELECT id, from_user_id, to_user_id, amount, currency, status, idempotency_key, created_at
		FROM transfers WHERE idempotency_key = $1`

	err := r.db.QueryRow(ctx, query, key).Scan(
		&transfer.ID, &transfer.FromUserID, &transfer.ToUserID, &transfer.Amount,
		&transfer.Currency, &transfer.Status, &transfer.IdempotencyKey, &transfer.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &transfer, err
}

func (r *TransferRepository) GetTransferByID(ctx context.Context, transferID uuid.UUID) (*Transfer, error) {
	var transfer Transfer
	query := `SELECT id, from_user_id, to_user_id, amount, currency, status, idempotency_key, created_at
		FROM transfers WHERE id = $1`

	err := r.db.QueryRow(ctx, query, transferID).Scan(
		&transfer.ID, &transfer.FromUserID, &transfer.ToUserID, &transfer.Amount,
		&transfer.Currency, &transfer.Status, &transfer.IdempotencyKey, &transfer.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pgx.ErrNoRows
		}
		return nil, err
	}

	return &transfer, nil
}

func (r *TransferRepository) ListTransfersByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Transfer, error) {
	query := `
    SELECT id, from_user_id, to_user_id, amount, currency, status, idempotency_key, created_at
    FROM transfers
    WHERE from_user_id = $1 OR to_user_id = $1
    ORDER BY created_at DESC
    LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transfers []Transfer
	for rows.Next() {
		var t Transfer
		if err := rows.Scan(&t.ID, &t.FromUserID, &t.ToUserID, &t.Amount,
			&t.Currency, &t.Status, &t.IdempotencyKey, &t.CreatedAt); err != nil {
			return nil, err
		}

		transfers = append(transfers, t)
	}
	return transfers, rows.Err()
}

func (r *TransferRepository) CreatePayment(ctx context.Context, payment *Payment) error {
	query := `INSERT INTO payments (id, user_id, amount, currency, type, status, source, destination, idempotency_key, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`
	_, err := r.db.Exec(ctx, query,
		payment.ID, payment.UserID, payment.Amount, payment.Currency,
		payment.Type, payment.Status, payment.Source, payment.Destination,
		payment.IdempotencyKey, payment.CreatedAt)

	return err
}

func (r *TransferRepository) GetPaymentByIdempotencyKey(ctx context.Context, key string) (*Payment, error) {
	var payment Payment
	query := `SELECT id, user_id, amount, currency, type, status, source, destination, idempotency_key, created_at
        FROM payments WHERE idempotency_key = $1`
	err := r.db.QueryRow(ctx, query, key).Scan(
		&payment.ID, &payment.UserID, &payment.Amount, &payment.Currency,
		&payment.Type, &payment.Status, &payment.Source, &payment.Destination,
		&payment.IdempotencyKey, &payment.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &payment, nil
}

// help fnx
func (r *TransferRepository) getAccountID(ctx context.Context, tx pgx.Tx, userID uuid.UUID, currency string) (int64, error) {
	var accID int64
	query := `SELECT id FROM accounts WHERE user_id = $1 AND currency = $2 FOR UPDATE`
	if err := tx.QueryRow(ctx, query, userID, currency).Scan(&accID); err != nil {
		return 0, err
	}
	return accID, nil
}
