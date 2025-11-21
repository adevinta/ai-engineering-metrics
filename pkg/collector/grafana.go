package collector

// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"net/http"
// 	"net/url"
// 	"time"
// )

// // GrafanaCollector collects Claude metrics from Grafana/Prometheus
// type GrafanaCollector struct {
// 	baseURL    string
// 	apiKey     string
// 	datasource string
// 	queries    []PrometheusQuery
// 	httpClient *http.Client
// }

// // PrometheusQuery represents a Prometheus query configuration
// type PrometheusQuery struct {
// 	Name       string            // Friendly name for the query
// 	Query      string            // PromQL query
// 	MetricType string            // "token_usage", "cost", "user_activity", "model_usage"
// 	Labels     map[string]string // Additional labels to extract
// }

// // GrafanaConfig holds configuration for the Grafana collector
// type GrafanaConfig struct {
// 	BaseURL    string            `yaml:"base_url"`
// 	APIKey     string            `yaml:"api_key"`
// 	Datasource string            `yaml:"datasource"`
// 	Queries    []PrometheusQuery `yaml:"queries"`
// }

// // NewGrafanaCollector creates a new Grafana collector
// func NewGrafanaCollector(config GrafanaConfig) (*GrafanaCollector, error) {
// 	if config.BaseURL == "" {
// 		return nil, fmt.Errorf("base_url is required")
// 	}
// 	if config.APIKey == "" {
// 		return nil, fmt.Errorf("api_key is required")
// 	}
// 	if config.Datasource == "" {
// 		config.Datasource = "Prometheus" // Default datasource name
// 	}

// 	// Set default queries if none provided
// 	if len(config.Queries) == 0 {
// 		config.Queries = getDefaultQueries()
// 	}

// 	return &GrafanaCollector{
// 		baseURL:    config.BaseURL,
// 		apiKey:     config.APIKey,
// 		datasource: config.Datasource,
// 		queries:    config.Queries,
// 		httpClient: &http.Client{
// 			Timeout: 30 * time.Second,
// 		},
// 	}, nil
// }

// // getDefaultQueries returns default Prometheus queries for Claude metrics
// func getDefaultQueries() []PrometheusQuery {
// 	return []PrometheusQuery{
// 		{
// 			Name:       "token_usage",
// 			Query:      `sum by (user, model) (increase(claude_tokens_total[1h]))`,
// 			MetricType: "token_usage",
// 			Labels:     map[string]string{"type": "tokens"},
// 		},
// 		{
// 			Name:       "input_tokens",
// 			Query:      `sum by (user, model) (increase(claude_input_tokens_total[1h]))`,
// 			MetricType: "token_usage",
// 			Labels:     map[string]string{"type": "input_tokens"},
// 		},
// 		{
// 			Name:       "output_tokens",
// 			Query:      `sum by (user, model) (increase(claude_output_tokens_total[1h]))`,
// 			MetricType: "token_usage",
// 			Labels:     map[string]string{"type": "output_tokens"},
// 		},
// 		{
// 			Name:       "request_count",
// 			Query:      `sum by (user, model) (increase(claude_requests_total[1h]))`,
// 			MetricType: "user_activity",
// 			Labels:     map[string]string{"type": "requests"},
// 		},
// 		{
// 			Name:       "model_usage",
// 			Query:      `sum by (model) (increase(claude_requests_total[1h]))`,
// 			MetricType: "model_usage",
// 			Labels:     map[string]string{"type": "model_requests"},
// 		},
// 	}
// }

// // Name returns the name of the collector
// func (gc *GrafanaCollector) Name() string {
// 	return "grafana"
// }

// // Collect retrieves metrics from Grafana/Prometheus for the specified time range
// func (gc *GrafanaCollector) Collect(ctx context.Context, start, end time.Time) ([]Metric, error) {
// 	var allMetrics []Metric

// 	for _, query := range gc.queries {
// 		metrics, err := gc.executeQuery(ctx, query, start, end)
// 		if err != nil {
// 			// Log error but continue with other queries
// 			fmt.Printf("Error executing query %s: %v\n", query.Name, err)
// 			continue
// 		}
// 		allMetrics = append(allMetrics, metrics...)
// 	}

// 	return allMetrics, nil
// }

// // executeQuery executes a single Prometheus query via Grafana API
// func (gc *GrafanaCollector) executeQuery(ctx context.Context, query PrometheusQuery, start, end time.Time) ([]Metric, error) {
// 	// Build the Grafana datasource proxy URL
// 	queryURL := fmt.Sprintf("%s/api/datasources/proxy/uid/%s/api/v1/query_range",
// 		gc.baseURL, gc.datasource)

// 	// Prepare query parameters
// 	params := url.Values{}
// 	params.Add("query", query.Query)
// 	params.Add("start", fmt.Sprintf("%d", start.Unix()))
// 	params.Add("end", fmt.Sprintf("%d", end.Unix()))
// 	params.Add("step", "3600") // 1 hour step

