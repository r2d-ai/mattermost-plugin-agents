// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package conversations

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/mattermost/mattermost-plugin-agents/v2/conversation"
	"github.com/mattermost/mattermost-plugin-agents/v2/enterprise"
	"github.com/mattermost/mattermost-plugin-agents/v2/enterprise/enterprisetest"
	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
	"github.com/mattermost/mattermost-plugin-agents/v2/mmapi"
	"github.com/mattermost/mattermost-plugin-agents/v2/mmapi/mocks"
	"github.com/mattermost/mattermost-plugin-agents/v2/prompts"
	"github.com/mattermost/mattermost-plugin-agents/v2/store"
	"github.com/mattermost/mattermost-plugin-agents/v2/streaming"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type summaryRegenerationStreaming struct {
	regenNoopStreaming
	calls int
	text  string
	err   error
}

func (s *summaryRegenerationStreaming) StreamToPost(_ context.Context, result *llm.TextStreamResult, _ *model.Post, _, _ string) {
	s.calls++
	s.text, s.err = result.ReadAll()
}

func TestHandleRegenerateFreeSummariesPreservesAuthorization(t *testing.T) {
	tests := []struct {
		name       string
		thread     bool
		nilChecker bool
		denyRead   bool
		wrongOwner bool
		wantError  string
	}{
		{name: "thread summary", thread: true},
		{name: "thread summary without checker", thread: true, nilChecker: true},
		{name: "thread source access revoked", thread: true, denyRead: true, wantError: ErrAnalysisSourceUnavailable.Error()},
		{name: "thread summary belongs to another user", thread: true, wrongOwner: true, wantError: "only the original requester"},
		{name: "channel summary", wantError: ErrNotRegenerable.Error()},
		{name: "channel summary without checker", nilChecker: true, wantError: ErrNotRegenerable.Error()},
		{name: "channel summary belongs to another user", wrongOwner: true, wantError: "only the original requester"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			convStore, conv := loadedStateConversationStore()
			c, lm, _ := toolLicenseConversationsAt(t, convStore, enterprise.LevelUnlicensed)
			if tc.nilChecker {
				c.licenseChecker = nil
			}
			promptSet, err := llm.NewPrompts(prompts.PromptsFolder)
			require.NoError(t, err)
			c.prompts = promptSet
			c.convService = conversation.NewService(convStore, promptSet, c.mmClient, c.bots)
			c.mmClient.(*mocks.MockClient).On("KVGet", "user_tool_providers_"+testRequesterID, mock.Anything).Return(mmapi.ErrKVNotFound).Maybe()
			streamer := &summaryRegenerationStreaming{}
			c.streamingService = streamer

			userPostID := "question-post"
			c.mmClient.(*mocks.MockClient).On("GetPost", userPostID).Return(&model.Post{Id: userPostID, UserId: testRequesterID, ChannelId: "dm-channel"}, nil).Maybe()
			post := &model.Post{Id: "summary-post", UserId: "bot-id", ChannelId: "dm-channel"}
			post.AddProp(streaming.ConversationIDProp, conv.ID)
			post.AddProp(streaming.RespondingToProp, userPostID)
			post.AddProp(AnalysisTypeProp, "summarize_channel")
			if tc.thread {
				post.AddProp(ThreadIDProp, "thread-root")
				post.AddProp(AnalysisTypeProp, "summarize_thread")
				conv.Operation = llm.OperationThreadAnalysis
				conv.RootPostID = &post.Id
				require.NoError(t, convStore.CreateConversation(conv))
				if !tc.wrongOwner {
					mm := c.mmClient.(*mocks.MockClient)
					threadPost := &model.Post{Id: "thread-root", ChannelId: "source-channel", UserId: "author-id", Message: "source thread content"}
					mm.On("GetPost", "thread-root").Return(threadPost, nil).Once()
					mm.On("HasPermissionToChannel", testRequesterID, "source-channel", model.PermissionReadChannel).Return(!tc.denyRead).Once()
					if !tc.denyRead {
						mm.On("GetPostThread", "thread-root").Return(&model.PostList{Order: []string{"thread-root"}, Posts: map[string]*model.Post{"thread-root": threadPost}}, nil).Once()
						mm.On("GetUser", "author-id").Return(&model.User{Id: "author-id", Username: "author"}, nil).Once()
					}
				}
			}
			for i, seed := range []struct {
				role   string
				postID *string
				text   string
			}{
				{role: "user", postID: &userPostID, text: "summarize the conversation"},
				{role: "assistant", postID: &post.Id, text: "previous summary"},
			} {
				content, err := json.Marshal([]conversation.ContentBlock{{Type: conversation.BlockTypeText, Text: seed.text}})
				require.NoError(t, err)
				require.NoError(t, convStore.CreateTurn(&store.Turn{ID: seed.role, ConversationID: conv.ID, Role: seed.role, PostID: seed.postID, Content: content, Sequence: i + 1}))
			}
			userID := testRequesterID
			if tc.wrongOwner {
				userID = "other-user"
			}
			channel := &model.Channel{Id: "dm-channel", Type: model.ChannelTypeDirect, Name: "bot-id__" + testRequesterID}
			err = c.HandleRegenerate(t.Context(), userID, post, channel)
			if tc.wantError != "" {
				require.ErrorContains(t, err, tc.wantError)
				require.Empty(t, lm.requests, "authorization must fail before calling the provider")
				require.Zero(t, streamer.calls)
				return
			}
			require.NoError(t, err)
			require.NoError(t, streamer.err)
			require.Equal(t, "done", streamer.text)
			require.Equal(t, 1, streamer.calls)
			require.Len(t, lm.requests, 1)
		})
	}
}

