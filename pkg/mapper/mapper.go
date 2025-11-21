package mapper

import (
	"context"
	"fmt"
	"regexp"
	"sync"
)

// MapperConfig represents configuration for the user ID mapper
type MapperConfig struct {
	Type   string                 `yaml:"type"`
	Config map[string]interface{} `yaml:"config"`
}

func NewUserIDMapper(cfg MapperConfig) (UserIDMapper, error) {
	switch cfg.Type {
	case "passthrough", "":
		return NewPassthroughMapper(), nil
	case "static":
		m, err := NewStaticMapper(cfg.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to create static mapper: %w", err)
		}
		return m, nil
	case "regex":
		m, err := NewRegexMapper(cfg.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to create regex mapper: %w", err)
		}
		return m, nil
	default:
		return nil, fmt.Errorf("unknown mapper type: %s", cfg.Type)
	}
}

type PassthroughUserIDMapper struct {
}

var _ UserIDMapper = &PassthroughUserIDMapper{}

func NewPassthroughMapper() *PassthroughUserIDMapper {
	return &PassthroughUserIDMapper{}
}

func (m *PassthroughUserIDMapper) Map(ctx context.Context, userID string) (string, error) {
	return userID, nil
}

type UserIDMapper interface {
	Map(ctx context.Context, userID string) (string, error)
}

// StaticUserIDMapper is a simple in-memory mapper that uses a static map
type StaticUserIDMapper struct {
	mappings map[string]string
	mu       sync.RWMutex
}

// NewStaticMapper creates a new static mapper with the given mappings
func NewStaticMapper(config map[string]any) (*StaticUserIDMapper, error) {
	mappings := make(map[string]string)
	for k, v := range config {
		value, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("value is not a string")
		}
		mappings[k] = value
	}
	return &StaticUserIDMapper{
		mappings: mappings,
	}, nil
}

// Map converts a user ID from one format to another
func (m *StaticUserIDMapper) Map(ctx context.Context, userID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if mapped, ok := m.mappings[userID]; ok {
		return mapped, nil
	}

	// Return original if no mapping found
	return userID, nil
}

type RegexUserIDMapper struct {
	regex       *regexp.Regexp
	replacement string
}

var _ UserIDMapper = &RegexUserIDMapper{}

func NewRegexMapper(config map[string]any) (*RegexUserIDMapper, error) {
	regexPattern, ok := config["regex"].(string)
	if !ok {
		return nil, fmt.Errorf("regex is not a string")
	}
	replacement, ok := config["replacement"].(string)
	if !ok {
		return nil, fmt.Errorf("replacement is not a string")
	}
	regex, err := regexp.Compile(regexPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regex: %w", err)
	}
	return &RegexUserIDMapper{
		regex:       regex,
		replacement: replacement,
	}, nil
}

func (m *RegexUserIDMapper) Map(ctx context.Context, userID string) (string, error) {
	return m.regex.ReplaceAllString(userID, m.replacement), nil
}
