package dx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
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
