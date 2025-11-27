package dx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type WebAPIClient struct {
	apiKey     string
	apiURL     string
	httpClient *http.Client
}

type WebClientOption func(*WebAPIClient)

func WithHTTPClient(httpClient *http.Client) WebClientOption {
	return func(c *WebAPIClient) {
		c.httpClient = httpClient
	}
}

func WithAPIKey(apiKey string) WebClientOption {
	return func(c *WebAPIClient) {
		c.apiKey = apiKey
	}
}

func WithAPIURL(apiURL string) WebClientOption {
	return func(c *WebAPIClient) {
		c.apiURL = apiURL
	}
}

func NewClient(opts ...WebClientOption) (*WebAPIClient, error) {
	c := &WebAPIClient{
		apiURL: "https://api.getdx.com",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}

	if c.apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}
	if c.apiURL == "" {
		return nil, fmt.Errorf("api url is required")
	}
	if c.httpClient == nil {
		return nil, fmt.Errorf("http client is required")
	}
	return c, nil
}

type dxResponse interface {
	Success() bool
	Error() string
}

type listTeamsResponse struct {
	OK    bool   `json:"ok"`
	Err   string `json:"error"`
	Teams []Team `json:"teams"`
}

func (r *listTeamsResponse) Success() bool {
	return r.OK
}

func (r *listTeamsResponse) Error() string {
	return r.Err
}

type Team struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Ancestors     []string  `json:"ancestors"`
	ParentID      string    `json:"parent_id"`
	ManagerID     string    `json:"manager_id"`
	Parent        bool      `json:"parent"`
	LastChangedAt time.Time `json:"last_changed_at"`
	Contributors  int       `json:"contributors"`
	ReferenceID   string    `json:"reference_d"`
}

func (c *WebAPIClient) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	return c.httpClient.Do(req)
}

func (c *WebAPIClient) get(url string, values url.Values, v dxResponse) error {
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

func (c *WebAPIClient) ListTeams() ([]Team, error) {
	resp := listTeamsResponse{}
	if err := c.get(fmt.Sprintf("%s/teams.list", c.apiURL), nil, &resp); err != nil {
		return nil, err
	}
	return resp.Teams, nil
}

type TeamInfo struct {
	Lead         UserInfo   `json:"lead"`
	Contributors []UserInfo `json:"contributors"`
}

type UserInfo struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	Avatar         string `json:"avatar"`
	GitHubUsername string `json:"github_username"`
	Developer      bool   `json:"developer"`
	TimeZone       string `json:"tz"`
}

func (c *WebAPIClient) GetTeamInfo(teamID string) (TeamInfo, error) {
	resp := teamInfoResponse{}
	values := url.Values{}
	values.Add("team_id", teamID)
	if err := c.get(fmt.Sprintf("%s/teams.info", c.apiURL), values, &resp); err != nil {
		return TeamInfo{}, err
	}
	return resp.Team, nil
}

type teamInfoResponse struct {
	OK   bool     `json:"ok"`
	Err  string   `json:"error"`
	Team TeamInfo `json:"team"`
}

func (r *teamInfoResponse) Success() bool {
	return r.OK
}

func (r *teamInfoResponse) Error() string {
	return r.Err
}
