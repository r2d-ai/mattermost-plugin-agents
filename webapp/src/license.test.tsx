// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {Provider} from 'react-redux';
import {createStore} from 'redux';
import {renderHook} from '@testing-library/react';

import {GlobalState} from '@mattermost/types/store';

import {Capability, licenseAllows, useAgentLimit, useIsLicensedFor, useServiceLimit} from './license';

const freeCapabilities: Capability[] = [
    'multiplayer_channels',
    'thread_summarization',
    'channel_summarization',
    'multiple_llm_services',
    'remote_mcp',
    'semantic_search',
    'channel_auto_reply',
];

const paidCapabilities: Capability[] = [
    'provider_web_search',
    'agent_access_controls',
    'token_accounting',
    'model_fallback',
    'state_changing_tools',
    'sovereign_web_search',
    'tool_approval_policies',
    'meetings',
    'mcp_service_account',
    'shared_prompts',
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

describe('free agent capabilities', () => {
    test.each<{name: string; license?: Record<string, string> | null}>([
        {name: 'missing license'},
        {name: 'null license', license: null},
        {name: 'empty license', license: {}},
        {name: 'unlicensed server', license: {IsLicensed: 'false'}},
    ])('allows the selected features with a $name without enabling other paid capabilities', ({license}) => {
        const state = makeState(license);

        for (const capability of freeCapabilities) {
            expect({capability, allowed: licenseAllows(state, capability)}).toEqual({capability, allowed: true});
        }
        for (const capability of paidCapabilities) {
            expect({capability, allowed: licenseAllows(state, capability)}).toEqual({capability, allowed: false});
        }
    });

    test('exposes free features through the UI hook while keeping unrelated capability gates', () => {
        const store = createStore(() => makeState({IsLicensed: 'false'}));
        const wrapper = ({children}: {children: React.ReactNode}) => (
            <Provider store={store}>{children}</Provider>
        );
        const {result} = renderHook(() => ({
            channels: useIsLicensedFor('multiplayer_channels'),
            threadSummaries: useIsLicensedFor('thread_summarization'),
            channelSummaries: useIsLicensedFor('channel_summarization'),
            providers: useIsLicensedFor('multiple_llm_services'),
            remoteMcp: useIsLicensedFor('remote_mcp'),
            search: useIsLicensedFor('semantic_search'),
            autoReply: useIsLicensedFor('channel_auto_reply'),
            accessControls: useIsLicensedFor('agent_access_controls'),
            stateChangingTools: useIsLicensedFor('state_changing_tools'),
            agents: useAgentLimit(),
            services: useServiceLimit(),
        }), {wrapper});

        expect(result.current).toEqual({
            channels: true,
            threadSummaries: true,
            channelSummaries: true,
            providers: true,
            remoteMcp: true,
            search: true,
            autoReply: true,
            accessControls: false,
            stateChangingTools: false,
            agents: null,
            services: null,
        });
    });

    test.each(['', 'professional', 'enterprise', 'entry', 'advanced'])('keeps agent and provider counts uncapped for SKU "%s"', (sku) => {
        const store = createStore(() => makeState({SkuShortName: sku}));
        const wrapper = ({children}: {children: React.ReactNode}) => (
            <Provider store={store}>{children}</Provider>
        );
        const {result} = renderHook(() => ({agents: useAgentLimit(), services: useServiceLimit()}), {wrapper});

        expect(result.current).toEqual({agents: null, services: null});
    });
});
