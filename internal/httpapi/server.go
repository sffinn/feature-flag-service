package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/feature-flag-service/feature-flag-service/internal/flag"
)

// FlagService is the subset of flag.Service used by handlers.
type FlagService interface {
	Create(ctx context.Context, username, flagName string, enabled bool) (string, error)
	Update(ctx context.Context, username, flagName string, enabled bool) (string, error)
	Evaluate(ctx context.Context, username, flagName string) (string, bool, error)
}

type Server struct {
	flags  FlagService
	logger *slog.Logger
}

func NewServer(flags FlagService, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{flags: flags, logger: logger}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("POST /v1/flag", s.handleCreate)
	mux.HandleFunc("PUT /v1/flag", s.handleUpdate)
	mux.HandleFunc("GET /v1/flag", s.handleEvaluate)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

type createRequest struct {
	User     string `json:"user"`
	FlagName string `json:"flagname"`
	Enabled  bool   `json:"enabled"`
}

type createResponse struct {
	FlagName string `json:"flagname"`
}

type updateRequest struct {
	User     string `json:"user"`
	FlagName string `json:"flagname"`
	Enabled  bool   `json:"enabled"`
}

type updateResponse struct {
	FlagName string `json:"flagname"`
}

type evaluateResponse struct {
	FlagName string `json:"flagname"`
	Enabled  bool   `json:"enabled"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	name, err := s.flags.Create(r.Context(), req.User, req.FlagName, req.Enabled)
	if err != nil {
		s.writeFlagError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, createResponse{FlagName: name})
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	name, err := s.flags.Update(r.Context(), req.User, req.FlagName, req.Enabled)
	if err != nil {
		s.writeFlagError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updateResponse{FlagName: name})
}

func (s *Server) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	user := r.URL.Query().Get("user")
	flagName := r.URL.Query().Get("flagname")

	name, enabled, err := s.flags.Evaluate(r.Context(), user, flagName)
	if err != nil {
		s.writeFlagError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, evaluateResponse{FlagName: name, Enabled: enabled})
}

func (s *Server) writeFlagError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, flag.ErrInvalidFlagName):
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid flag name"})
	case errors.Is(err, flag.ErrFlagExists):
		writeJSON(w, http.StatusConflict, errorResponse{Error: "flag exists"})
	case errors.Is(err, flag.ErrFlagNotFound):
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "flag not found"})
	default:
		s.logger.Error("request failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
