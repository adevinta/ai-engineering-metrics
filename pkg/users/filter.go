package users

import "fmt"

type UserList struct {
	Type   string                 `yaml:"type"`
	Config map[string]interface{} `yaml:"config"`
}

type UsersList interface {
	Include(userID string) bool
	List() []string
}

func NewUserList(cfg UserList) (UsersList, error) {
	switch cfg.Type {
	case "static":
		return NewStaticUserList(cfg.Config)
	default:
		return nil, fmt.Errorf("unknown filter type: %s", cfg.Type)
	}
}

type StaticUserFilter struct {
	userIDs map[string]struct{}
}

var _ UsersList = &StaticUserFilter{}

func NewStaticUserList(config map[string]any) (*StaticUserFilter, error) {
	userIDsInterface, ok := config["user_ids"].([]any)
	if !ok {
		return nil, fmt.Errorf("user_ids is not a list of strings")
	}

	userIDs := make(map[string]struct{})
	for _, userID := range userIDsInterface {
		userIDString, ok := userID.(string)
		if !ok {
			return nil, fmt.Errorf("user_ids is not a list of strings")
		}
		userIDs[userIDString] = struct{}{}
	}
	return &StaticUserFilter{userIDs: userIDs}, nil
}

func (f *StaticUserFilter) Include(userID string) bool {
	_, ok := f.userIDs[userID]
	return ok
}

func (f *StaticUserFilter) List() []string {
	userIDs := make([]string, 0, len(f.userIDs))
	for userID := range f.userIDs {
		userIDs = append(userIDs, userID)
	}
	return userIDs
}
