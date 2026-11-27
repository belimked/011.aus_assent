import React,{useEffect,useMemo,useRef,useState} from 'react';
import {View} from 'react-native';
import {WebView,type WebViewMessageEvent,type WebViewProps} from 'react-native-webview';
import {DEFAULT_CENTER} from './defaultCenter';
import type {Property} from '../../data/types';
import {propColor} from '../../domain';
import {colors} from '../../theme';
import {useLivePosition} from './useLivePosition';
import {markerKind} from './mapMarks';
import {useStore} from '../../store';
import {MAP_HTML,type MapMark,type MapMsg,type MapState,type MapDrawing} from './mapPage';

// OpenStreetMap (Leaflet in a WebView) instead of Apple/Google Maps, matching the prototype's map and markers.
// baseUrl gives tile requests a Referer, which the OSM tile policy requires.
const BASE_URL='https://axleo-api.saulxj.ggff.net/';
const tint:Record<string,string>={green:colors.green,orange:colors.red,red:colors.red,sky:colors.sky,pending:colors.steel};
// react-native-webview 14 class typings resolve to never under React 19 types; the runtime component is fine.
const Web=WebView as unknown as React.ComponentType<WebViewProps&{ref?:React.Ref<WebView>}>;

// follow: with no selection, keep re-centring on the phone as it moves (about every 25 m) instead of only on the first fix.
// locate: bump it to jump back to the phone's position once (full-screen "My location" button).
export function MapView({properties,selected,onSelect,satellite=false,height=400,square=false,onTap,follow=false,locate=0,drawing}:{ properties:Property[];selected?:string;onSelect:(id:string)=>void;satellite?:boolean;height?:number;square?:boolean;onTap?:(lat:number,lng:number)=>void;follow?:boolean;locate?:number;drawing?:MapDrawing}){
  const live=useLivePosition().position;const listings=useStore(s=>s.listings);const company=useStore(s=>s.auth?.company);
  const web=useRef<WebView>(null);const [ready,setReady]=useState(false);
  const sel=properties.find(p=>p.id===selected&&p.ll?.length===2);
  // No selection: centre on the phone's real position; before the first fix, show the data area (no "you are here" marker).
  const center:[number,number]=sel?[sel.ll![0],sel.ll![1]]:live?[live.lat,live.lng]:[DEFAULT_CENTER.lat,DEFAULT_CENTER.lng];
  // The WebView re-centres when centerKey changes; mapPage keeps the view when a selection is merely closed.
  const centerKey=selected||(live?(follow?`me:${Math.round(live.lat*4000)},${Math.round(live.lng*4000)}`:`me#${locate}`):'init');
  const state=useMemo<MapState>(()=>{
    const placed=properties.filter(p=>p.ll?.length===2);
    const marks:MapMark[]=placed.map(p=>{const c=propColor(p);const listed=listings.some(l=>l.propertyId===p.id&&l.status==='listed');
      return {id:p.id,ll:[p.ll![0],p.ll![1]],no:p.no,color:tint[c]||colors.steel,kind:markerKind(p,listed,company)}});
    return {sat:satellite,selected,center,centerKey,locate,zoom:17,marks,me:live?[live.lat,live.lng]:null,labels:marks.length<=120,drawing};
  // center is derived from selected/properties
  // eslint-disable-next-line react-hooks/exhaustive-deps
  },[properties,listings,company,selected,satellite,live?.lat,live?.lng,centerKey,locate,drawing]);
  const lastSent=useRef('');
  useEffect(()=>{if(!ready)return;const js=JSON.stringify(state);if(js!==lastSent.current){lastSent.current=js;web.current?.injectJavaScript(`window.axUpdate(${js});true;`)}},[ready,state]);
  const onMessage=(e:WebViewMessageEvent)=>{let m:MapMsg;try{m=JSON.parse(e.nativeEvent.data)}catch{return}
    if(m.t==='ready'){lastSent.current='';setReady(true)}
    else if(m.t==='sel')onSelect(m.id);
    else if(m.t==='tap')onTap?.(m.lat,m.lng);};
  return <View style={{height,overflow:'hidden',borderRadius:square?0:12,backgroundColor:'#E9EEF3'}}>
    <Web ref={web} source={{html:MAP_HTML,baseUrl:BASE_URL}} originWhitelist={['*']} onMessage={onMessage} scrollEnabled={false} bounces={false} overScrollMode="never" setSupportMultipleWindows={false} javaScriptEnabled style={{flex:1,backgroundColor:'transparent'}}/>
  </View>;
}
