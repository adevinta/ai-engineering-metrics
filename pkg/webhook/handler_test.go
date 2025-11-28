package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	plainConfig = `
github:
  webhook_secret: secret
  if: true
  pass_thru:
  - url: https://static-namespace.getdx.net/webhooks/github
    method: POST
  dx:
    track_event:
    - api_key: apikey
      if: true
      event_name: github.pull_request.static
      user_name: slug
      is_test: false
      event_metadata:
        repository: repository
        pull_request: 5678
        pull_request_url: https://github.com/owner/repo/pull/5678
        pull_request_title: title`
	celConfig = `
github:
  webhook_secret: ${env.GITHUB_WEBHOOK_SECRET}
  if: ${headers["X-GitHub-Event"][0] == "pull_request" || headers["X-GitHub-Event"][0] == "pull_request_review"}
  pass_thru:
  - if: ${method == "POST"}
    url: https://${env.NAMESPACE}.getdx.net/webhooks/github
    method: ${method}
  dx:
    track_event:
    - api_key: ${env.GETDX_API_KEY}
      if: ${body.action == "opened"}
      event_name: github.pull_request.${body.action}
      user_name: ${body.sender.login}
      is_test: ${body.action == "opened"}
      event_metadata:
        repository: ${body.repository.name}
        pull_request: ${body.pull_request.number}
        pull_request_url: ${body.pull_request.html_url}
        pull_request_title: ${body.pull_request.title}`
)

func p[T any](v T) *T {
	return &v
}

type transportFunc func(req *http.Request) (*http.Response, error)

func (t transportFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return t(req)
}

func TestWebhookHandlergWithCELExpressions(t *testing.T) {
	t.Setenv("GITHUB_WEBHOOK_SECRET", "secret")
	t.Setenv("GETDX_API_KEY", "apikey")
	t.Setenv("NAMESPACE", "some-namespace")

	transport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = transport
	})
	calls := 0

	var cfg WebhookConfig
	err := yaml.Unmarshal([]byte(celConfig), &cfg)
	require.NoError(t, err)

	handler := &Handler{}
	err = populateGitHubHandler(&handler.GitHubHandler, cfg.GithubConfig)
	require.NoError(t, err)
	require.Len(t, handler.GitHubHandler.DX.TrackEvent, 1)
	require.Len(t, handler.GitHubHandler.PassThru, 1)

	http.DefaultTransport = transportFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		assert.Equal(t, "https://api.getdx.com/events.track?test_data=true", req.URL.String())
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		var event map[string]any
		err = json.Unmarshal(body, &event)
		require.NoError(t, err)
		assert.Equal(t, map[string]any{
			"github_username": "some-user",
			"name":            "github.pull_request.opened",
			"timestamp":       "1609459200",
			"metadata": map[string]any{
				"repository":         "some-repo",
				"pull_request":       float64(123),
				"pull_request_url":   "https://github.com/owner/repo/pull/123",
				"pull_request_title": "some-title",
			},
		}, event)
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(`{"ok": true}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})
	require.NoError(t, handler.sendEventToDX(context.Background(), handler.GitHubHandler.DX.TrackEvent[0], &githubWebhookContext{
		Headers: http.Header{
			"X-GitHub-Event": []string{"pull_request"},
		},
		Event: map[string]any{
			"action": "opened",
			"pull_request": map[string]any{
				"number":     123,
				"title":      "some-title",
				"html_url":   "https://github.com/owner/repo/pull/123",
				"created_at": "2021-01-01T00:00:00Z",
			},
			"repository": map[string]any{
				"name": "some-repo",
			},
			"sender": map[string]any{
				"login": "some-user",
			},
		},
	}))
	assert.Equal(t, 1, calls)

	t.Setenv("NAMESPACE", "some-namespace")

	calls = 0
	http.DefaultTransport = transportFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		assert.Equal(t, "https://some-namespace.getdx.net/webhooks/github", req.URL.String())
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		assert.Equal(t, "body", string(body))
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(`{"ok": true}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})
	require.NoError(t, handler.sendRawRequestToDX(context.Background(), handler.GitHubHandler.PassThru[0], &githubWebhookContext{
		Headers: http.Header{
			"X-GitHub-Event": []string{"pull_request"},
		},
		Method:  "POST",
		URL:     "https://some-namespace.getdx.net/webhooks/github",
		rawBody: []byte(`body`),
	}))
	assert.Equal(t, 1, calls)
}

func TestWebhookHandlerWithPlainValues(t *testing.T) {

	var cfg WebhookConfig
	err := yaml.Unmarshal([]byte(plainConfig), &cfg)
	require.NoError(t, err)

	handler := &Handler{}
	err = populateGitHubHandler(&handler.GitHubHandler, cfg.GithubConfig)
	require.NoError(t, err)

	require.Len(t, handler.GitHubHandler.DX.TrackEvent, 1)
	require.Len(t, handler.GitHubHandler.PassThru, 1)

	calls := 0
	http.DefaultTransport = transportFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		assert.Equal(t, "https://api.getdx.com/events.track", req.URL.String())
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		var event map[string]any
		err = json.Unmarshal(body, &event)
		require.NoError(t, err)
		assert.Equal(t, map[string]any{
			"github_username": "slug",
			"name":            "github.pull_request.static",
			"timestamp":       "1612137600",
			"metadata": map[string]any{
				"repository":         "repository",
				"pull_request":       float64(5678),
				"pull_request_url":   "https://github.com/owner/repo/pull/5678",
				"pull_request_title": "title",
			},
		}, event)
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(`{"ok": true}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})
	require.NoError(t, handler.sendEventToDX(context.Background(), handler.GitHubHandler.DX.TrackEvent[0], &githubWebhookContext{
		Headers: http.Header{},
		Event: map[string]any{
			"pull_request": map[string]any{
				"created_at": "2021-02-01T00:00:00Z",
			},
		},
	}))
	assert.Equal(t, 1, calls)

	t.Setenv("NAMESPACE", "some-namespace")

	calls = 0
	http.DefaultTransport = transportFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		assert.Equal(t, "https://static-namespace.getdx.net/webhooks/github", req.URL.String())
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		assert.Equal(t, "body", string(body))
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(`{"ok": true}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})
	require.NoError(t, handler.sendRawRequestToDX(context.Background(), handler.GitHubHandler.PassThru[0], &githubWebhookContext{
		Headers: http.Header{
			"X-GitHub-Event": []string{"pull_request"},
		},
		Method:  "POST",
		URL:     "https://some-namespace.getdx.net/webhooks/github",
		rawBody: []byte(`body`),
	}))
	assert.Equal(t, 1, calls)
}
