package lcel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"
)

func p[T any]() T {
	return *new(T)
}

type ResolvedValue[T any] struct {
	pattern string
	raw     *T
	pgm     cel.Program
	pgms    map[string]cel.Program
}

func (r *ResolvedValue[T]) Resolve(ctx context.Context, data ...map[string]any) (T, error) {
	if r == nil {
		return *new(T), fmt.Errorf("resolved value is nil")
	}
	if r.raw != nil {
		return *r.raw, nil
	}
	switch any(*new(T)).(type) {
	// Only strings can have multiple patterns
	case string:
		for pattern, pgm := range r.pgms {
			val, _, err := pgm.ContextEval(ctx, Context(data...))
			if err != nil {
				return *new(T), fmt.Errorf("failed to evaluate CEL program: %w", err)
			}
			result, ok := val.Value().(string)
			if !ok {
				return *new(T), fmt.Errorf("failed to convert CEL program result to string: %v", val.Value())
			}
			r.pattern = strings.ReplaceAll(r.pattern, pattern, result)
		}
		return any(r.pattern).(T), nil
	default:
		if r.pgm == nil {
			return *new(T), fmt.Errorf("no CEL program bound")
		}
		val, _, err := r.pgm.ContextEval(ctx, Context(data...))
		if err != nil {
			return *new(T), fmt.Errorf("failed to evaluate CEL program: %w", err)
		}
		result, ok := val.Value().(T)
		if !ok {
			return *new(T), fmt.Errorf("failed to convert CEL program result to %T: %v", result, val.Value())
		}
		return result, nil
	}
}

func BindPtr[T any](pattern string, target **ResolvedValue[T], opts ...cel.EnvOption) error {
	if pattern == "" {
		return nil
	}
	if target == nil {
		return fmt.Errorf("target is nil")
	}
	if *target == nil {
		*target = &ResolvedValue[T]{}
	}
	return Bind(pattern, *target, opts...)
}

func Bind[T any](pattern string, target *ResolvedValue[T], opts ...cel.EnvOption) error {
	if pattern == "" {
		return nil
	}
	if target == nil {
		return fmt.Errorf("target is nil")
	}
	if !isCELExpression(pattern) {
		target.raw = new(T)
		switch any(*new(T)).(type) {
		case string:
			*target.raw = any(pattern).(T)
		default:
			// Try to consider a json. If it fails, it's likely
			// that it actually was a string. Strings has already be loaded
			// from json, so they are not serialized
			err := json.Unmarshal([]byte(pattern), target.raw)
			if err != nil {
				*target.raw = any(pattern).(T)
			}
		}
		return nil
	}

	switch any(*new(T)).(type) {
	case string:
		patterns, err := extractPatterns(pattern)
		if err != nil {
			return fmt.Errorf("failed to extract patterns: %w", err)
		}
		target.pattern = pattern
		target.pgms = make(map[string]cel.Program)
		for _, pattern := range patterns {
			pgm, err := Compile(pattern, opts...)
			if err != nil {
				return fmt.Errorf("failed to compile CEL program %s: %w", pattern, err)
			}
			target.pgms[strings.Join([]string{startPattern, pattern, string(endChar)}, "")] = pgm
		}
	default:
		pattern, err := rawPattern(pattern)
		if err != nil {
			return fmt.Errorf("failed to extract raw pattern: %w", err)
		}
		pgm, err := Compile(pattern, opts...)
		if err != nil {
			return fmt.Errorf("failed to compile CEL program %s: %w", pattern, err)
		}
		target.pgm = pgm
	}
	return nil
}

func extractPatterns(input string) ([]string, error) {
	var results []string
	n := len(input)
	i := 0

	for i < n {
		// Find the next "${"
		start := strings.Index(input[i:], "${")
		if start == -1 {
			break
		}

		// Position after "${"
		j := i + start + 2
		depth := 0
		foundEnd := false

		for j < n {
			ch := input[j]
			switch ch {
			case '{':
				depth++
			case '}':

				if depth == 0 {
					// This '}' closes the "${...}"
					pattern := input[i+start+2 : j]
					results = append(results, pattern)
					foundEnd = true
				}
				depth--
			}
			if foundEnd {
				break
			}
			j++
		}
		i = j + 1
		// If we didn't find a closing '}', stop to avoid infinite loop
		if !foundEnd {
			return nil, fmt.Errorf("failed to find closing '}' for pattern: %s", input)
		}
	}
	return results, nil
}
