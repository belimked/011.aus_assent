import type React from 'react';
import type {SheetProps} from './types';
import {DictateSheet} from './enq/Dictate';
export const enqSheets: Record<string, React.ComponentType<SheetProps>> = {dictate:DictateSheet};
