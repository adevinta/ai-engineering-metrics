package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/adevinta/ai-engineering-metrics/pkg/dx"
	"github.com/adevinta/ai-engineering-metrics/pkg/lcel"
	"github.com/google/cel-go/cel"
	"github.com/google/go-github/v75/github"
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
	body, err := h.getPayload(r)
	if err != nil {
		fmt.Printf("error getting payload: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	event := map[string]any{}
	err = json.Unmarshal(body, &event)
	if err != nil {
		fmt.Printf("error unmarshalling body: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	header := make(http.Header)
	for k, v := range r.Header {
		header[k] = v
	}

	ghCtx := &githubWebhookContext{
		Method:  r.Method,
		URL:     r.URL.String(),
		Headers: r.Header,
		Event:   event,
		rawBody: body,
	}

	include, err := shouldInclude(context.Background(), h.GitHubHandler.If, ghCtx)
	if err != nil {
		fmt.Printf("error evaluating if expression: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !include {
		fmt.Printf("Skipping event %s because if expression is false\n", ghCtx.Headers.Get(github.EventTypeHeader))
		return
	}

	errgroup := errgroup.Group{}
	for _, trackEvent := range h.GitHubHandler.DX.TrackEvent {
		errgroup.Go(func() error {
			return h.sendEventToDX(r.Context(), trackEvent, ghCtx)
		})
	}
	for _, passThru := range h.GitHubHandler.PassThru {
		errgroup.Go(func() error {
			return h.sendRawRequestToDX(r.Context(), passThru, ghCtx)
		})
	}
	err = errgroup.Wait()
	if err != nil {
		log.Printf("error processing event: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (h *Handler) sendRawRequestToDX(ctx context.Context, passThru *GitHubPassThruTarget, ghCtx *githubWebhookContext) error {
	method, err := passThru.Method.Resolve(ctx, ghCtx.Data())
	if err != nil {
		return err
	}
	url, err := passThru.URL.Resolve(ctx, ghCtx.Data())
	if err != nil {
		return err
	}
	include, err := shouldInclude(ctx, passThru.If, ghCtx)
	if err != nil {
		return err
	}
	if !include {
		return nil
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(ghCtx.rawBody))
	if err != nil {
		return err
	}
	req.Header = ghCtx.Headers
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	log.Printf("response from DX: status: %d, body: %s", resp.StatusCode, string(data))
	return nil
}

func (h *Handler) sendEventToDX(ctx context.Context, trackEvent GitHubTrackEventTarget, ghCtx *githubWebhookContext) error {

	include, err := shouldInclude(ctx, trackEvent.If, ghCtx)
	if err != nil {
		return fmt.Errorf("invalid include expression: %w", err)
	}
	if !include {
		return nil
	}

	isTest := false
	if trackEvent.IsTest != nil {
		v, err := trackEvent.IsTest.Resolve(ctx, ghCtx.Data())
		if err != nil {
			return fmt.Errorf("invalid is test expression: %w", err)
		}
		isTest = v
	}

	eventName, err := trackEvent.EventName.Resolve(ctx, ghCtx.Data())
	if err != nil {
		return fmt.Errorf("invalid event name expression: %w", err)
	}

	userName, err := trackEvent.UserName.Resolve(ctx, ghCtx.Data())
	if err != nil {
		return fmt.Errorf("invalid user name expression: %w", err)
	}

	metadata := make(map[string]any)
	for k, v := range trackEvent.EventMetadata {
		val, err := v.Resolve(ctx, ghCtx.Data())
		if err != nil {
			return fmt.Errorf("invalid metadata expression for %s: %w", k, err)
		}
		metadata[k] = val
	}

	_, err = trackEvent.WebAPIClient.TrackEvent(
		dx.WithEventName(eventName),
		dx.WithEventGitHubUserName(userName),
		dx.WithEventTimestamp(getCreatedAt(ghCtx)),
		dx.WithEventTestData(isTest),
		dx.WithEventMetadata(metadata),
	)

	if err != nil {
		return err
	}

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
