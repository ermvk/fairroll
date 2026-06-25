package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

var ErrNotFound = errors.New("not found")

var ErrInsufficientFunds = errors.New("insufficient funds")

type Account struct {
	ID          int64
	UserID      uuid.UUID
	Currency    string
	AccountType string
	CreatedAt   time.Time
}

type Transaction struct {
	ID             uuid.UUID
	IdempotencyKey string
	Type           string
	Status         string
	CreatedAt      time.Time
}

type LedgerEntry struct {
	ID            int64
	TransactionID uuid.UUID
	AccountID     int64
	Direction     string
	Amount        decimal.Decimal
	CreatedAt     time.Time
}

type WalletRepository struct {
	db *pgx.Conn
}

func NewWalletRepository(db *pgx.Conn) *WalletRepository {
	return &WalletRepository{
		db: db,
	}
}

func (r *WalletRepository) GetOrCreateAccount(ctx context.Context, userID uuid.UUID, currency, accountType string) (*Account, error) {
	acc, err := r.GetAccountByUserID(ctx, userID, currency)
	if err == nil {
		return acc, nil
	}

	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	query := `INSERT INTO accounts (user_id, currency, account_type, created_at)
    VALUES ($1, $2, $3, $4) RETURNING id`

	acc = &Account{
		UserID:      userID,
		Currency:    currency,
		AccountType: accountType,
		CreatedAt:   time.Now(),
	}

	err = r.db.QueryRow(ctx, query, userID, currency, accountType, acc.CreatedAt).Scan(&acc.ID)
	if err != nil {
		return nil, err
	}
	return acc, nil
}

func (r *WalletRepository) GetAccountByUserID(ctx context.Context, userID uuid.UUID, currency string) (*Account, error) {
	query := `SELECT id, user_id, currency, account_type, created_at
		FROM accounts WHERE user_id = $1 AND currency = $2`
	var acc Account

	err := r.db.QueryRow(ctx, query, userID, currency).Scan(
		&acc.ID,
		&acc.UserID,
		&acc.Currency,
		&acc.AccountType,
		&acc.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &acc, nil
}

func (r *WalletRepository) GetBalance(ctx context.Context, accountID int64) (decimal.Decimal, error) {
	query := `SELECT COALESCE(SUM(
	CASE WHEN direction = 'credit' THEN  amount ELSE -amount END),0) FROM ledger_entries WHERE account_id = $1`

	var balanceStr string
	err := r.db.QueryRow(ctx, query, accountID).Scan(&balanceStr)
	if err != nil {
		return decimal.Zero, err
	}

	return decimal.NewFromString(balanceStr)
}

func (r *WalletRepository) CreateTransactionWithEntries(ctx context.Context, tx *Transaction, entries []LedgerEntry) error {
	pgTx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}

	defer pgTx.Rollback(ctx)

	txQuery := `INSERT INTO transactions (id, idempotency_key, type, status, created_at)
		VALUES ($1, $2, $3, $4, $5)`

	_, err = pgTx.Exec(ctx, txQuery, tx.ID, tx.IdempotencyKey, tx.Type, tx.Status, tx.CreatedAt)
	if err != nil {
		return err
	}

	entryQuery := `INSERT INTO ledger_entries (transaction_id, account_id, direction, amount, created_at)
    	VALUES ($1, $2, $3, $4, $5)`

	for _, e := range entries {
		_, err = pgTx.Exec(ctx, entryQuery, e.TransactionID, e.AccountID, e.Direction, e.Amount.String(), e.CreatedAt)
		if err != nil {
			return err
		}
	}

	return pgTx.Commit(ctx)
}

func (r *WalletRepository) GetTransactionByIdempotencyKey(ctx context.Context, key string) (*Transaction, error) {
	query := `SELECT id, idempotency_key, type, status, created_at
		FROM transactions WHERE idempotency_key = $1`

	var tx Transaction

	err := r.db.QueryRow(ctx, query, key).Scan(
		&tx.ID,
		&tx.IdempotencyKey,
		&tx.Type,
		&tx.Status,
		&tx.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &tx, nil
}

func (r *WalletRepository) ListTransactionsByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Transaction, error) {
	query := `SELECT DISTINCT t.id, t.idempotency_key, t.type, t.status, t.created_at
		FROM transactions t
		JOIN ledger_entries le ON le.transaction_id = t.id
		JOIN accounts a ON a.id = le.account_id
		WHERE a.user_id = $1
		ORDER BY t.created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var txs []Transaction

	for rows.Next() {
		var tx Transaction
		if err := rows.Scan(&tx.ID, &tx.IdempotencyKey, &tx.Type, &tx.Status, &tx.CreatedAt); err != nil {
			return nil, err
		}

		txs = append(txs, tx)
	}

	return txs, rows.Err()
}
