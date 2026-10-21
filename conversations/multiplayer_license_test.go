// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package conversations_test

import (
	"context"
	"testing"

	"github.com/mattermost/mattermost-plugin-agents/v2/conversations"
	"github.com/mattermost/mattermost-plugin-agents/v2/enterprise/enterprisetest"
	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
	"github.com/mattermost/mattermost-plugin-agents/v2/streaming"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/require"
)

func TestChannelMentionWorksAtEveryLicenseLevel(t *testing.T) {
	channels := []struct {
		name      string
		configure func(*model.Channel)
	}{
		{
			name: "public channel",
			configure: func(channel *model.Channel) {
				channel.Type = model.ChannelTypeOpen
			},
		},
		{
			name: "private channel",
			configure: func(channel *model.Channel) {
				channel.Type = model.ChannelTypePrivate
			},
		},
		{
			name: "group message",
			configure: func(channel *model.Channel) {
				channel.Type = model.ChannelTypeGroup
				channel.Name = autoReplyBotUserID + "__" + autoReplyUserID + "__other"
				channel.TeamId = ""
			},
		},
	}
	posts := []struct {
		name  string
		build func(*autoReplyTestEnv) *model.Post
	}{
		{
			name: "top-level mention",
			build: func(env *autoReplyTestEnv) *model.Post {
				return env.rootPost(autoReplyUserID, "@"+autoReplyBotUsername+" help me")
			},
		},
		{
			name: "mention in existing thread",
			build: func(env *autoReplyTestEnv) *model.Post {
				return env.threadReply(autoReplyUserID, "@"+autoReplyBotUsername+" help me", false)
			},
		},
	}

	for _, channel := range channels {
		t.Run(channel.name, func(t *testing.T) {
			for _, level := range enterprisetest.AllLevels {
				t.Run(level.String(), func(t *testing.T) {
					for _, postCase := range posts {
						t.Run(postCase.name, func(t *testing.T) {
							env := setupAutoReplyTestEnv(t, []llm.BotConfig{autoReplyBotConfig()}, dmMakeTextStream("canned reply"))
							env.overrideLicense(enterprisetest.LicenseFor(level))
							channel.configure(env.channel)

							post := postCase.build(env)
							env.conversations.MessageHasBeenPosted(nil, post)

							requireMultiplayerConversationStarted(t, env, post, post.Message)
						})
					}
				})
			}
		})
	}
}

func TestDMMentionWorksAtEveryLicenseLevel(t *testing.T) {
	for _, level := range enterprisetest.AllLevels {
		t.Run(level.String(), func(t *testing.T) {
			env := setupDMTestEnv(t, dmMakeTextStream("hello from dm"))
			overrideMockLicense(env.mockAPI, enterprisetest.LicenseFor(level))

			env.conversations.MessageHasBeenPosted(nil, &model.Post{
				Id:        "dm-post",
				UserId:    env.userID,
				ChannelId: env.channelID,
				Message:   "hello",
			})

			require.NotEmpty(t, env.mmClient.createdPosts, "agent DMs are available at every license level")
			require.Empty(t, env.mmClient.ephemeralPosts)
		})
	}
}

func TestHandleLoopInAgentWorksAtEveryLicenseLevel(t *testing.T) {
	for _, level := range enterprisetest.AllLevels {
		t.Run(level.String(), func(t *testing.T) {
			env := setupAutoReplyTestEnv(t, []llm.BotConfig{autoReplyBotConfig()}, dmMakeTextStream("canned reply"))
			env.overrideLicense(enterprisetest.LicenseFor(level))
			post := env.threadReply(autoReplyUserID, "thanks", true)
			bot := env.botService.GetBotByID(autoReplyBotUserID)
			require.NotNil(t, bot)

			err := env.conversations.HandleLoopInAgent(context.Background(), autoReplyUserID, bot, post, env.channel)

			require.NoError(t, err)
			requireMultiplayerConversationStarted(t, env, post, "@"+autoReplyBotUsername+" "+post.Message)
		})
	}
}

