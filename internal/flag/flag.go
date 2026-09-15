package flag

import (
	"errors"
	"regexp"
	"strings"
)

const maxFlagNameLen = 128

var (
	ErrInvalidFlagName = errors.New("invalid flag name")
	ErrFlagExists      = errors.New("flag exists")
	ErrFlagNotFound    = errors.New("flag not found")
)

var flagNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// UserFlag is a per-user override of a feature flag.
type UserFlag struct {
	Username string
	FlagName string
	Enabled  bool
}

// GlobalFlag is a globally scoped feature flag.
type GlobalFlag struct {
	FlagName string
	Enabled  bool
}

// ValidateFlagName trims and validates a flag name per the service contract.
func ValidateFlagName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > maxFlagNameLen || !flagNamePattern.MatchString(name) {
		return "", ErrInvalidFlagName
	}
	return name, nil
}
