package logging

import (
	"context"

	"github.com/sirupsen/logrus"
)

type contextKey string

const loggerKey contextKey = "logger"

// WithLoggingFields adds logging fields to the context
func WithLoggingFields(ctx context.Context, fields logrus.Fields) context.Context {
	logger := LoggerFromCtx(ctx).WithFields(fields)
	return context.WithValue(ctx, loggerKey, logger)
}

// WithLoggingField adds a single logging field to the context
func WithLoggingField(ctx context.Context, key string, value interface{}) context.Context {
	logger := LoggerFromCtx(ctx).WithField(key, value)
	return context.WithValue(ctx, loggerKey, logger)
}

// LoggerFromCtx returns a logger from context or creates a default one
func LoggerFromCtx(ctx context.Context) *logrus.Entry {
	if logger, ok := ctx.Value(loggerKey).(*logrus.Entry); ok {
		return logger
	}
	// Return default logger
	return logrus.NewEntry(logrus.StandardLogger())
}

// InitLogger sets up the default logger
func InitLogger() {
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetLevel(logrus.InfoLevel)
}