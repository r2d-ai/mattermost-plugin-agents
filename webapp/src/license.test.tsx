// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {Provider} from 'react-redux';
import {createStore} from 'redux';
import {renderHook} from '@testing-library/react';

import {GlobalState} from '@mattermost/types/store';

import {Capability, licenseAllows, useAgentLimit, useIsLicensedFor, useServiceLimit} from './license';

const paidCapabilities: Capability[] = [
    'thread_summarization',
    'channel_summarization',
    'provider_web_search',
    'agent_access_controls',
    'token_accounting',
    'multiple_llm_services',
    'model_fallback',
    'state_changing_tools',
    'sovereign_web_search',
    'tool_approval_policies',
    'remote_mcp',
    'semantic_search',
    'meetings',
    'mcp_service_account',
    'shared_prompts',
    'channel_auto_reply',
    'attribute_based_access',
];

function makeState(license?: Record<string, string> | null): GlobalState {
    return {
        entities: {
            general: {
                config: {EnableTesting: 'false', EnableDeveloper: 'false'},
                license,
            },
        },
    } as unknown as GlobalState;
}

describe('channel agent license policy', () => {
    test.each<{name: string; license?: Record<string, string> | null}>([
        {name: 'missing license'},
        {name: 'null license', license: null},
        {name: 'empty license', license: {}},
        {name: 'unlicensed server', license: {IsLicensed: 'false'}},
    ])('allows channel mentions with a $name without enabling other paid capabilities', ({license}) => {
        const state = makeState(license);

        expect(licenseAllows(state, 'multiplayer_channels')).toBe(true);
        for (const capability of paidCapabilities) {
            expect({capability, allowed: licenseAllows(state, capability)}).toEqual({capability, allowed: false});
        }
    });

    test('exposes channel access through the UI hook while keeping free limits and other gates', () => {
        const store = createStore(() => makeState({IsLicensed: 'false'}));
        const wrapper = ({children}: {children: React.ReactNode}) => (
            <Provider store={store}>{children}</Provider>
        );
        const {result} = renderHook(() => ({
            channels: useIsLicensedFor('multiplayer_channels'),
            autoReply: useIsLicensedFor('channel_auto_reply'),
            accessControls: useIsLicensedFor('agent_access_controls'),
            stateChangingTools: useIsLicensedFor('state_changing_tools'),
            agents: useAgentLimit(),
            services: useServiceLimit(),
        }), {wrapper});

        expect(result.current).toEqual({
            channels: true,
            autoReply: false,
            accessControls: false,
            stateChangingTools: false,
            agents: 1,
            services: 1,
        });
    });
});
