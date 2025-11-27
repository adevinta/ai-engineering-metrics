package dx

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type WebAPIClient struct {
	dxClient
}

type WebClientOption func(*WebAPIClient)

func WithWebAPIKey(apiKey string) WebClientOption {
	return WebClientOption(WithAPIKey[*WebAPIClient](apiKey))
}

func WithWebAPIURL(apiURL string) WebClientOption {
	return WebClientOption(WithAPIURL[*WebAPIClient](apiURL))
}

func WithWebHTTPClient(httpClient *http.Client) WebClientOption {
	return WebClientOption(WithHTTPClient[*WebAPIClient](httpClient))
}

func NewWebAPIClient(opts ...WebClientOption) (*WebAPIClient, error) {
	c := &WebAPIClient{}
	c.dxClient.setAPIURL("https://api.getdx.com")
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
