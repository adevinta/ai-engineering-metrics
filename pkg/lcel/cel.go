package lcel

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/google/cel-go/cel"
)

const (
	startPattern = "${"
	endChar      = '}'
)

func Options(opts ...cel.EnvOption) []cel.EnvOption {
	return append([]cel.EnvOption{
		cel.Variable("env", cel.MapType(cel.StringType, cel.AnyType)),
	}, opts...)
}

func merge(args ...map[string]any) map[string]any {
	r := map[string]any{}
	for _, m := range args {
		maps.Copy(r, m)
	}
	return r
}

func Context(data ...map[string]any) map[string]any {
	return merge(
		append(
			[]map[string]any{
				{
					"env": getenv(),
				},
			},
			data...,
		)...,
	)
}

func Compile(pattern string, opts ...cel.EnvOption) (cel.Program, error) {
	env, err := cel.NewEnv(Options(opts...)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}
	ast, issues := env.Compile(pattern)
	if issues.Err() != nil {
		return nil, fmt.Errorf("failed to compile CEL program: %w", issues.Err())
	}
	return env.Program(ast)
}

func Eval[T any](ctx context.Context, pgm cel.Program, data ...map[string]any) (T, error) {
	val, _, err := pgm.ContextEval(ctx, Context(data...))
	if err != nil {
		return *new(T), fmt.Errorf("failed to evaluate CEL program: %w", err)
	}
	result, ok := val.Value().(T)
	if !ok {
		return *new(T), fmt.Errorf("failed to convert CEL program result to %T: %v", result, val.Value())
	}
	return result, nil
}

func isCELExpression(pattern string) bool {
	return strings.Contains(pattern, startPattern)
}

func rawPattern(pattern string) (string, error) {
	pattern = strings.TrimSpace(pattern)
	if !strings.HasPrefix(pattern, startPattern) || !strings.HasSuffix(pattern, string(endChar)) {
		return "", fmt.Errorf("invalid pattern: %s", pattern)
	}
	return pattern[len(startPattern) : len(pattern)-1], nil
}
