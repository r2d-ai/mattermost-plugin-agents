// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {fireEvent, render, screen, waitFor} from '@testing-library/react';
import {IntlProvider} from 'react-intl';
import {Provider} from 'react-redux';
import {createStore} from 'redux';

import {Post} from '@mattermost/types/posts';

import {doThreadAnalysis} from '../client';

import PostMenu from './post_menu';

jest.mock('react-intl', () => {
    const actual = jest.requireActual('react-intl');
    return {
        ...actual,
        useIntl: () => ({
            formatMessage: ({defaultMessage}: {defaultMessage: string}) => defaultMessage,
        }),
        FormattedMessage: ({defaultMessage}: {defaultMessage: string}) => defaultMessage,
    };
});

jest.mock('@/hooks', () => ({
    useSelectPost: () => jest.fn(),
}));

jest.mock('@/bots', () => ({
    useBotlistForChannel: () => ({
        bots: [{id: 'bot1', username: 'matty', displayName: 'Matty'}],
        activeBot: {id: 'bot1', username: 'matty', displayName: 'Matty'},
        setActiveBot: jest.fn(),
    }),
}));

jest.mock('../client', () => ({
    doReaction: jest.fn(),
    doThreadAnalysis: jest.fn(),
}));

jest.mock('./dot_menu', () => ({
    __esModule: true,
    default: ({
        children,
        size,
        padding,
        title,
        testId,
    }: {
        children: React.ReactNode;
        size?: string;
        padding?: string;
        title?: string;
        testId?: string;
    }) => (
        <div>
            <button
                type='button'
                data-testid={testId}
                data-size={size}
                data-padding={padding}
                aria-label={title}
            />
            {children}
        </div>
    ),
    DropdownMenu: ({children}: {children: React.ReactNode}) => <div>{children}</div>,
    DropdownMenuItem: ({label, onClick}: {label: React.ReactNode; onClick?: () => void}) => (
        <button
            type='button'
            onClick={onClick}
        >
            {label}
        </button>
    ),
}));

jest.mock('./bot_selector', () => ({
    DropdownBotSelector: () => <div>{'bot-selector'}</div>,
}));

const post = {id: 'post1', channel_id: 'chan1'} as Post;

function renderMenu(skuShortName = '') {
    const store = createStore(() => ({
        entities: {
            general: {
                config: {EnableTesting: 'false', EnableDeveloper: 'false'},
                license: {SkuShortName: skuShortName},
            },
        },
    }));
    return render(
        <Provider store={store}>
            <IntlProvider locale='en'>
                <PostMenu post={post}/>
            </IntlProvider>
        </Provider>,
    );
}

describe('PostMenu summary availability', () => {
    test.each(['', 'professional'])('shows thread summary actions for SKU "%s"', (sku) => {
        renderMenu(sku);

        expect(screen.getByText('Summarize Thread')).not.toBeNull();
        expect(screen.getByText('Find action items')).not.toBeNull();
        expect(screen.getByText('Find open questions')).not.toBeNull();
        expect(screen.getByText('React for me')).not.toBeNull();
    });

    test('an unlicensed user can request a summary for the selected thread and agent', async () => {
        const mockDoThreadAnalysis = doThreadAnalysis as jest.MockedFunction<typeof doThreadAnalysis>;
        mockDoThreadAnalysis.mockResolvedValue({postid: 'summary', channelid: 'dm1'});
        renderMenu();

        fireEvent.click(screen.getByText('Summarize Thread'));
        await waitFor(() => expect(mockDoThreadAnalysis).toHaveBeenCalledWith('post1', 'summarize_thread', 'bot1'));
    });
});

describe('PostMenu sizing', () => {
    test('uses small + compact IconButton to match host post-menu actions (28px)', () => {
        renderMenu();

        const button = screen.getByTestId('ai-actions-menu');
        expect(button.getAttribute('data-size')).toBe('small');
        expect(button.getAttribute('data-padding')).toBe('compact');
        expect(button.getAttribute('aria-label')).toBe('AI Actions');
        expect(button.getAttribute('title')).toBeNull();
    });
});
