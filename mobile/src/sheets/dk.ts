import type React from 'react';
import type {SheetProps} from './types';
// Sheets owned by the dk feature. Add entries here only.
import {Cmd,Confirm,Change,Note,TagPick,QuickAdd} from './dk/Core';
import {Photo,Call,TextSheet,Email,Followup,Remind,BookAppr,Rel,EField,Merge} from './dk/Actions';
export const dkSheets: Record<string, React.ComponentType<SheetProps>> = {cmd:Cmd,confirm:Confirm,change:Change,note:Note,tagpick:TagPick,quickadd:QuickAdd,photo:Photo,call:Call,text:TextSheet,email:Email,followup:Followup,remind:Remind,bookappr:BookAppr,rel:Rel,efield:EField,merge:Merge};
