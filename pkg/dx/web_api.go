package dx

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
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

type Event struct {
	Name           string         `json:"name"`
	Timestamp      string         `json:"timestamp"`
	Email          string         `json:"email,omitempty"`
	GitHubUserName string         `json:"github_username,omitempty"`
	GitLabUserName string         `json:"gitlab_username,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type event struct {
	payload Event
	values  url.Values
}

type EventArg func(event *event)

func WithEventEmail(email string) EventArg {
	return func(event *event) {
		event.payload.Email = email
	}
}

func WithEventGitHubUserName(githubUserName string) EventArg {
	return func(event *event) {
		event.payload.GitHubUserName = githubUserName
	}
}

func WithEventGitLabUserName(gitlabUserName string) EventArg {
	return func(event *event) {
		event.payload.GitLabUserName = gitlabUserName
	}
}

func WithEventName(name string) EventArg {
	return func(event *event) {
		event.payload.Name = name
	}
}

func WithEventTimestamp(t time.Time) EventArg {
	return func(event *event) {
		event.payload.Timestamp = strconv.Itoa(int(t.Unix()))
	}
}

func WithEventMetadata(metadata map[string]any) EventArg {
	return func(event *event) {
		event.payload.Metadata = metadata
	}
}

func WithEventTestData(testData bool) EventArg {
	return func(event *event) {
		if testData {
			event.values.Set("test_data", "true")
		} else {
			event.values.Del("test_data")
		}
	}
}

func (c *WebAPIClient) TrackEvent(args ...EventArg) (*EventResponse, error) {
	evt := event{
		payload: Event{
			Timestamp: strconv.Itoa(int(time.Now().Unix())),
		},
		values: url.Values{},
	}
	for _, arg := range args {
		arg(&evt)
	}
	if evt.payload.Email == "" && evt.payload.GitHubUserName == "" && evt.payload.GitLabUserName == "" {
		return nil, fmt.Errorf("email or github_username or gitlab_username is required")
	}
	if evt.payload.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if evt.payload.Timestamp == "" {
		return nil, fmt.Errorf("timestamp is required")
	}
	resp := EventResponse{}
	if err := c.post(fmt.Sprintf("%s/events.track", c.apiURL), evt.values, evt.payload, &resp); err != nil {
		return nil, err
	}
	if !resp.Success() {
		return &resp, &resp
	}
	return &resp, nil
}

type EventResponse struct {
	OK  bool   `json:"ok"`
	Err string `json:"error"`
}

func (r *EventResponse) Success() bool {
	return r.OK
}

func (r *EventResponse) Error() string {
	return r.Err
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
