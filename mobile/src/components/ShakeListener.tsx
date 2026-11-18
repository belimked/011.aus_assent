import {useEffect,useRef} from 'react';
import {AppState,Platform} from 'react-native';
import {usePathname} from 'expo-router';
import {Accelerometer} from 'expo-sensors';
import {closeSheet,openSheet,useSheet,useStore} from '../store';
export function ShakeListener(){const sheet=useSheet();const auth=useStore(s=>s.auth);const pathname=usePathname();const current=useRef(sheet);const enabled=useRef(false);useEffect(()=>{enabled.current=!!auth&&pathname!=='/login'},[auth,pathname]);useEffect(()=>{current.current=sheet},[sheet]);useEffect(()=>{if(Platform.OS==='web')return;let foreground=AppState.currentState==='active';let armed=true;let last=0;Accelerometer.setUpdateInterval(120);const subscription=Accelerometer.addListener(({x,y,z})=>{if(!foreground||!enabled.current)return;const force=Math.sqrt(x*x+y*y+z*z);if(force<1.25)armed=true;if(force>1.85&&armed&&Date.now()-last>1800){armed=false;last=Date.now();if(current.current?.kind==='translate')closeSheet();else openSheet({kind:'translate'})}});const app=AppState.addEventListener('change',state=>{foreground=state==='active';armed=false});return()=>{subscription.remove();app.remove()}},[]);return null}
