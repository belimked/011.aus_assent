import { useSyncExternalStore } from 'react';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { seed } from '../data/seed';
import type { AppState } from '../data/types';
import { l2 } from '../i18n';
import {reconcileReminders} from '../lib/notifications';
// Prototype fixtures are only loaded by the explicit offline demo reset.
const emptyState = (): AppState => ({...seed(),auth:null,contacts:[],properties:[],todos:[],captures:[],enquiries:[],leads:[],listings:[],ofi_attendees:[],sales:[],contracts:[],referrals:[],market:[],zones:[],myTags:[],audit:[],approvals:[],syncq:[],queue:0});
let state = emptyState();
let ready = false;
const listeners = new Set<() => void>();
const emit = () => listeners.forEach(fn => fn());
export const getState = () => state;
export const subscribe = (fn: () => void) => { listeners.add(fn); return () => {listeners.delete(fn)}; };
export function useStore<T>(selector: (state: AppState) => T): T { return selector(useSyncExternalStore(subscribe, getState, getState)); }
export async function hydrate() { try { const raw = await AsyncStorage.getItem('ax_state'); const saved = raw && JSON.parse(raw); if (saved?.v === 8 && saved.contacts && saved.captures) state = {...emptyState(),...saved,ofi_attendees:saved.ofi_attendees||[]}; } catch {} ready = true; emit(); reconcileReminders(state); }
export const useReady = () => useSyncExternalStore(subscribe, () => ready, () => true);
let localWrite: ((before: AppState, after: AppState) => void) | null = null;
export const onLocalWrite = (fn: typeof localWrite) => { localWrite = fn; };
export function replaceState(next: AppState) { state = next; emit(); reconcileReminders(state); void AsyncStorage.setItem('ax_state', JSON.stringify(state)).catch(() => {}); }
export function update(fn: (draft: AppState) => void) { const before = state; const next = structuredClone(state); fn(next); replaceState(next); localWrite?.(before,next); }
export function resetDemo() { state = seed(); emit(); reconcileReminders(state); void AsyncStorage.setItem('ax_state', JSON.stringify(state)).catch(() => {}); }
type Sheet = {kind:string;prev?:Sheet;[key:string]:any};
let sheet: Sheet | null = null;
const sheetListeners = new Set<() => void>();
const notifySheet = () => sheetListeners.forEach(fn => fn());
export const openSheet = (s: Sheet) => { sheet = {...s, prev: sheet ?? undefined}; notifySheet(); };
export const closeSheet = () => { sheet = sheet?.prev ?? null; notifySheet(); };
export const dismissSheet = () => { sheet = null; notifySheet(); };
export const markSheetDirty = () => { if(sheet) sheet.dirty = true; };
export const dismissSheetBackdrop = () => { if(sheet?.dirty){showToast(l2('Unsaved changes. Tap Cancel to discard.','有未保存的更改。点击取消以放弃。'));return} dismissSheet(); };
export const useSheet = () => useSyncExternalStore(fn => {sheetListeners.add(fn); return () => {sheetListeners.delete(fn)}}, () => sheet, () => null);
let toast: {text:string;undo?:()=>void}|null = null;
const toastListeners = new Set<() => void>();
let toastSeq = 0;
const notifyToast = () => toastListeners.forEach(fn=>fn());
// Each toast owns its timer: an older toast's timeout must not clear a newer one.
export const showToast = (text:string, undo?:()=>void) => {const seq=++toastSeq;toast={text,undo};notifyToast();setTimeout(()=>{if(seq!==toastSeq)return;toast=null;notifyToast()},undo?6000:2500)};
export const hideToast = () => {toastSeq++;toast=null;notifyToast();};
export const useToast = () => useSyncExternalStore(fn=>{toastListeners.add(fn);return()=>{toastListeners.delete(fn)}},()=>toast,()=>null);
