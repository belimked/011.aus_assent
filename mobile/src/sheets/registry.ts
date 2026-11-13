import type React from 'react';
import {PlusSheet,AskSheet,TodoSheet,ReassignSheet,CaptureSheet} from './sheets';
import type {SheetProps} from './types';
import {dkSheets} from './dk';
import {ofiSheets} from './ofi';
import {enqSheets} from './enq';
export type {SheetProps};
export const registry:Record<string,React.ComponentType<SheetProps>> = {plus:PlusSheet,ask:AskSheet,todo:TodoSheet,tdre:ReassignSheet,capture:CaptureSheet,...dkSheets,...ofiSheets,...enqSheets};
