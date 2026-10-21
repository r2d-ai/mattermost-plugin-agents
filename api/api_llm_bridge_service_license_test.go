// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api

import (
	"testing"

	"github.com/mattermost/mattermost-plugin-agents/v2/enterprise"
	"github.com/mattermost/mattermost-plugin-agents/v2/enterprise/enterprisetest"
	"github.com/mattermost/mattermost-plugin-agents/v2/llm"
	"github.com/mattermost/mattermost-plugin-agents/v2/public/bridgeclient"
	"github.com/stretchr/testify/require"
)

// TestBridgeServiceEndpointsByLevel pins that bridge service listing and
// completion can use every configured provider at every level, while fallback
// chains still require Enterprise Advanced.
func TestBridgeServiceEndpointsByLevel(t *testing.T) {
	primary := bridgeServiceConfig("svc-primary", "Primary", llm.ServiceTypeOpenAI, "gpt-4o")
	primary.FallbackServiceID = "svc-second"
	second := bridgeServiceConfig("svc-second", "Second", llm.ServiceTypeAnthropic, "claude-sonnet-4-5")
	services := []llm.ServiceConfig{primary, second}

	levels := append([]*enterprise.Level{nil}, levelPointers()...)
	for _, level := range levels {
		name := "nil checker allows all configured providers"
		current := enterprise.LevelUnlicensed
		if level != nil {
			name = level.String()
			current = *level
		}
		t.Run(name, func(t *testing.T) {
			e, builder, client := setupServiceBridge(t, services, NewFakeLLM("ok"))
			if level == nil {
				e.api.licenseChecker = nil
			} else {
				e.OverrideLicense(enterprisetest.LicenseFor(*level))
			}
			fallbacks := current >= enterprise.RequiredLevel(enterprise.CapModelFallback)

			listed, err := client.GetServices("")
			require.NoError(t, err)
			ids := make([]string, 0, len(listed))
			for _, svc := range listed {
				ids = append(ids, svc.ID)
			}
			require.ElementsMatch(t, []string{"svc-primary", "svc-second"}, ids)

			request := bridgeclient.CompletionRequest{Posts: []bridgeclient.Post{{Role: "user", Message: "hi"}}}
			_, err = client.ServiceCompletion("svc-primary", request)
			require.NoError(t, err)
			builds := builder.buildCalls()
			require.Len(t, builds, 1)
			if fallbacks {
				require.Len(t, builds[0].fallbacks, 1)
				require.Equal(t, "svc-second", builds[0].fallbacks[0].ID)
			} else {
				require.Empty(t, builds[0].fallbacks)
			}

			_, err = client.ServiceCompletion("svc-second", request)
			require.NoError(t, err)
		})
	}
}

func levelPointers() []*enterprise.Level {
	out := make([]*enterprise.Level, 0, len(enterprisetest.AllLevels))
	for _, level := range enterprisetest.AllLevels {
		out = append(out, &level)
	}
	return out
}
