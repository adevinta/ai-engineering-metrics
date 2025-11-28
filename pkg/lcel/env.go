package lcel

import (
	"context"
	"fmt"
	"os"
	"strings"
)

func getenv() map[string]string {
	env := map[string]string{}
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	return env
}

func ExpandEnv(pattern string) (string, error) {
	if !isCELExpression(pattern) {
		return pattern, nil
	}
	pattern, err := rawPattern(pattern)
	if err != nil {
		return "", fmt.Errorf("failed to extract raw pattern: %w", err)
	}
	pgm, err := Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("failed to compile CEL program: %w", err)
	}
	return Eval[string](context.Background(), pgm)
}
