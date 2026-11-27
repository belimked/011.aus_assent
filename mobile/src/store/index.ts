import { useRef, useSyncExternalStore } from 'react';
import { produce, setAutoFreeze } from 'immer';
import { AppState as RNAppState } from 'react-native';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { seed } from '../data/seed';
import type { AppState } from '../data/types';
import { l2 } from '../i18n';
import {reconcileReminders} from '../lib/notifications';
// Prototype fixtures are only loaded by the explicit offline demo reset.
const emptyState = (): AppState => ({...seed(),auth:null,contacts:[],properties:[],todos:[],captures:[],enquiries:[],leads:[],listings:[],ofi_attendees:[],sales:[],contracts:[],referrals:[],market:[],zones:[],myTags:[],audit:[],approvals:[],syncq:[],queue:0});
// Avoid freezing the full 40k-property tree when a single record changes.
setAutoFreeze(false);
let state = emptyState();
let ready = false;
const listeners = new Set<() => void>();
const emit = () => listeners.forEach(fn => fn());
export const getState = () => state;
export const subscribe = (fn: () => void) => { listeners.add(fn); return () => {listeners.delete(fn)}; };
export function useStore<T>(selector: (state: AppState) => T): T {
  // Subscribe to the selected snapshot, not the entire state object.
  return useSyncExternalStore(subscribe, () => selector(getState()), () => selector(getState()));
}
// Pages using domain helpers that need AppState can subscribe to exactly the fields they read.
export function useStoreKeys(...keys: (keyof AppState)[]): AppState {
  const previous = useRef<AppState>(null);
  const snapshot = () => {
    const current = getState();
    if (previous.current && keys.every(key => previous.current![key] === current[key])) return previous.current;
    previous.current = current;
    return current;
  };
  return useSyncExternalStore(subscribe, snapshot, snapshot);
}
export async function hydrate() { try { const raw = await AsyncStorage.getItem('ax_state'); const saved = raw && JSON.parse(raw); if (saved?.v === 8 && saved.contacts && saved.captures) state = {...emptyState(),...saved,ofi_attendees:saved.ofi_attendees||[]}; } catch {} ready = true; emit(); reconcileReminders(state); }
export const useReady = () => useSyncExternalStore(subscribe, () => ready, () => true);
let localWrite: ((before: AppState, after: AppState) => void) | null = null;
export const onLocalWrite = (fn: typeof localWrite) => { localWrite = fn; };
let persistTimer: ReturnType<typeof setTimeout> | undefined;
let persistGeneration = 0;
let persistChain: Promise<unknown> = Promise.resolve();
const persist = () => {
  clearTimeout(persistTimer);
  const generation = ++persistGeneration;
  persistTimer = setTimeout(() => {
    persistTimer = undefined;
    // Serialize only once for the latest state; order writes to avoid an older write winning.
    const snapshot = JSON.stringify(state);
    persistChain = persistChain.then(() => {
      if (generation !== persistGeneration) return;
      return AsyncStorage.setItem('ax_state', snapshot);
    }).catch(() => {});
  }, 800);
};
export const flushState = () => {
  if (!persistTimer) return;
  clearTimeout(persistTimer);
  persistTimer = undefined;
  const generation = ++persistGeneration;
  const snapshot = JSON.stringify(state);
  persistChain = persistChain.then(() => {
    if (generation !== persistGeneration) return;
    return AsyncStorage.setItem('ax_state', snapshot);
  }).catch(() => {});
};
export const cancelStateWrite = () => { clearTimeout(persistTimer); persistTimer = undefined; ++persistGeneration; };
RNAppState.addEventListener('change', status => { if (status === 'background' || status === 'inactive') flushState(); });
export function replaceState(next: AppState) { state = next; emit(); reconcileReminders(state); persist(); }
// Drop the callback's return value: `d=>d.list.push(x)` returns a number, and immer throws when a producer both mutates and returns.
export function update(fn: (draft: AppState) => void) { const before = state; const next = produce(state, draft => { fn(draft); }); if (next === before) return; replaceState(next); localWrite?.(before,next); }
export function resetDemo() { state = seed(); emit(); reconcileReminders(state); persist(); }
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
