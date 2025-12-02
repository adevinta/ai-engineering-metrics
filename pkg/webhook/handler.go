package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/adevinta/ai-engineering-metrics/pkg/dx"
	"github.com/adevinta/ai-engineering-metrics/pkg/lcel"
	"github.com/adevinta/ai-engineering-metrics/pkg/logging"
	"github.com/google/cel-go/cel"
	"github.com/google/go-github/v75/github"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type Handler struct {
	GitHubHandler
}

type GitHubHandler struct {
	webhookSecret string
	If            *lcel.ResolvedValue[bool]
	PassThru      []*GitHubPassThruTarget
	DX            *GitHubDXTarget
}

type GitHubPassThruTarget struct {
	If     *lcel.ResolvedValue[bool]
	URL    lcel.ResolvedValue[string]
	Method lcel.ResolvedValue[string]
}

type GitHubDXTarget struct {
	TrackEvent []GitHubTrackEventTarget
}

type GitHubTrackEventTarget struct {
	*dx.WebAPIClient
	IsTest        *lcel.ResolvedValue[bool]
	If            *lcel.ResolvedValue[bool]
	EventName     lcel.ResolvedValue[string]
	UserName      lcel.ResolvedValue[string]
	EventMetadata map[string]lcel.ResolvedValue[any]
}

type HandlerOption func(h *Handler) error

func WithWebhookSecret(webhookSecret string) HandlerOption {
	return func(h *Handler) error {
		h.webhookSecret = webhookSecret
		return nil
	}
}

type githubWebhookContext struct {
	Method  string
	URL     string
	Headers http.Header
	Event   map[string]any
	rawBody []byte
}

var (
	githubWebhookContextVariables = []cel.EnvOption{
		cel.Variable("headers", cel.MapType(cel.StringType, cel.ListType(cel.StringType))),
		cel.Variable("body", cel.MapType(cel.StringType, cel.AnyType)),
		cel.Variable("method", cel.StringType),
		cel.Variable("url", cel.StringType),
	}
)

func (c *githubWebhookContext) Data() map[string]any {
	return map[string]any{
		"headers": c.Headers,
		"body":    c.Event,
		"method":  c.Method,
		"url":     c.URL,
	}
}

func NewHandler(opts ...HandlerOption) (*Handler, error) {
	h := &Handler{}
	for _, opt := range opts {
		if err := opt(h); err != nil {
			return nil, err
		}
	}
	return h, nil
}

