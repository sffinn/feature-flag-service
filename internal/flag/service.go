package flag

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Store persists feature flags.
type Store interface {
	CreateGlobal(ctx context.Context, f GlobalFlag) error
	CreateUser(ctx context.Context, f UserFlag) error
	UpdateGlobal(ctx context.Context, f GlobalFlag) error
	UpdateUser(ctx context.Context, f UserFlag) error
	GetGlobal(ctx context.Context, flagName string) (enabled bool, err error)
	GetUser(ctx context.Context, username, flagName string) (enabled bool, err error)
}

// Cache is a cache-aside layer for flag evaluation.
type Cache interface {
	Get(ctx context.Context, key string) (enabled bool, hit bool, err error)
	Set(ctx context.Context, key string, enabled bool, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
}

// Service implements create and evaluate flows.
type Service struct {
	store Store
	cache Cache
	ttl   time.Duration
}

func NewService(store Store, cache Cache, ttl time.Duration) *Service {
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &Service{store: store, cache: cache, ttl: ttl}
}

func globalKey(flagName string) string {
	return fmt.Sprintf("flag:global:%s", flagName)
}

func userKey(username, flagName string) string {
	return fmt.Sprintf("flag:user:%s:%s", username, flagName)
}

// Create inserts a global flag (empty user) or a user override.
func (s *Service) Create(ctx context.Context, username, flagName string, enabled bool) (string, error) {
	flagName, err := ValidateFlagName(flagName)
	if err != nil {
		return "", err
	}

	username = strings.TrimSpace(username)
	if username == "" {
		err = s.store.CreateGlobal(ctx, GlobalFlag{FlagName: flagName, Enabled: enabled})
		if err != nil {
			return "", err
		}
		_ = s.cache.Delete(ctx, globalKey(flagName))
		return flagName, nil
	}

	err = s.store.CreateUser(ctx, UserFlag{Username: username, FlagName: flagName, Enabled: enabled})
	if err != nil {
		return "", err
	}
	_ = s.cache.Delete(ctx, userKey(username, flagName))
	return flagName, nil
}

func (s *Service) Update(ctx context.Context, username, flagName string, enabled bool) (string, error) {
	flagName, err := ValidateFlagName(flagName)
	if err != nil {
		return "", err
	}
	username = strings.TrimSpace(username)
	if username == "" {
		err = s.store.UpdateGlobal(ctx, GlobalFlag{FlagName: flagName, Enabled: enabled})
		if err != nil {
			return "", err
		}
		_ = s.cache.Delete(ctx, globalKey(flagName))
		return flagName, nil
	}

	err = s.store.UpdateUser(ctx, UserFlag{Username: username, FlagName: flagName, Enabled: enabled})
	if err != nil {
		return "", err
	}
	_ = s.cache.Delete(ctx, userKey(username, flagName))
	return flagName, nil
}

// Evaluate returns whether a flag is enabled for a user (override, then global).
func (s *Service) Evaluate(ctx context.Context, username, flagName string) (string, bool, error) {
	flagName, err := ValidateFlagName(flagName)
	if err != nil {
		return "", false, err
	}
	username = strings.TrimSpace(username)

	if username != "" {
		enabled, found, err := s.getUser(ctx, username, flagName)
		if err != nil {
			return "", false, err
		}
		if found {
			return flagName, enabled, nil
		}
	}

	enabled, found, err := s.getGlobal(ctx, flagName)
	if err != nil {
		return "", false, err
	}
	if !found {
		return "", false, ErrFlagNotFound
	}
	return flagName, enabled, nil
}

func (s *Service) getUser(ctx context.Context, username, flagName string) (bool, bool, error) {
	key := userKey(username, flagName)
	if enabled, hit, err := s.cache.Get(ctx, key); err == nil && hit {
		return enabled, true, nil
	}

	enabled, err := s.store.GetUser(ctx, username, flagName)
	if errors.Is(err, ErrFlagNotFound) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	_ = s.cache.Set(ctx, key, enabled, s.ttl)
	return enabled, true, nil
}

func (s *Service) getGlobal(ctx context.Context, flagName string) (bool, bool, error) {
	key := globalKey(flagName)
	if enabled, hit, err := s.cache.Get(ctx, key); err == nil && hit {
		return enabled, true, nil
	}

	enabled, err := s.store.GetGlobal(ctx, flagName)
	if errors.Is(err, ErrFlagNotFound) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	_ = s.cache.Set(ctx, key, enabled, s.ttl)
	return enabled, true, nil
}
