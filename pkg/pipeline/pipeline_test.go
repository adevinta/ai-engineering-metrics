package pipeline

import (
	"testing"

	"github.com/adevinta/ai-engineering-metrics/pkg/collector"
	"github.com/adevinta/ai-engineering-metrics/pkg/publisher"
	"github.com/adevinta/ai-engineering-metrics/pkg/users"
	"github.com/stretchr/testify/assert"
)

func TestPipelineWithOptionalUsers(t *testing.T) {
	tests := []struct {
		name            string
		config          PipelineConfig
		expectErr       bool
		checkUserFilter func(users.UsersList) bool
	}{
		{
			name: "pipeline with users field",
			config: PipelineConfig{
				Users: &users.UserList{
					Type:   "all",
					Config: map[string]interface{}{},
				},
				Collectors: []collector.CollectorConfig{},
				Publishers: []publisher.PublisherConfig{},
			},
			expectErr: false,
			checkUserFilter: func(ul users.UsersList) bool {
				return ul.Include("any-user")
			},
		},
		{
			name: "pipeline without users field (optional)",
			config: PipelineConfig{
				Users:      nil, // No users field
				Collectors: []collector.CollectorConfig{},
				Publishers: []publisher.PublisherConfig{},
			},
			expectErr: false,
			checkUserFilter: func(ul users.UsersList) bool {
				return ul.Include("any-user") // Should default to allow all
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline, err := NewPipeline(tt.config)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pipeline.Users)
				assert.True(t, tt.checkUserFilter(pipeline.Users))
			}
		})
	}
}
