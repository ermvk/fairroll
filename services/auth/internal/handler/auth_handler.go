package handler

import (
	"encoding/json"
	"fairroll/pkg/middleware"
	"fairroll/services/auth/internal/model"
	"fairroll/services/auth/internal/service"
	"net/http"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

func (h *AuthHandler) RegisterRouters(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/register", h.Register)
	mux.HandleFunc("POST /auth/login", h.Login)
	mux.HandleFunc("POST /auth/refresh", h.Refresh)
	mux.HandleFunc("GET /auth/me", h.Me)
}

// Register habdler for registration of new user
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {

	var req service.RegisterRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := h.authService.Register(r.Context(), &req)

	if err != nil {
		middleware.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	middleware.RespondSuccess(w, http.StatusCreated, map[string]interface{}{
		"user": &model.UserResponse{
			ID:        user.ID,
			Email:     user.Email,
			UserName:  user.Username,
			KYCstatus: user.KYCStatus,
			CreatedAt: user.CreatedAt,
		},
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req model.LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	authResp, err := h.authService.Login(r.Context(), &service.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		middleware.RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	middleware.RespondSuccess(w, http.StatusOK, authResp)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req model.RefreshRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	authResp, err := h.authService.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		middleware.RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	middleware.RespondSuccess(w, http.StatusOK, authResp)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	middleware.RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"message": "handler ME still not implemented",
	})
}
