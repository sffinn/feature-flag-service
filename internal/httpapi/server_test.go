package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/featureflag-service/featureflag-service/internal/flag"
	"github.com/featureflag-service/featureflag-service/internal/httpapi"
)

type stubFlags struct {
	createFn   func(ctx context.Context, user, name string, enabled bool) (string, error)
	evaluateFn func(ctx context.Context, user, name string) (string, bool, error)
}

func (s stubFlags) Create(ctx context.Context, user, name string, enabled bool) (string, error) {
	return s.createFn(ctx, user, name, enabled)
}

func (s stubFlags) Evaluate(ctx context.Context, user, name string) (string, bool, error) {
	return s.evaluateFn(ctx, user, name)
}

func TestHealth(t *testing.T) {
	t.Parallel()
	srv := httpapi.NewServer(stubFlags{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestCreateAndEvaluateHandlers(t *testing.T) {
	t.Parallel()

	flags := stubFlags{
		createFn: func(_ context.Context, user, name string, enabled bool) (string, error) {
			if name == "bad!" {
				return "", flag.ErrInvalidFlagName
			}
			if name == "exists" {
				return "", flag.ErrFlagExists
			}
			if user == "" && name == "ok" && enabled {
				return "ok", nil
			}
			return "", errors.New("unexpected")
		},
		evaluateFn: func(_ context.Context, user, name string) (string, bool, error) {
			if name == "missing" {
				return "", false, flag.ErrFlagNotFound
			}
			if name == "ok" && user == "alice" {
				return "ok", true, nil
			}
			return "", false, errors.New("unexpected")
		},
	}
	h := httpapi.NewServer(flags, nil).Handler()

	t.Run("create success", func(t *testing.T) {
		body := `{"user":"","flagname":"ok","enabled":true}`
		req := httptest.NewRequest(http.MethodPost, "/v1/flag", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
		var resp map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["flagname"] != "ok" {
			t.Fatalf("resp %#v", resp)
		}
	})

	t.Run("create invalid", func(t *testing.T) {
		body := `{"user":"","flagname":"bad!","enabled":true}`
		req := httptest.NewRequest(http.MethodPost, "/v1/flag", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d", rec.Code)
		}
	})

	t.Run("create conflict", func(t *testing.T) {
		body := `{"user":"","flagname":"exists","enabled":true}`
		req := httptest.NewRequest(http.MethodPost, "/v1/flag", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status %d", rec.Code)
		}
	})

	t.Run("evaluate success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/flag?user=alice&flagname=ok", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
	})

	t.Run("evaluate not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/flag?user=alice&flagname=missing", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status %d", rec.Code)
		}
	})
}
