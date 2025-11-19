package mapper

import (
	"context"
	"regexp"
	"sync"
)

type Mapper interface {
	Map(ctx context.Context, userID string) (string, error)
}

// StaticMapper is a simple in-memory mapper that uses a static map
type StaticMapper struct {
	mappings map[string]string
	mu       sync.RWMutex
}

// NewStaticMapper creates a new static mapper with the given mappings
func NewStaticMapper(mappings map[string]string) *StaticMapper {
	return &StaticMapper{
		mappings: mappings,
	}
}

// Map converts a user ID from one format to another
func (m *StaticMapper) Map(ctx context.Context, userID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if mapped, ok := m.mappings[userID]; ok {
		return mapped, nil
	}

	// Return original if no mapping found
	return userID, nil
}

type RegexMapper struct {
	regex       *regexp.Regexp
	replacement string
}

var _ Mapper = &RegexMapper{}

func NewRegexMapper(regex *regexp.Regexp, replacement string) *RegexMapper {
	return &RegexMapper{
		regex:       regex,
		replacement: replacement,
	}
}

func (m *RegexMapper) Map(ctx context.Context, userID string) (string, error) {
	return m.regex.ReplaceAllString(userID, m.replacement), nil
}
