import {useEffect,useSyncExternalStore} from 'react';
import {AppState,Linking,Platform} from 'react-native';
import * as Location from 'expo-location';
import type {Point} from '../../lib/geometry';

// Real device location only: no demo fallback. One shared watch for every screen that needs it.
// locating = waiting for the first fix; denied = can ask again; blocked = must enable in Settings;
// off = phone Location Services disabled; error = could not get a fix.
export type LocationStatus='locating'|'live'|'denied'|'blocked'|'off'|'error';
export type LiveLocation={status:LocationStatus;position:Point|null;accuracy:number|null};

let state:LiveLocation={status:'locating',position:null,accuracy:null};
const listeners=new Set<()=>void>();
let users=0,run=0,sub:Location.LocationSubscription|null=null,appState:string|null|undefined=AppState.currentState,appWatch=false;

const set=(patch:Partial<LiveLocation>)=>{state={...state,...patch};listeners.forEach(l=>l())};
const fix=(loc:Location.LocationObject)=>set({status:'live',position:{lat:loc.coords.latitude,lng:loc.coords.longitude},accuracy:loc.coords.accuracy??null});
const stop=()=>{run++;sub?.remove();sub=null};

async function start(ask:boolean){
  stop();const id=run;
  if(!state.position)set({status:'locating'});
  try{
    const perm=ask?await Location.requestForegroundPermissionsAsync():await Location.getForegroundPermissionsAsync();
    if(id!==run)return;
    if(!perm.granted){set({status:perm.canAskAgain?'denied':'blocked',position:null,accuracy:null});return}
    if(Platform.OS!=='web'&&!(await Location.hasServicesEnabledAsync())){if(id===run)set({status:'off',position:null,accuracy:null});return}
    if(Platform.OS!=='web'){const last=await Location.getLastKnownPositionAsync({maxAge:60_000,requiredAccuracy:200}).catch(()=>null);if(id!==run)return;if(last&&!state.position)fix(last)}
    const s=await Location.watchPositionAsync({accuracy:Location.Accuracy.High,distanceInterval:5,timeInterval:5000},loc=>{if(id===run)fix(loc)});
    if(id!==run||users===0){s.remove();return}
    sub=s;
  }catch{if(id===run&&!state.position)set({status:'error'})}
}

function watchAppState(){
  if(appWatch)return;appWatch=true;
  // Pause while backgrounded; on return re-check permission/services (the user may have just changed them in Settings).
  AppState.addEventListener('change',next=>{const prev=appState;appState=next;
    if(next==='background'){stop();return}
    if(next==='active'&&prev==='background'&&users>0)void start(false)});
}

const subscribe=(l:()=>void)=>{listeners.add(l);return ()=>{listeners.delete(l)}};

export function useLivePosition(enabled=true):LiveLocation{
  const snap=useSyncExternalStore(subscribe,()=>state,()=>state);
  useEffect(()=>{if(!enabled)return;watchAppState();users++;if(users===1)void start(true);
    return ()=>{users--;if(users===0)stop()}},[enabled]);
  return snap;
}

/** Ask for permission again (shows the system prompt if the OS still allows it). */
export const retryLocation=()=>{void start(true)};
export const canOpenSettings=Platform.OS!=='web';
export const openLocationSettings=()=>{void Linking.openSettings().catch(()=>{})};

/** Great-circle distance in metres. */
export function distanceM(a:Point,b:Point){
  const r=Math.PI/180,dLat=(b.lat-a.lat)*r,dLng=(b.lng-a.lng)*r;
  const h=Math.sin(dLat/2)**2+Math.cos(a.lat*r)*Math.cos(b.lat*r)*Math.sin(dLng/2)**2;
  return 2*6371000*Math.asin(Math.min(1,Math.sqrt(h)));
}
export const fmtDistance=(m:number)=>m<1000?`${Math.round(m)} m`:`${(m/1000).toFixed(m<10_000?1:0)} km`;