func TestCheckRegenerateLicense(t *testing.T) {
	threadPost := &model.Post{}
	threadPost.AddProp(ThreadIDProp, "thread-root")
	threadPost.AddProp(AnalysisTypeProp, "summarize_thread")

	channelPost := &model.Post{}
	channelPost.AddProp(AnalysisTypeProp, "summarize_channel")

	recordingPost := &model.Post{}
	recordingPost.AddProp(ReferencedRecordingFileID, "fileid")

	transcriptPost := &model.Post{}
	transcriptPost.AddProp(ReferencedTranscriptPostID, "transcript-post")

	plainPost := &model.Post{}

	tests := []struct {
		name     string
		post     *model.Post
		cap      enterprise.Capability
		minLevel enterprise.Level
		ungated  bool
	}{
		{
			name:     "thread summary",
			post:     threadPost,
			cap:      enterprise.CapThreadSummarization,
			minLevel: enterprise.LevelUnlicensed,
		},
		{
			name:     "channel summary",
			post:     channelPost,
			cap:      enterprise.CapChannelSummarization,
			minLevel: enterprise.LevelUnlicensed,
		},
		{
			name:     "recording transcription summary",
			post:     recordingPost,
			cap:      enterprise.CapMeetings,
			minLevel: enterprise.LevelEnterprise,
		},
		{
			name:     "transcription summary",
			post:     transcriptPost,
			cap:      enterprise.CapMeetings,
			minLevel: enterprise.LevelEnterprise,
		},
		{
			name:    "plain conversation post is not gated",
			post:    plainPost,
			ungated: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, level := range enterprisetest.AllLevels {
				t.Run(level.String(), func(t *testing.T) {
					c := &Conversations{licenseChecker: enterprisetest.CheckerAt(level)}
					err := c.checkRegenerateLicense(tc.post)
					if tc.ungated || level >= tc.minLevel {
						require.NoError(t, err)
						return
					}
					var licErr *enterprise.LicenseError
					require.Error(t, err)
					require.True(t, errors.As(err, &licErr))
					require.Equal(t, tc.cap, licErr.Capability)
					require.Equal(t, tc.minLevel, licErr.RequiredLevel)
				})
			}

			t.Run("nil checker keeps free summaries available", func(t *testing.T) {
				c := &Conversations{}
				err := c.checkRegenerateLicense(tc.post)
				if tc.ungated || tc.minLevel == enterprise.LevelUnlicensed {
					require.NoError(t, err)
					return
				}
				var licErr *enterprise.LicenseError
				require.Error(t, err)
				require.True(t, errors.As(err, &licErr))
			})
		})
	}
}
