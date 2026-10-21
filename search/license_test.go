// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package search

import (
	"testing"

	"github.com/mattermost/mattermost-plugin-agents/v2/embeddings"
	"github.com/mattermost/mattermost-plugin-agents/v2/embeddings/mocks"
	"github.com/mattermost/mattermost-plugin-agents/v2/enterprise"
	"github.com/mattermost/mattermost-plugin-agents/v2/enterprise/enterprisetest"
	mmapimocks "github.com/mattermost/mattermost-plugin-agents/v2/mmapi/mocks"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSearchServiceAtEveryLicenseLevel(t *testing.T) {
	checkers := map[string]*enterprise.LicenseChecker{"nil checker": nil}
	for _, level := range enterprisetest.AllLevels {
		checkers[level.String()] = enterprisetest.CheckerAt(level)
	}
	for name, checker := range checkers {
		t.Run(name, func(t *testing.T) {
			me := mocks.NewMockEmbeddingSearch(t)
			mm := mmapimocks.NewMockClient(t)
			allowVectorIndexStateRead(mm)
			mm.On("GetChannel", "channel-id").Return(&model.Channel{Id: "channel-id", DisplayName: "General"}, nil)
			mm.On("GetUser", "author-id").Return(&model.User{Id: "author-id", Username: "author"}, nil)
			me.On("Search", mock.Anything, "hello", embeddings.SearchOptions{
				Limit: 5, Offset: 2, TeamID: "team-id", ChannelID: "channel-id", UserID: "requester-id",
			}).Return([]embeddings.SearchResult{{
				Document: embeddings.PostDocument{PostID: "post-id", ChannelID: "channel-id", UserID: "author-id", Content: "matching post"},
				Score:    0.9,
			}}, nil).Once()

			s := New(func() embeddings.EmbeddingSearch { return me }, mm, checker)
			require.True(t, s.Enabled())
			results, err := s.Search(t.Context(), "hello", Options{
				Limit: 5, Offset: 2, TeamID: "team-id", ChannelID: "channel-id", UserID: "requester-id",
			})
			require.NoError(t, err)
			require.Len(t, results, 1)
			require.Equal(t, "post-id", results[0].PostID)
			require.Equal(t, "matching post", results[0].Content)
			require.Equal(t, "General", results[0].ChannelName)
			require.Equal(t, "author", results[0].Username)
		})
	}
}

func TestInitEmbeddingsSearchStillRequiresConfigurationAtEveryLevel(t *testing.T) {
	checkers := map[string]*enterprise.LicenseChecker{"nil checker": nil}
	for _, level := range enterprisetest.AllLevels {
		checkers[level.String()] = enterprisetest.CheckerAt(level)
	}
	for name, checker := range checkers {
		t.Run(name, func(t *testing.T) {
			result, err := InitEmbeddingsSearch(nil, embeddings.EmbeddingSearchConfig{}, checker, false)
			require.NoError(t, err)
			require.Nil(t, result, "unconfigured semantic search must stay disabled")

			result, err = InitEmbeddingsSearch(nil, embeddings.EmbeddingSearchConfig{
				Type: embeddings.SearchTypeComposite, Dimensions: 1536,
			}, checker, false)
			require.ErrorContains(t, err, "unsupported vector store type")
			require.Nil(t, result, "a license must not replace required vector-store configuration")
		})
	}
}