// 	fullURL := fmt.Sprintf("%s?%s", queryURL, params.Encode())

// 	// Create HTTP request
// 	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to create request: %w", err)
// 	}

// 	// Set authorization header
// 	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", gc.apiKey))
// 	req.Header.Set("Content-Type", "application/json")

// 	// Execute request
// 	resp, err := gc.httpClient.Do(req)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to execute request: %w", err)
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		body, _ := io.ReadAll(resp.Body)
// 		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
// 	}

// 	// Parse response
// 	var promResponse PrometheusResponse
// 	if err := json.NewDecoder(resp.Body).Decode(&promResponse); err != nil {
// 		return nil, fmt.Errorf("failed to decode response: %w", err)
// 	}

// 	if promResponse.Status != "success" {
// 		return nil, fmt.Errorf("query failed: %s", promResponse.Status)
// 	}

// 	// Convert Prometheus results to metrics
// 	return gc.convertToMetrics(promResponse, query)
// }

// // convertToMetrics converts Prometheus query results to our Metric type
// func (gc *GrafanaCollector) convertToMetrics(response PrometheusResponse, query PrometheusQuery) ([]Metric, error) {
// 	var metrics []Metric

// 	for _, result := range response.Data.Result {
// 		// Extract labels
// 		user := result.Metric["user"]
// 		model := result.Metric["model"]

// 		// Process each timestamp-value pair
// 		for _, value := range result.Values {
// 			timestamp := time.Unix(int64(value[0].(float64)), 0)

// 			var metricValue float64
// 			switch v := value[1].(type) {
// 			case string:
// 				fmt.Sscanf(v, "%f", &metricValue)
// 			case float64:
// 				metricValue = v
// 			default:
// 				continue
// 			}

// 			// Create metric
// 			metric := Metric{
// 				Timestamp: timestamp,
// 				UserID:    user,
// 				Model:     model,
// 				Metadata:  make(map[string]interface{}),
// 			}

// 			// Add service and source to metadata
// 			metric.Metadata["service"] = "claude"
// 			metric.Metadata["source"] = "grafana"

// 			// Set metric-specific fields
// 			switch query.MetricType {
// 			case "token_usage":
// 				metric.TokensInput = int(metricValue)
// 				metric.TokensOutput = 0 // Will be set by specific queries
// 				if query.Labels["type"] == "input_tokens" {
// 					metric.TokensInput = int(metricValue)
// 				} else if query.Labels["type"] == "output_tokens" {
// 					metric.TokensOutput = int(metricValue)
// 					metric.TokensInput = 0
// 				}
// 			case "user_activity":
// 				metric.Metadata["request_count"] = int64(metricValue)
// 			case "model_usage":
// 				metric.Metadata["model_requests"] = int64(metricValue)
// 			}

// 			// Add query labels to metadata
// 			for k, v := range query.Labels {
// 				metric.Metadata[k] = v
// 			}

// 			// Add all Prometheus labels to metadata
// 			for k, v := range result.Metric {
// 				if k != "user" && k != "model" {
// 					metric.Metadata[k] = v
// 				}
// 			}

// 			// Calculate cost (placeholder - would need actual pricing)
// 			metric.Cost = calculateClaudeCost(metric.Model, metric.TokensInput, metric.TokensOutput)

// 			metrics = append(metrics, metric)
// 		}
// 	}

// 	return metrics, nil
// }

// // calculateClaudeCost calculates the cost for Claude usage
// // This is a placeholder - actual pricing should be configured
// func calculateClaudeCost(model string, inputTokens, outputTokens int) float64 {
// 	// Placeholder pricing per million tokens
// 	var inputPrice, outputPrice float64

// 	switch model {
// 	case "claude-3-opus-20240229":
// 		inputPrice = 15.0
// 		outputPrice = 75.0
// 	case "claude-3-sonnet-20240229":
// 		inputPrice = 3.0
// 		outputPrice = 15.0
// 	case "claude-3-haiku-20240307":
// 		inputPrice = 0.25
// 		outputPrice = 1.25
// 	default:
// 		// Default to Sonnet pricing
// 		inputPrice = 3.0
// 		outputPrice = 15.0
// 	}

// 	inputCost := (float64(inputTokens) / 1_000_000) * inputPrice
// 	outputCost := (float64(outputTokens) / 1_000_000) * outputPrice

// 	return inputCost + outputCost
// }

// // PrometheusResponse represents the response from Prometheus API
// type PrometheusResponse struct {
// 	Status string `json:"status"`
// 	Data   struct {
// 		ResultType string             `json:"resultType"`
// 		Result     []PrometheusResult `json:"result"`
// 	} `json:"data"`
// }

// // PrometheusResult represents a single result from Prometheus
// type PrometheusResult struct {
// 	Metric map[string]string `json:"metric"`
// 	Values [][]interface{}   `json:"values"`
// }
