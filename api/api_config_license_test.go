// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mattermost/mattermost-plugin-agents/v2/config"
	"github.com/mattermost/mattermost-plugin-agents/v2/enterprise"
	"github.com/mattermost/mattermost-plugin-agents/v2/enterprise/enterprisetest"
	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/stretchr/testify/require"
)

func TestHandleSaveConfigLicenseGates(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		body       config.Config
		minLevel   enterprise.Level
		wantStatus int
	}{
		{
			name:       "single service is available at every level",
			body:       config.Config{Services: []llm.ServiceConfig{{ID: "s1", Name: "OpenAI", Type: "openai"}}},
			wantStatus: http.StatusOK,
		},
		{
			name:       "multiple services are available at every level",
			body:       config.Config{Services: []llm.ServiceConfig{{ID: "s1", Type: "openai"}, {ID: "s2", Type: "openai"}, {ID: "s3", Type: "anthropic"}}},
			wantStatus: http.StatusOK,
		},
		{
			name:       "token accounting is available at Professional and above",
			body:       config.Config{EnableTokenUsageLogging: true},
			minLevel:   enterprise.LevelProfessional,
			wantStatus: http.StatusForbidden,
		},
		{
			name: "turning token accounting off is accepted",
			body: config.Config{EnableTokenUsageLogging: false},
			// prev is seeded with logging on in the test body below
			wantStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, level := range enterprisetest.AllLevels {
				t.Run(level.String(), func(t *testing.T) {
					prev := &config.Config{}
					if tc.name == "turning token accounting off is accepted" {
						prev.EnableTokenUsageLogging = true
					}
					store := &testConfigStore{cfg: prev}
					router := gin.New()
					a := &API{
						configStore:     store,
						configUpdater:   &testConfigUpdater{},
						clusterNotifier: &testClusterNotifier{},
						licenseChecker:  enterprisetest.CheckerAt(level),
					}
					router.PUT("/admin/config", a.handleSaveConfig)

					body, err := json.Marshal(tc.body)
					require.NoError(t, err)
					req := httptest.NewRequest(http.MethodPut, "/admin/config", bytes.NewReader(body))
					req.Header.Set("Content-Type", "application/json")
					w := httptest.NewRecorder()
					router.ServeHTTP(w, req)

					denied := tc.minLevel != 0 && level < tc.minLevel
					if denied {
						require.Equal(t, http.StatusForbidden, w.Code)
						return
					}
					require.Equal(t, http.StatusOK, w.Code)
				})
			}
		})
	}
}

func TestHandleSaveConfigMultipleAgentsAndServicesAccepted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	proposed := config.Config{
		Services: []llm.ServiceConfig{{ID: "s1", Type: "openai"}, {ID: "s2", Type: "anthropic"}, {ID: "s3", Type: "openai"}},
		Bots: []llm.BotConfig{
			{ID: "b1", Name: "one", DisplayName: "One", ServiceID: "s1"},
			{ID: "b2", Name: "two", DisplayName: "Two", ServiceID: "s2"},
			{ID: "b3", Name: "three", DisplayName: "Three", ServiceID: "s3"},
		},
	}
	for _, tc := range []struct {
		name       string
		admin      bool
		nilChecker bool
	}{
		{name: "free admin can save all agents and providers", admin: true},
		{name: "admin without checker can save all agents and providers", admin: true, nilChecker: true},
		{name: "non-admin remains forbidden"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := SetupTestEnvironment(t)
			defer e.Cleanup(t)
			e.OverrideLicense(nil)
			if tc.nilChecker {
				e.api.licenseChecker = nil
			}
			e.mockAPI.On("HasPermissionTo", testUserID, model.PermissionManageSystem).Return(tc.admin)
			store := &testConfigStore{cfg: &config.Config{}}
			updater := &testConfigUpdater{}
			e.api.configStore = store
			e.api.configUpdater = updater
			e.api.clusterNotifier = &testClusterNotifier{}
			body, err := json.Marshal(proposed)
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPut, "/admin/config", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Mattermost-User-ID", testUserID)
			w := httptest.NewRecorder()
			e.api.ServeHTTP(&plugin.Context{}, w, req)
			if !tc.admin {
				require.Equal(t, http.StatusForbidden, w.Code)
				require.Empty(t, store.cfg.Services)
				require.Empty(t, store.cfg.Bots)
				require.Zero(t, updater.callCount)
				return
			}
			require.Equal(t, http.StatusOK, w.Code)
			require.Len(t, store.cfg.Services, 3)
			require.Len(t, store.cfg.Bots, 3)
			require.Equal(t, 1, updater.callCount)
			var saved config.Config
			require.NoError(t, json.NewDecoder(w.Body).Decode(&saved))
			require.Equal(t, proposed.Bots, saved.Bots)
			require.Len(t, saved.Services, 3)
		})
	}
}
