import {useEffect,useRef} from 'react';
import {AppState,Platform} from 'react-native';
import {usePathname} from 'expo-router';
import {Accelerometer} from 'expo-sensors';
import {closeSheet,openSheet,useSheet,useStore} from '../store';

// A deliberate shake, not a single bump: several strong peaks within a short window.
// Values are in g (1 = gravity at rest).
const SAMPLE_MS=60;      // accelerometer interval
const PEAK_G=2.6;        // a sample this strong counts as one shake peak
const RESET_G=1.3;       // must drop back below this before the next peak counts
const PEAKS_NEEDED=4;    // peaks required...
const WINDOW_MS=1000;    // ...within this window
const COOLDOWN_MS=2000;  // ignore further shakes after one fires

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
    let armed=true;
    let peaks:number[]=[];
    let last=0;
    Accelerometer.setUpdateInterval(SAMPLE_MS);
    const subscription=Accelerometer.addListener(({x,y,z})=>{
      if(!foreground||!enabled.current)return;
      const now=Date.now();
      if(now-last<COOLDOWN_MS)return;
      const force=Math.sqrt(x*x+y*y+z*z);
      if(force<RESET_G){armed=true;return}
      if(force<PEAK_G||!armed)return;
      armed=false;
      peaks=[...peaks.filter(t=>now-t<WINDOW_MS),now];
      if(peaks.length<PEAKS_NEEDED)return;
      peaks=[];
      last=now;
      if(current.current?.kind==='translate')closeSheet();
      else openSheet({kind:'translate'});
    });
    const app=AppState.addEventListener('change',state=>{foreground=state==='active';armed=false;peaks=[]});
    return()=>{subscription.remove();app.remove()};
  },[]);
  return null;
}
