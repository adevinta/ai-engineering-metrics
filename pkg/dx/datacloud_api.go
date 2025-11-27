package dx

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type DatacloudAPIClient struct {
	dxClient
}

type DatacloudClientOption func(*DatacloudAPIClient)

func WithDatacloudAPIKey(apiKey string) DatacloudClientOption {
	return DatacloudClientOption(WithAPIKey[*DatacloudAPIClient](apiKey))
}

func WithDatacloudHTTPClient(httpClient *http.Client) DatacloudClientOption {
	return DatacloudClientOption(WithHTTPClient[*DatacloudAPIClient](httpClient))
}

func WithDatacloudAPIURL(apiURL string) DatacloudClientOption {
	return DatacloudClientOption(WithAPIURL[*DatacloudAPIClient](apiURL))
}

func NewDatacloudAPIClient(opts ...DatacloudClientOption) (*DatacloudAPIClient, error) {
	c := &DatacloudAPIClient{}
	c.dxClient.setHTTPClient(&http.Client{
		Timeout: 30 * time.Second,
	})
	for _, opt := range opts {
		opt(c)
	}
	err := c.dxClient.checkConfig()
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *DatacloudAPIClient) PushAIMetric(ctx context.Context, metric DXAIMetric) (DXAIMetricResponse, error) {
	resp := DXAIMetricResponse{}
	if err := c.dxClient.post(fmt.Sprintf("%s/api/aiToolMetrics.push", c.apiURL), nil, metric, &resp); err != nil {
		return DXAIMetricResponse{}, err
	}
	return resp, nil
}

func (c *DatacloudAPIClient) PushAIMetrics(ctx context.Context, metrics DXAIMetrics) (DXAIMetricsResponse, error) {
	resp := DXAIMetricsResponse{}
	if err := c.dxClient.post(fmt.Sprintf("%s/api/aiToolMetrics.pushAll", c.apiURL), nil, metrics, &resp); err != nil {
		return DXAIMetricsResponse{}, err
	}
	return resp, nil
}

type DXAIMetrics struct {
	Data []DXAIMetric `json:"data"`
}

// DXAIMetric represents a metric in GetDX format
type DXAIMetric struct {
	Email    string         `json:"email"`
	Date     string         `json:"date"`
	IsActive bool           `json:"is_active"`
	Tool     string         `json:"tool"`
	Metrics  map[string]any `json:"metrics"`
}

type DXAIMetricResponse struct {
	ID       string         `json:"id"`
	Email    string         `json:"email"`
	Date     string         `json:"date"`
	IsActive bool           `json:"is_active"`
	Tool     string         `json:"tool"`
	Metrics  map[string]any `json:"metrics"`
}

type DXAIMetricsResponse struct {
	Data []DXAIMetricResponse `json:"data"`
}
