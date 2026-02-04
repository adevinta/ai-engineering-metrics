package dx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/adevinta/ai-engineering-metrics/pkg/logging"
	"github.com/sirupsen/logrus"
)

type dxClientConfig interface {
	setAPIKey(apiKey string)
	setAPIURL(apiURL string)
	setHTTPClient(httpClient *http.Client)
}

type dxClient struct {
	apiKey     string
	apiURL     string
	httpClient *http.Client
}

func (c *dxClient) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	return c.httpClient.Do(req)
}

func (c *dxClient) get(url string, values url.Values, v dxResponse) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	if values != nil {
		req.URL.RawQuery = values.Encode()
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return err
	}
	if !v.Success() {
		return fmt.Errorf("failed to list teams: %s", v.Error())
	}
	return nil
}

func (c *dxClient) post(url string, values url.Values, input, out any) error {
	data, err := json.Marshal(input)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if values != nil {
		req.URL.RawQuery = values.Encode()
	}

	// Log the outgoing request
	logger := logging.LoggerFromCtx(req.Context())
	logger.WithFields(logrus.Fields{
		"method":       "POST",
		"url":          url,
		"content_type": "application/json",
		"payload_size": len(data),
	}).Info("sending dx api request")

	// Debug log the request payload
	logger.WithField("request_payload", string(data)).Debug("dx api request payload")

	requestStart := time.Now()
	resp, err := c.Do(req)
	duration := time.Since(requestStart)

	if err != nil {
		logger.WithError(err).WithField("duration_ms", duration.Milliseconds()).Error("dx api request failed")
		return err
	}
	defer resp.Body.Close()

	logger.WithFields(logrus.Fields{
		"status_code": resp.StatusCode,
		"duration_ms": duration.Milliseconds(),
	}).Info("received dx api response")

	if resp.StatusCode != http.StatusOK {
		// Read the response body for debugging
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.WithError(err).Error("failed to read error response body")
		} else {
			logger.WithFields(logrus.Fields{
				"status_code":   resp.StatusCode,
				"response_body": string(body),
			}).Error("dx api returned non-200 status")
		}
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		logger.WithError(err).Error("failed to decode dx api response")
		return err
	}

	logger.Debug("dx api response decoded successfully")
	return nil
}

func (c *dxClient) setAPIKey(apiKey string) {
	c.apiKey = apiKey
}

func (c *dxClient) setAPIURL(apiURL string) {
	c.apiURL = apiURL
}

func (c *dxClient) setHTTPClient(httpClient *http.Client) {
	c.httpClient = httpClient
}

func (c *dxClient) checkConfig() error {
	if c.apiKey == "" {
		return fmt.Errorf("api key is required")
	}
	if c.apiURL == "" {
		return fmt.Errorf("api url is required")
	}
	if c.httpClient == nil {
		return fmt.Errorf("http client is required")
	}
	return nil
}

type dxClientOption[T dxClientConfig] func(T)

func WithHTTPClient[T dxClientConfig](httpClient *http.Client) dxClientOption[T] {
	return func(c T) {
		c.setHTTPClient(httpClient)
	}
}

func WithAPIKey[T dxClientConfig](apiKey string) dxClientOption[T] {
	return func(c T) {
		c.setAPIKey(apiKey)
	}
}

func WithAPIURL[T dxClientConfig](apiURL string) dxClientOption[T] {
	return func(c T) {
		c.setAPIURL(apiURL)
	}
}
