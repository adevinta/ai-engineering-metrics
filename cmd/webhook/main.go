package main

import (
	"context"
	"flag"
	"net/http"

	"github.com/adevinta/ai-engineering-metrics/pkg/logging"
	"github.com/adevinta/ai-engineering-metrics/pkg/webhook"
	"github.com/sirupsen/logrus"
)

func main() {
	var (
		configFile = flag.String("config", "webhook.yaml", "Path to webhook configuration file")
		port       = flag.String("port", "8080", "Port to serve on")
	)
	flag.Parse()

	// Initialize structured logging
	logging.InitLogger()
	logger := logging.LoggerFromCtx(logging.WithLoggingFields(
		context.Background(),
		logrus.Fields{
			"component":   "webhook_main",
			"config_file": *configFile,
			"port":        *port,
		},
	))

	logger.Info("starting webhook server")

	// Load webhook configuration
	handler, err := webhook.NewHandler(webhook.FromConfigFile(*configFile))
	if err != nil {
		logger.WithError(err).Fatal("failed to create webhook handler")
	}

	// Start HTTP server
	addr := ":" + *port
	logger.WithField("address", addr).Info("webhook server starting")

	if err := http.ListenAndServe(addr, handler); err != nil {
		logger.WithError(err).Fatal("webhook server failed")
	}
}
