import React,{useState} from 'react';
import {View,StyleSheet,Platform} from 'react-native';
import {AppleMaps,GoogleMaps,type CameraMoveEvent} from 'expo-maps';
import {visibleParcels,visibleProperties,DEMO_POSITION} from '../../data/properties';
import type {Property} from '../../data/types';
import {propColor} from '../../domain';
import {colors} from '../../theme';
import {useLivePosition} from './useLivePosition';
import {useStore} from '../../store';

const pos=(lat:number,lng:number)=>({latitude:lat,longitude:lng});
const tint:Record<string,string>={green:colors.green,orange:colors.accent,red:colors.red,sky:colors.sky,pending:colors.steel};
export function MapView({properties,selected,onSelect,satellite=false,height=400,onTap}:{properties:Property[];selected?:string;onSelect:(id:string)=>void;satellite?:boolean;height?:number;onTap?:(lat:number,lng:number)=>void}){
  const live=useLivePosition();const listings=useStore(s=>s.listings);
  const [view,setView]=useState({lat:DEMO_POSITION.lat,lng:DEMO_POSITION.lng,zoom:17,delta:.008});
  const bounds={south:view.lat-view.delta,north:view.lat+view.delta,west:view.lng-view.delta,east:view.lng+view.delta};
  const parcels=view.zoom>=17?visibleParcels(bounds):[];
  const background=view.zoom>=15?visibleProperties(bounds,250):[];
  const shapes=parcels.map(p=>({id:p.id,coordinates:p.ring.map(x=>pos(x.lat,x.lng)),color:'#0b5d8a11',lineColor:'#0b5d8a66',lineWidth:1}));
  const markers=[...properties.filter(p=>p.ll?.length===2&&(p.otherListing||p.otherAgent)).map(p=>({id:'agency-'+p.id,coordinates:pos(p.ll![0]+.00008,p.ll![1]),title:'Listed, other agency',tintColor:colors.accent})),...properties.filter(p=>p.ll?.length===2&&listings.some(l=>l.propertyId===p.id&&l.status==='listed')).map(p=>({id:'agency-'+p.id,coordinates:pos(p.ll![0]+.00008,p.ll![1]),title:'P · Listed with your agency',tintColor:colors.link})),{id:'you-live',coordinates:pos(live.lat,live.lng),title:'You · live',tintColor:colors.link},...background.map(p=>({id:'q-'+p.id,coordinates:pos(p.lat,p.lng),title:`${p.number} ${p.street}`,tintColor:'#8998a9'})),...properties.filter(p=>p.ll?.length===2).map(p=>({id:p.id,coordinates:pos(p.ll![0],p.ll![1]),title:`${p.no} ${p.street}${p.pinned?' ★':''}`,tintColor:tint[propColor(p)]}))];
  const move=(e:CameraMoveEvent)=>setView({lat:e.coordinates.latitude??DEMO_POSITION.lat,lng:e.coordinates.longitude??DEMO_POSITION.lng,zoom:e.zoom,delta:Math.max(e.latitudeDelta,e.longitudeDelta)/2});
  const camera={coordinates:pos(properties.find(p=>p.id===selected)?.ll?.[0]??DEMO_POSITION.lat,properties.find(p=>p.id===selected)?.ll?.[1]??DEMO_POSITION.lng),zoom:17};
  return <View style={{height,overflow:'hidden',borderRadius:12}}>{Platform.OS==='ios'?<AppleMaps.View style={StyleSheet.absoluteFill} cameraPosition={camera} markers={markers} polygons={shapes} onCameraMove={move} onMarkerClick={e=>{if(e.id!=='you-live')onSelect((e.id||'').replace(/^agency-/,''))}} onMapClick={e=>onTap?.(e.coordinates.latitude??DEMO_POSITION.lat,e.coordinates.longitude??DEMO_POSITION.lng)} properties={{mapType:satellite?AppleMaps.MapType.HYBRID:AppleMaps.MapType.STANDARD}}/>:<GoogleMaps.View style={StyleSheet.absoluteFill} cameraPosition={camera} markers={markers.map(({tintColor,...m})=>m)} circles={[{id:'you-live',center:pos(live.lat,live.lng),radius:8,color:colors.link,lineColor:'#fff'},...properties.filter(p=>p.ll?.length===2).map(p=>({id:p.id,center:pos(p.ll![0],p.ll![1]),radius:9,color:tint[propColor(p)]+'99',lineColor:tint[propColor(p)]}))]} polygons={shapes} onCameraMove={move} onMarkerClick={e=>{if(e.id!=='you-live')onSelect((e.id||'').replace(/^agency-/,''))}} onCircleClick={e=>{if(e.id!=='you-live')onSelect(e.id||'')}} onMapClick={e=>onTap?.(e.coordinates.latitude??DEMO_POSITION.lat,e.coordinates.longitude??DEMO_POSITION.lng)} properties={{mapType:satellite?GoogleMaps.MapType.HYBRID:GoogleMaps.MapType.NORMAL}}/>}</View>;
}
