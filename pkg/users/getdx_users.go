package users

import (
	"fmt"
	"strings"
	"sync"

	"github.com/adevinta/ai-engineering-metrics/pkg/dx"
)

type DXUsers struct {
	m     sync.Mutex
	users map[string]struct{}
}

var _ UsersList = &DXUsers{}

func (u *DXUsers) Include(userID string) bool {
	u.m.Lock()
	defer u.m.Unlock()
	_, ok := u.users[userID]
	return ok
}

func (u *DXUsers) List() []string {
	u.m.Lock()
	defer u.m.Unlock()
	userIDs := make([]string, 0, len(u.users))
	for userID := range u.users {
		userIDs = append(userIDs, userID)
	}
	return userIDs
}

func NewGetDXUsers(config map[string]any) (*DXUsers, error) {
	u := &DXUsers{
		users: make(map[string]struct{}),
	}
	apiKey, ok := config["api_key"].(string)
	if !ok {
		return nil, fmt.Errorf("api_key is not a string")
	}
	opts := []dx.WebClientOption{dx.WithWebAPIKey(apiKey)}
	anyApiURL, ok := config["api_url"]
	if ok {
		apiURL, ok := anyApiURL.(string)
		if !ok {
			return nil, fmt.Errorf("api_url is not a string")
		}
		opts = append(opts, dx.WithWebAPIURL(apiURL))
	}
	client, err := dx.NewWebAPIClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}
	users, err := getDXUsers(client)
	if err != nil {
		return nil, fmt.Errorf("failed to get DX users: %w", err)
	}
	u.users = users
	return u, nil
}

func getDXUsers(client *dx.WebAPIClient) (map[string]struct{}, error) {
	teams, err := client.ListTeams()
	if err != nil {
		return nil, fmt.Errorf("failed to list teams: %w", err)
	}
	users := make(map[string]struct{})
	for _, team := range teams {
		if team.Parent {
			continue
		}
		teamInfo, err := client.GetTeamInfo(team.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get team info: %w", err)
		}
		for _, contributor := range teamInfo.Contributors {
			users[strings.ToLower(contributor.Email)] = struct{}{}
		}
		users[strings.ToLower(teamInfo.Lead.Email)] = struct{}{}
	}
	return users, nil
}
