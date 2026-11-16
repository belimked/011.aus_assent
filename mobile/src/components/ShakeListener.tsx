import {useEffect,useRef} from 'react';
import {AppState,Platform} from 'react-native';
import {Accelerometer} from 'expo-sensors';
import {closeSheet,openSheet,useSheet} from '../store';
export function ShakeListener(){const sheet=useSheet();const current=useRef(sheet);useEffect(()=>{current.current=sheet},[sheet]);useEffect(()=>{if(Platform.OS==='web')return;let foreground=AppState.currentState==='active';let armed=true;let last=0;Accelerometer.setUpdateInterval(120);const subscription=Accelerometer.addListener(({x,y,z})=>{if(!foreground)return;const force=Math.sqrt(x*x+y*y+z*z);if(force<1.25)armed=true;if(force>1.85&&armed&&Date.now()-last>1800){armed=false;last=Date.now();if(current.current?.kind==='translate')closeSheet();else openSheet({kind:'translate'})}});const app=AppState.addEventListener('change',state=>{foreground=state==='active';armed=false});return()=>{subscription.remove();app.remove()}},[]);return null}