func TestUnlicensedChannelMentionPreservesRestrictions(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*llm.BotConfig)
		post      func(*autoReplyTestEnv) *model.Post
	}{
		{
			name: "blocked user",
			configure: func(cfg *llm.BotConfig) {
				cfg.UserAccessLevel = llm.UserAccessLevelBlock
				cfg.UserIDs = []string{autoReplyUserID}
			},
		},
		{
			name: "blocked channel",
			configure: func(cfg *llm.BotConfig) {
				cfg.ChannelAccessLevel = llm.ChannelAccessLevelBlock
				cfg.ChannelIDs = []string{autoReplyChannelID}
			},
		},
		{
			name: "channel outside allowlist",
			configure: func(cfg *llm.BotConfig) {
				cfg.ChannelAccessLevel = llm.ChannelAccessLevelAllow
				cfg.ChannelIDs = []string{"another-channel"}
			},
		},
		{
			name: "agent cannot trigger itself",
			post: func(env *autoReplyTestEnv) *model.Post {
				return env.rootPost(autoReplyBotUserID, "@"+autoReplyBotUsername+" help me")
			},
		},
		{
			name: "other bot without explicit activation",
			post: func(env *autoReplyTestEnv) *model.Post {
				return env.rootPost(autoReplyForeignBotID, "@"+autoReplyBotUsername+" help me")
			},
		},
		{
			name: "webhook mention",
			post: func(env *autoReplyTestEnv) *model.Post {
				post := env.rootPost(autoReplyUserID, "@"+autoReplyBotUsername+" help me")
				post.AddProp(conversations.FromWebhookProp, true)
				return post
			},
		},
		{
			name: "ordinary message without mention",
			post: func(env *autoReplyTestEnv) *model.Post {
				return env.rootPost(autoReplyUserID, "hello channel")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := autoReplyBotConfig()
			if tt.configure != nil {
				tt.configure(&cfg)
			}
			env := setupAutoReplyTestEnv(t, []llm.BotConfig{cfg})
			env.overrideLicense(nil)
			post := env.rootPost(autoReplyUserID, "@"+autoReplyBotUsername+" help me")
			if tt.post != nil {
				post = tt.post(env)
			}

			env.conversations.MessageHasBeenPosted(nil, post)

			require.Empty(t, env.mmClient.createdPosts)
			require.Empty(t, allConversations(env.convStore))
			env.fakeLLM.mu.Lock()
			requestCount := len(env.fakeLLM.requests)
			env.fakeLLM.mu.Unlock()
			require.Zero(t, requestCount, "rejected posts must not invoke the provider")
		})
	}
}

// requireMultiplayerConversationStarted checks that the production handler
// reaches the provider and creates an ordinary agent response in the same
// thread, rather than merely posting a licensing explanation.
func requireMultiplayerConversationStarted(t *testing.T, env *autoReplyTestEnv, post *model.Post, expectedMessage string) {
	t.Helper()
	rootID := post.RootId
	if rootID == "" {
		rootID = post.Id
	}
	require.Len(t, env.mmClient.createdPosts, 1)
	assertInitialResponsePlaceholder(t, env.mmClient.createdPosts[0], autoReplyBotUserID, post.UserId, post.ChannelId, rootID, post.Id)
	require.Empty(t, env.mmClient.ephemeralPosts)

	convs := allConversations(env.convStore)
	require.Len(t, convs, 1)
	require.NotNil(t, convs[0].RootPostID)
	require.Equal(t, rootID, *convs[0].RootPostID)
	require.Equal(t, post.UserId, convs[0].UserID)
	require.Equal(t, autoReplyBotUserID, convs[0].BotID)
	require.NotEmpty(t, env.mmClient.updatedPosts)
	require.Equal(t, convs[0].ID, env.mmClient.updatedPosts[0].GetProp(streaming.ConversationIDProp))
	blocks := singleConversationUserBlocks(t, env)
	require.Equal(t, expectedMessage, blocks[0].Text)

	env.fakeLLM.mu.Lock()
	requestCount := len(env.fakeLLM.requests)
	env.fakeLLM.mu.Unlock()
	require.Equal(t, 1, requestCount, "channel mentions must reach the provider at every license level")
	require.Empty(t, env.mmClient.loggedErrors())
}
