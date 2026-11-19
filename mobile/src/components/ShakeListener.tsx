import {useEffect,useRef} from 'react';
import {AppState,Platform} from 'react-native';
import {usePathname} from 'expo-router';
import {Accelerometer} from 'expo-sensors';
import {closeSheet,openSheet,useSheet,useStore} from '../store';

// A deliberate shake, not a single bump: several sharp changes in acceleration within a short window.
// We measure the change between consecutive samples (jerk), which ignores gravity and phone orientation,
// so it does not depend on the phone dropping back to rest between strokes.
// Values are in g (1 = gravity).
const SAMPLE_MS=50;      // accelerometer interval
const JOLT_G=1.4;        // change between two samples that counts as one shake stroke
const MIN_GAP_MS=120;    // one stroke per back-and-forth; ignore samples closer than this
const PEAKS_NEEDED=3;    // strokes required...
const WINDOW_MS=1000;    // ...within this window
const COOLDOWN_MS=1500;  // ignore further shakes after one fires

export function ShakeListener(){
  const sheet=useSheet();
  const auth=useStore(s=>s.auth);
  const pathname=usePathname();
  const current=useRef(sheet);
  const enabled=useRef(false);
  useEffect(()=>{enabled.current=!!auth&&pathname!=='/login'},[auth,pathname]);
  useEffect(()=>{current.current=sheet},[sheet]);
  useEffect(()=>{
    if(Platform.OS==='web')return;
    let foreground=AppState.currentState==='active';
    let prev:{x:number;y:number;z:number}|null=null;
    let peaks:number[]=[];
    let last=0;
    Accelerometer.setUpdateInterval(SAMPLE_MS);
    const subscription=Accelerometer.addListener(({x,y,z})=>{
      if(!foreground||!enabled.current)return;
      const now=Date.now();
      const p=prev;prev={x,y,z};
      if(!p||now-last<COOLDOWN_MS)return;
      const jolt=Math.sqrt((x-p.x)**2+(y-p.y)**2+(z-p.z)**2);
      if(jolt<JOLT_G)return;
      if(peaks.length&&now-peaks[peaks.length-1]<MIN_GAP_MS)return;
      peaks=[...peaks.filter(t=>now-t<WINDOW_MS),now];
      if(peaks.length<PEAKS_NEEDED)return;
      peaks=[];
      last=now;
      if(current.current?.kind==='translate')closeSheet();
      else openSheet({kind:'translate'});
    });
    const app=AppState.addEventListener('change',state=>{foreground=state==='active';prev=null;peaks=[]});
    return()=>{subscription.remove();app.remove()};
  },[]);
  return null;
}
