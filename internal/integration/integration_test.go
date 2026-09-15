//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	rediscache "github.com/featureflag-service/featureflag-service/internal/cache/redis"
	"github.com/featureflag-service/featureflag-service/internal/flag"
	"github.com/featureflag-service/featureflag-service/internal/httpapi"
	"github.com/featureflag-service/featureflag-service/internal/migrate"
	"github.com/featureflag-service/featureflag-service/internal/store/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func TestIntegrationCreateEvaluate(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	redisURL := os.Getenv("REDIS_URL")
	if dbURL == "" || redisURL == "" {
		t.Skip("DATABASE_URL and REDIS_URL required for integration tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	if err := migrate.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(opts)
	t.Cleanup(func() { _ = rdb.Close() })

	suffix := time.Now().UnixNano()
	flagName := "itest-flag"
	_ = suffix

	// isolate with unique flag names
	flagName = "itest-" + time.Now().Format("150405.000000000")

	svc := flag.NewService(postgres.NewStore(pool), rediscache.New(rdb), time.Minute)
	h := httpapi.NewServer(svc, nil).Handler()

	create := func(user string, enabled bool) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]any{
			"user":     user,
			"flagname": flagName,
			"enabled":  enabled,
		})
		req := httptest.NewRequest(http.MethodPost, "/v1/flag", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	eval := func(user string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/v1/flag?user="+user+"&flagname="+flagName, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	if rec := create("", false); rec.Code != http.StatusCreated {
		t.Fatalf("create global: %d %s", rec.Code, rec.Body.String())
	}
	if rec := create("", false); rec.Code != http.StatusConflict {
		t.Fatalf("expected conflict, got %d", rec.Code)
	}
	if rec := eval("alice"); rec.Code != http.StatusOK {
		t.Fatalf("eval global: %d %s", rec.Code, rec.Body.String())
	} else {
		var resp struct {
			Enabled bool `json:"enabled"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.Enabled {
			t.Fatal("expected global false")
		}
	}

	if rec := create("alice", true); rec.Code != http.StatusCreated {
		t.Fatalf("create user: %d %s", rec.Code, rec.Body.String())
	}
	if rec := eval("alice"); rec.Code != http.StatusOK {
		t.Fatalf("eval override: %d", rec.Code)
	} else {
		var resp struct {
			Enabled bool `json:"enabled"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if !resp.Enabled {
			t.Fatal("expected override true")
		}
	}
}
