import type React from 'react';
import {PlusSheet,AskSheet,TodoSheet,ReassignSheet,CaptureSheet} from './sheets';
export type SheetProps = {params:{kind:string;[key:string]:any}};
export const registry:Record<string,React.ComponentType<SheetProps>> = {plus:PlusSheet,ask:AskSheet,todo:TodoSheet,tdre:ReassignSheet,capture:CaptureSheet};
