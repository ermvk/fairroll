package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"fairroll/services/wallet/internal/model"
	"fairroll/services/wallet/internal/repository"
	"fairroll/services/wallet/internal/service"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/shopspring/decimal"
)

type WalletHandler struct {
	walletService *service.WalletService
}

func NewWalletHandler(walletService *service.WalletService) *WalletHandler {
	return &WalletHandler{
		walletService: walletService,
	}
}

// GetAccount handles GET /wallet/accounts/{userId}
func (h *WalletHandler) GetAccount(w http.ResponseWriter, r *http.Request, userId openapi_types.UUID, params model.GetAccountParams) {
	currency := "USD"
	if params.Currency != nil {
		currency = *params.Currency
	}

	balance, err := h.walletService.GetBalance(r.Context(), uuid.UUID(userId), currency)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	balanceStr := balance.String()
	respondJSON(w, http.StatusOK, model.AccountResponse{
		UserId:   &userId,
		Currency: &currency,
		Balance:  &balanceStr,
	})
}

// Deposit handles POST /wallet/deposit
func (h *WalletHandler) Deposit(w http.ResponseWriter, r *http.Request, params model.DepositParams) {
	var req model.DepositRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid amount format")
		return
	}

	source := ""
	if req.Source != nil {
		source = *req.Source
	}

	// tx — final transaction
	// wasReplay — true if already processed
	tx, wasReplay, err := h.walletService.Deposit(r.Context(), service.DepositRequest{
		UserID:         uuid.UUID(req.UserId),
		Amount:         amount,
		Currency:       req.Currency,
		Source:         source,
		IdempotencyKey: params.IdempotencyKey,
	})
	if err != nil {
		handleServiceError(w, err)
		return
	}

	// 201 for new one and 208 if already processed
	status := http.StatusCreated
	if wasReplay {
		status = http.StatusAlreadyReported
	}
	respondJSON(w, status, toTransactionResponse(tx))
}

// Withdraw handles POST /wallet/withdraw
func (h *WalletHandler) Withdraw(w http.ResponseWriter, r *http.Request, params model.WithdrawParams) {
	var req model.WithdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid amount format")
		return
	}

	tx, wasReplay, err := h.walletService.WithDraw(r.Context(), service.WithdrawRequest{
		UserID:         uuid.UUID(req.UserId),
		Amount:         amount,
		Currency:       req.Currency,
		IdempotencyKey: params.IdempotencyKey,
	})
	if err != nil {
		handleServiceError(w, err)
		return
	}

	status := http.StatusCreated
	if wasReplay {
		status = http.StatusAlreadyReported
	}
	respondJSON(w, status, toTransactionResponse(tx))
}

// Transfer handles POST /wallet/transfer
func (h *WalletHandler) Transfer(w http.ResponseWriter, r *http.Request, params model.TransferParams) {
	respondError(w, http.StatusNotImplemented, "transfer not implemented yet")
}

// ListTransactions handles GET /wallet/transactions/{userId}
func (h *WalletHandler) ListTransactions(w http.ResponseWriter, r *http.Request, userId openapi_types.UUID, params model.ListTransactionsParams) {
	limit := 20
	if params.Limit != nil {
		limit = *params.Limit
	}

	offset := 0
	if params.Offset != nil {
		offset = *params.Offset
	}

	txs, err := h.walletService.ListTransactions(r.Context(), uuid.UUID(userId), limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	responses := make([]model.TransactionResponse, 0, len(txs))
	for _, tx := range txs {
		responses = append(responses, toTransactionResponse(&tx))
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"transactions": responses,
		"total":        len(responses),
	})
}

// helper fnx

func toTransactionResponse(tx *repository.Transaction) model.TransactionResponse {
	status := model.TransactionResponseStatus(tx.Status)
	txType := model.TransactionResponseType(tx.Type)
	id := openapi_types.UUID(tx.ID)

	return model.TransactionResponse{
		Id:        &id,
		Type:      &txType,
		Status:    &status,
		CreatedAt: &tx.CreatedAt,
	}
}

func handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidAmount):
		respondError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, repository.ErrInsufficientFunds):
		respondError(w, http.StatusPaymentRequired, err.Error())
	case errors.Is(err, service.ErrIdempotencyConflict):
		respondError(w, http.StatusConflict, err.Error())
	default:
		respondError(w, http.StatusInternalServerError, err.Error())
	}
}

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, model.ErrorResponse{
		Code:      &status,
		Error:     &message,
		Timestamp: timePtr(),
	})
}

func timePtr() *time.Time {
	t := time.Now()
	return &t
}
