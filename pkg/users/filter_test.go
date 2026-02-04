package users

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAllowAllUserFilter(t *testing.T) {
	filter := NewAllowAllUserFilter()

	// Test that all users are included
	assert.True(t, filter.Include("user1@example.com"))
	assert.True(t, filter.Include("user2@company.org"))
	assert.True(t, filter.Include("repository-name"))
	assert.True(t, filter.Include("organization-name"))
	assert.True(t, filter.Include("any-string-at-all"))

	// Test that List() returns empty slice (since we allow all)
	list := filter.List()
	assert.Empty(t, list)
}

func TestNewUserListWithAllType(t *testing.T) {
	cfg := UserList{
		Type:   "all",
		Config: map[string]interface{}{},
	}

	userList, err := NewUserList(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, userList)

	// Verify it behaves like AllowAllUserFilter
	assert.True(t, userList.Include("any-user"))
	assert.Empty(t, userList.List())
}
