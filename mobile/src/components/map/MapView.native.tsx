import React,{useEffect,useMemo,useRef,useState} from 'react';
import {View} from 'react-native';
import {WebView,type WebViewMessageEvent,type WebViewProps} from 'react-native-webview';
import {visibleParcels,visibleProperties,DEMO_POSITION} from '../../data/properties';
import type {Property} from '../../data/types';
import {propColor} from '../../domain';
import {colors} from '../../theme';
import {useLivePosition} from './useLivePosition';
import {useStore} from '../../store';
import {MAP_HTML,type MapMark,type MapMsg,type MapState} from './mapPage';

// OpenStreetMap (Leaflet in a WebView) instead of Apple/Google Maps, matching the prototype's map and markers.
// baseUrl gives tile requests a Referer, which the OSM tile policy requires.
const BASE_URL='https://axleo-api.saulxj.ggff.net/';
const tint:Record<string,string>={green:colors.green,orange:colors.accent,red:colors.red,sky:colors.sky,pending:colors.steel};
// react-native-webview 14 class typings resolve to never under React 19 types; the runtime component is fine.
const Web=WebView as unknown as React.ComponentType<WebViewProps&{ref?:React.Ref<WebView>}>;
type View_={s:number;n:number;w:number;e:number;z:number};

export function MapView({properties,selected,onSelect,satellite=false,height=400,onTap}:{properties:Property[];selected?:string;onSelect:(id:string)=>void;satellite?:boolean;height?:number;onTap?:(lat:number,lng:number)=>void}){
  const live=useLivePosition().position;const listings=useStore(s=>s.listings);
  const web=useRef<WebView>(null);const [ready,setReady]=useState(false);const [view,setView]=useState<View_|null>(null);
  const sel=properties.find(p=>p.id===selected&&p.ll?.length===2);
  // No selection: centre on the phone's real position; before the first fix, show the data area (no "you are here" marker).
  const center:[number,number]=sel?[sel.ll![0],sel.ll![1]]:live?[live.lat,live.lng]:[DEMO_POSITION.lat,DEMO_POSITION.lng];
  const state=useMemo<MapState>(()=>{
    const b=view&&{south:view.s,north:view.n,west:view.w,east:view.e};
    const zoom=view?.z??17;
    const placed=properties.filter(p=>p.ll?.length===2);
    const marks:MapMark[]=placed.map(p=>{const c=propColor(p);const listed=listings.some(l=>l.propertyId===p.id&&l.status==='listed');
      return {id:p.id,ll:[p.ll![0],p.ll![1]],no:p.no,color:tint[c]||colors.steel,kind:p.flags?.dnc?'dnc':p.pinned?'pin':listed?'listed':(p.otherListing||p.otherAgent)?'other':c==='pending'?'pending':'dot'}});
    const taken=new Set(placed.map(p=>`${p.ll![0].toFixed(5)},${p.ll![1].toFixed(5)}`));
    const bg:[number,number][]=b&&zoom>=15?visibleProperties(b,250).filter(q=>!taken.has(`${q.lat.toFixed(5)},${q.lng.toFixed(5)}`)).map(q=>[q.lat,q.lng]):[];
    const parcels=b&&zoom>=17?visibleParcels(b).map(p=>p.ring.map(x=>[x.lat,x.lng] as [number,number])):[];
    return {sat:satellite,selected,center,centerKey:selected||(live?'me':'init'),zoom:17,marks,bg,parcels,me:live?[live.lat,live.lng]:null,labels:marks.length<=120};
  // center is derived from selected/properties
  // eslint-disable-next-line react-hooks/exhaustive-deps
  },[properties,listings,selected,satellite,view,live?.lat,live?.lng]);
  const lastSent=useRef('');
  useEffect(()=>{if(!ready)return;const js=JSON.stringify(state);if(js!==lastSent.current){lastSent.current=js;web.current?.injectJavaScript(`window.axUpdate(${js});true;`)}},[ready,state]);
  const onMessage=(e:WebViewMessageEvent)=>{let m:MapMsg;try{m=JSON.parse(e.nativeEvent.data)}catch{return}
    if(m.t==='ready'){lastSent.current='';setReady(true)}
    else if(m.t==='move')setView(v=>v&&v.s===m.s&&v.n===m.n&&v.w===m.w&&v.e===m.e&&v.z===m.z?v:{s:m.s,n:m.n,w:m.w,e:m.e,z:m.z});
    else if(m.t==='sel')onSelect(m.id);
    else if(m.t==='tap')onTap?.(m.lat,m.lng);};
  return <View style={{height,overflow:'hidden',borderRadius:12,backgroundColor:'#E9EEF3'}}>
    <Web ref={web} source={{html:MAP_HTML,baseUrl:BASE_URL}} originWhitelist={['*']} onMessage={onMessage} scrollEnabled={false} bounces={false} overScrollMode="never" setSupportMultipleWindows={false} javaScriptEnabled style={{flex:1,backgroundColor:'transparent'}}/>
  </View>;
}
