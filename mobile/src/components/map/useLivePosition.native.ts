import {useEffect,useState} from 'react';
import * as Location from 'expo-location';
import {DEMO_POSITION} from '../../data/properties';
import {showToast} from '../../store';
let warned=false;
export function useLivePosition(enabled=true){
 const [position,setPosition]=useState(DEMO_POSITION);
 useEffect(()=>{if(!enabled)return;let active=true;let sub:Location.LocationSubscription|undefined;
  const fail=()=>{if(active&&!warned){warned=true;showToast('Location unavailable · showing demo position')}if(active)setPosition(DEMO_POSITION)};
  const timeout=<T,>(promise:Promise<T>):Promise<T>=>Promise.race([promise,new Promise<T>((_,reject)=>setTimeout(()=>reject(new Error('Location timeout')),8000))]);
  void (async()=>{try{const permission=await timeout(Location.requestForegroundPermissionsAsync());if(!permission.granted){fail();return}const current=await timeout(Location.getCurrentPositionAsync({accuracy:Location.Accuracy.Balanced}));if(!active)return;setPosition({lat:current.coords.latitude,lng:current.coords.longitude});sub=await timeout(Location.watchPositionAsync({accuracy:Location.Accuracy.Balanced,distanceInterval:5},loc=>{if(active)setPosition({lat:loc.coords.latitude,lng:loc.coords.longitude})}));if(!active)sub.remove()}catch{fail()}})();
  return ()=>{active=false;sub?.remove()};
 },[enabled]);return position;
}
