// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {render, screen} from '@testing-library/react';
import {Provider} from 'react-redux';
import {createStore} from 'redux';

import SearchButton from './search_button';

jest.mock('./assets/icon_ai', () => ({
    __esModule: true,
    default: () => <span>{'ai-icon'}</span>,
}));

describe('SearchButton availability', () => {
    test.each(['', 'professional', 'enterprise'])('shows the semantic search entry for SKU "%s"', (sku) => {
        const store = createStore(() => ({
            entities: {
                general: {
                    config: {EnableTesting: 'false', EnableDeveloper: 'false'},
                    license: {SkuShortName: sku},
                },
            },
        }));
        render(<Provider store={store}><SearchButton/></Provider>);
        expect(screen.getByText('Agents')).not.toBeNull();
    });
});
