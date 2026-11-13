import type React from 'react';
import type {SheetProps} from './types';
import {TranslateSheet} from './ofi/Translate';
import {ExchangeSheet} from './ofi/Exchange';
export const ofiSheets: Record<string, React.ComponentType<SheetProps>> = {translate:TranslateSheet,exchange:ExchangeSheet};