func (h *Handler) getPayload(r *http.Request) ([]byte, error) {
	if h.webhookSecret != "" {
		return github.ValidatePayload(r, []byte(h.webhookSecret))
	}
	return io.ReadAll(r.Body)

}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestStart := time.Now()
	fields := logrus.Fields{
		"component":      "webhook_handler",
		"method":         r.Method,
		"path":           r.URL.Path,
		"user_agent":     r.UserAgent(),
		"content_length": r.ContentLength,
	}
	for _, header := range []string{"X-GitHub-Event", "X-GitHub-Hook-Installation-Target-Type", "X-GitHub-Delivery"} {
		_, ok := r.Header[header]
		if !ok {
			continue
		}
		fields[header] = r.Header.Get(header)
	}
	// Add context logging
	ctx := logging.WithLoggingFields(r.Context(), fields)
	logger := logging.LoggerFromCtx(ctx)

	logger.Info("incoming webhook request")

	// Handle health check endpoints
	switch r.URL.Path {
	case "/health":
		h.handleLiveness(w, r)
		duration := time.Since(requestStart)
		logger.WithField("duration_ms", duration.Milliseconds()).Info("health check completed")
		return
	case "/ready":
		h.handleReadiness(w, r)
		duration := time.Since(requestStart)
		logger.WithField("duration_ms", duration.Milliseconds()).Info("readiness check completed")
		return
	}

	// Handle webhook requests
	body, err := h.getPayload(r)
	if err != nil {
		logger.WithError(err).Error("error getting payload")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logger.WithField("payload_size", len(body)).Debug("payload received")

	event := map[string]any{}
	err = json.Unmarshal(body, &event)
	if err != nil {
		logger.WithError(err).Error("error unmarshalling body")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logger.WithField("payload_keys", getTopLevelKeys(event)).Info("webhook event parsed")

	ghCtx := &githubWebhookContext{
		Method:  r.Method,
		URL:     r.URL.String(),
		Headers: r.Header,
		Event:   event,
		rawBody: body,
	}

	include, err := shouldInclude(ctx, h.GitHubHandler.If, ghCtx)
	if err != nil {
		logger.WithError(err).Error("error evaluating if expression")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !include {
		logger.Info("skipping event due to if expression")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		return
	}

	logger.WithFields(logrus.Fields{
		"track_events":      len(h.GitHubHandler.DX.TrackEvent),
		"pass_thru_targets": len(h.GitHubHandler.PassThru),
	}).Info("processing webhook event")

	errgroup := errgroup.Group{}
	for _, trackEvent := range h.GitHubHandler.DX.TrackEvent {
		errgroup.Go(func() error {
			return h.sendEventToDX(ctx, trackEvent, ghCtx)
		})
	}
	for _, passThru := range h.GitHubHandler.PassThru {
		errgroup.Go(func() error {
			return h.sendRawRequestToDX(ctx, passThru, ghCtx)
		})
	}

	err = errgroup.Wait()
	duration := time.Since(requestStart)

	if err != nil {
		logger.WithError(err).WithField("duration_ms", duration.Milliseconds()).Error("error processing event")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logger.WithField("duration_ms", duration.Milliseconds()).Info("webhook request completed successfully")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (h *Handler) sendRawRequestToDX(ctx context.Context, passThru *GitHubPassThruTarget, ghCtx *githubWebhookContext) error {
	logger := logging.LoggerFromCtx(ctx)

	method, err := passThru.Method.Resolve(ctx, ghCtx.Data())
	if err != nil {
		logger.WithError(err).Error("failed to resolve pass-through method")
		return err
	}
	url, err := passThru.URL.Resolve(ctx, ghCtx.Data())
	if err != nil {
		logger.WithError(err).Error("failed to resolve pass-through URL")
		return err
	}

	include, err := shouldInclude(ctx, passThru.If, ghCtx)
	if err != nil {
		logger.WithError(err).Error("failed to evaluate pass-through include expression")
		return err
	}
	if !include {
		logger.Debug("skipping pass-through due to include expression")
		return nil
	}

	logger = logger.WithFields(logrus.Fields{
		"method":       method,
		"url":          url,
		"payload_size": len(ghCtx.rawBody),
	})
	logger.Info("forwarding raw request to dx")

	req, err := http.NewRequest(method, url, bytes.NewReader(ghCtx.rawBody))
	if err != nil {
		logger.WithError(err).Error("failed to create pass-through request")
		return err
	}
	req.Header = ghCtx.Headers

	requestStart := time.Now()
	resp, err := http.DefaultClient.Do(req)
	duration := time.Since(requestStart)

	if err != nil {
		logger.WithError(err).WithField("duration_ms", duration.Milliseconds()).Error("pass-through request failed")
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.WithError(err).Error("failed to read pass-through response")
		return err
	}

	logger.WithFields(logrus.Fields{
		"status_code":   resp.StatusCode,
		"duration_ms":   duration.Milliseconds(),
		"response_size": len(data),
		"response_body": string(data),
	}).Info("pass-through request completed")

	return nil
}

func (h *Handler) sendEventToDX(ctx context.Context, trackEvent GitHubTrackEventTarget, ghCtx *githubWebhookContext) error {
	logger := logging.LoggerFromCtx(ctx)

	include, err := shouldInclude(ctx, trackEvent.If, ghCtx)
	if err != nil {
		logger.WithError(err).Error("invalid include expression")
		return fmt.Errorf("invalid include expression: %w", err)
	}
	if !include {
		logger.Debug("skipping event due to include expression")
		return nil
	}

	isTest := false
	if trackEvent.IsTest != nil {
		v, err := trackEvent.IsTest.Resolve(ctx, ghCtx.Data())
		if err != nil {
			logger.WithError(err).Error("invalid is test expression")
			return fmt.Errorf("invalid is test expression: %w", err)
		}
		isTest = v
	}

	eventName, err := trackEvent.EventName.Resolve(ctx, ghCtx.Data())
	if err != nil {
		logger.WithError(err).Error("invalid event name expression")
		return fmt.Errorf("invalid event name expression: %w", err)
	}

	userName, err := trackEvent.UserName.Resolve(ctx, ghCtx.Data())
	if err != nil {
		logger.WithError(err).Error("invalid user name expression")
		return fmt.Errorf("invalid user name expression: %w", err)
	}

	metadata := make(map[string]any)
	for k, v := range trackEvent.EventMetadata {
		val, err := v.Resolve(ctx, ghCtx.Data())
		if err != nil {
			logger.WithError(err).WithField("metadata_key", k).Error("invalid metadata expression")
			return fmt.Errorf("invalid metadata expression for %s: %w", k, err)
		}
		metadata[k] = val
	}

	eventTimestamp := getCreatedAt(ghCtx)
	logger = logger.WithFields(logrus.Fields{
		"event_name":      eventName,
		"user_name":       userName,
		"is_test":         isTest,
		"event_timestamp": eventTimestamp,
		"metadata":        metadata,
	})
	logger.Info("sending event to dx")

	_, err = trackEvent.WebAPIClient.TrackEvent(
		dx.WithEventName(eventName),
		dx.WithEventGitHubUserName(userName),
		dx.WithEventTimestamp(eventTimestamp),
		dx.WithEventTestData(isTest),
		dx.WithEventMetadata(metadata),
	)

	if err != nil {
		logger.WithError(err).Error("failed to send event to dx")
		return err
	}

	logger.Info("successfully sent event to dx")
	return nil
}

func getCreatedAt(ghCtx *githubWebhookContext) time.Time {
	for _, candidate := range github.MessageTypes() {
		v, ok := ghCtx.Event[candidate]
		if !ok {
			continue
		}
		val, ok := v.(map[string]any)
		if !ok {
			continue
		}
		createdAt, ok := val["created_at"]
		if !ok {
			continue
		}
		createdAtStr, ok := createdAt.(string)
		if !ok {
			continue
		}
		t, err := time.Parse(time.RFC3339, createdAtStr)
		if err != nil {
			continue
		}
		return t
	}
	return time.Now().UTC()
}

func shouldInclude(ctx context.Context, resolver *lcel.ResolvedValue[bool], ghCtx *githubWebhookContext) (bool, error) {
	if resolver == nil {
		return true, nil
	}

	include, err := resolver.Resolve(ctx, ghCtx.Data())
	if err != nil {
		return false, err
	}
	return include, nil
}

func getTopLevelKeys(event map[string]any) []string {
	keys := make([]string, 0, len(event))
	for k := range event {
		keys = append(keys, k)
	}
	return keys
}

// handleLiveness handles the liveness probe endpoint
func (h *Handler) handleLiveness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "alive", "timestamp": "` + time.Now().UTC().Format(time.RFC3339) + `"}`))
}

// handleReadiness handles the readiness probe endpoint
func (h *Handler) handleReadiness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Basic readiness check - we could add more sophisticated checks here
	// such as checking external dependencies (DX API, database connections, etc.)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "ready", "timestamp": "` + time.Now().UTC().Format(time.RFC3339) + `"}`))
}
