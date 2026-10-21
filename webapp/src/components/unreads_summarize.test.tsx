// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {render, screen} from '@testing-library/react';
import {IntlProvider} from 'react-intl';
import {Provider} from 'react-redux';
import {createStore} from 'redux';

import UnreadsSumarize from './unreads_summarize';

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
        bots: [{username: 'matty', displayName: 'Matty'}],
        activeBot: {username: 'matty', displayName: 'Matty'},
        setActiveBot: jest.fn(),
    }),
}));

jest.mock('@/client', () => ({
    getChannelInterval: jest.fn(),
}));

jest.mock('./dot_menu', () => ({
    __esModule: true,
    default: ({children}: {children: React.ReactNode}) => <div data-testid='unreads-menu'>{children}</div>,
    DotMenuButton: ({children}: {children: React.ReactNode}) => <div>{children}</div>,
    DropdownMenu: ({children}: {children: React.ReactNode}) => <div>{children}</div>,
    DropdownMenuItem: ({label}: {label: React.ReactNode}) => <button type='button'>{label}</button>,
}));

jest.mock('./bot_selector', () => ({
    DropdownBotSelector: () => <div>{'bot-selector'}</div>,
}));

describe('UnreadsSumarize availability', () => {
    test.each(['', 'professional'])('shows channel summary actions for SKU "%s"', (sku) => {
        const store = createStore(() => ({
            entities: {
                general: {
                    config: {EnableTesting: 'false', EnableDeveloper: 'false'},
                    license: {SkuShortName: sku},
                },
            },
        }));
        render(
            <Provider store={store}>
                <IntlProvider locale='en'>
                    <UnreadsSumarize
                        lastViewedAt={1}
                        channelId='chan1'
                        threadId=''
                    />
                </IntlProvider>
            </Provider>,
        );

        expect(screen.getByText('Summarize new messages')).not.toBeNull();
        expect(screen.getByText('Find action items')).not.toBeNull();
        expect(screen.getByText('Find open questions')).not.toBeNull();
    });
});
