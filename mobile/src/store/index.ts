import { useSyncExternalStore } from 'react';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { seed } from '../data/seed';
import type { AppState } from '../data/types';
let state = seed();
let ready = false;
const listeners = new Set<() => void>();
const emit = () => listeners.forEach(fn => fn());
export const getState = () => state;
export const subscribe = (fn: () => void) => { listeners.add(fn); return () => {listeners.delete(fn)}; };
export function useStore<T>(selector: (state: AppState) => T): T { return selector(useSyncExternalStore(subscribe, getState, getState)); }
export async function hydrate() { try { const raw = await AsyncStorage.getItem('ax_state'); const saved = raw && JSON.parse(raw); if (saved?.v === 8 && saved.contacts && saved.captures) state = saved; } catch {} ready = true; emit(); }
export const useReady = () => useSyncExternalStore(subscribe, () => ready, () => true);
export function update(fn: (draft: AppState) => void) { const next = structuredClone(state); fn(next); state = next; emit(); void AsyncStorage.setItem('ax_state', JSON.stringify(state)).catch(() => {}); }
export function resetDemo() { state = seed(); emit(); void AsyncStorage.setItem('ax_state', JSON.stringify(state)).catch(() => {}); }
type Sheet = {kind:string;prev?:Sheet;[key:string]:any};
let sheet: Sheet | null = null;
const sheetListeners = new Set<() => void>();
const notifySheet = () => sheetListeners.forEach(fn => fn());
export const openSheet = (s: Sheet) => { sheet = {...s, prev: sheet ?? undefined}; notifySheet(); };
export const closeSheet = () => { sheet = sheet?.prev ?? null; notifySheet(); };
export const dismissSheet = () => { sheet = null; notifySheet(); };
export const useSheet = () => useSyncExternalStore(fn => {sheetListeners.add(fn); return () => {sheetListeners.delete(fn)}}, () => sheet, () => null);
let toast: {text:string;undo?:()=>void}|null = null;
const toastListeners = new Set<() => void>();
export const showToast = (text:string, undo?:()=>void) => {toast={text,undo};toastListeners.forEach(fn=>fn());setTimeout(()=>{toast=null;toastListeners.forEach(fn=>fn())},undo?6000:2500)};
export const useToast = () => useSyncExternalStore(fn=>{toastListeners.add(fn);return()=>{toastListeners.delete(fn)}},()=>toast,()=>null);
