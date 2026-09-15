package flag_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/featureflag-service/featureflag-service/internal/flag"
)

func TestValidateFlagName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in      string
		want    string
		wantErr error
	}{
		{in: "dark-mode", want: "dark-mode"},
		{in: "  feature_1  ", want: "feature_1"},
		{in: "a.b", want: "a.b"},
		{in: "", wantErr: flag.ErrInvalidFlagName},
		{in: "   ", wantErr: flag.ErrInvalidFlagName},
		{in: "bad name", wantErr: flag.ErrInvalidFlagName},
		{in: "bad!", wantErr: flag.ErrInvalidFlagName},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			got, err := flag.ValidateFlagName(tt.in)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}

	long := make([]byte, 129)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := flag.ValidateFlagName(string(long)); !errors.Is(err, flag.ErrInvalidFlagName) {
		t.Fatalf("expected invalid for long name, got %v", err)
	}
}

type memStore struct {
	mu       sync.Mutex
	global   map[string]bool
	userFlag map[string]bool
}

func newMemStore() *memStore {
	return &memStore{
		global:   map[string]bool{},
		userFlag: map[string]bool{},
	}
}

func userMapKey(user, name string) string { return user + "\x00" + name }

func (m *memStore) CreateGlobal(_ context.Context, f flag.GlobalFlag) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.global[f.FlagName]; ok {
		return flag.ErrFlagExists
	}
	m.global[f.FlagName] = f.Enabled
	return nil
}

func (m *memStore) CreateUser(_ context.Context, f flag.UserFlag) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := userMapKey(f.Username, f.FlagName)
	if _, ok := m.userFlag[k]; ok {
		return flag.ErrFlagExists
	}
	m.userFlag[k] = f.Enabled
	return nil
}

func (m *memStore) UpdateGlobal(_ context.Context, f flag.GlobalFlag) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.global[f.FlagName] = f.Enabled
	return nil
}

func (m *memStore) UpdateUser(_ context.Context, f flag.UserFlag) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.userFlag[userMapKey(f.Username, f.FlagName)] = f.Enabled
	return nil
}

func (m *memStore) GetGlobal(_ context.Context, flagName string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.global[flagName]
	if !ok {
		return false, flag.ErrFlagNotFound
	}
	return v, nil
}

func (m *memStore) GetUser(_ context.Context, username, flagName string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.userFlag[userMapKey(username, flagName)]
	if !ok {
		return false, flag.ErrFlagNotFound
	}
	return v, nil
}

type memCache struct {
	mu   sync.Mutex
	data map[string]bool
}

func newMemCache() *memCache {
	return &memCache{data: map[string]bool{}}
}

func (c *memCache) Get(_ context.Context, key string) (bool, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.data[key]
	return v, ok, nil
}

func (c *memCache) Set(_ context.Context, key string, enabled bool, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = enabled
	return nil
}

func (c *memCache) Delete(_ context.Context, keys ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, k := range keys {
		delete(c.data, k)
	}
	return nil
}

func TestEvaluatePrecedence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newMemStore()
	cache := newMemCache()
	svc := flag.NewService(store, cache, time.Minute)

	if _, err := svc.Create(ctx, "", "beta", false); err != nil {
		t.Fatal(err)
	}
	name, enabled, err := svc.Evaluate(ctx, "alice", "beta")
	if err != nil {
		t.Fatal(err)
	}
	if name != "beta" || enabled {
		t.Fatalf("got %s=%v, want beta=false from global", name, enabled)
	}

	if _, err := svc.Create(ctx, "alice", "beta", true); err != nil {
		t.Fatal(err)
	}
	_, enabled, err = svc.Evaluate(ctx, "alice", "beta")
	if err != nil {
		t.Fatal(err)
	}
	if !enabled {
		t.Fatal("expected user override true")
	}

	_, enabled, err = svc.Evaluate(ctx, "bob", "beta")
	if err != nil {
		t.Fatal(err)
	}
	if enabled {
		t.Fatal("bob should still see global false")
	}
}

func TestEvaluateNotFound(t *testing.T) {
	t.Parallel()
	svc := flag.NewService(newMemStore(), newMemCache(), time.Minute)
	_, _, err := svc.Evaluate(context.Background(), "alice", "missing")
	if !errors.Is(err, flag.ErrFlagNotFound) {
		t.Fatalf("got %v, want ErrFlagNotFound", err)
	}
}

func TestCreateConflictAndInvalid(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc := flag.NewService(newMemStore(), newMemCache(), time.Minute)

	if _, err := svc.Create(ctx, "", "bad name", true); !errors.Is(err, flag.ErrInvalidFlagName) {
		t.Fatalf("got %v, want invalid", err)
	}
	if _, err := svc.Create(ctx, "", "x", true); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, "", "x", false); !errors.Is(err, flag.ErrFlagExists) {
		t.Fatalf("got %v, want exists", err)
	}
}
