import React,{useState,useRef} from 'react';
import {Text,View,Pressable,Switch} from 'react-native';
import {useRouter} from 'expo-router';
import {Screen} from '../../components/Screen';
import {Button,Card,Icon,Logo} from '../../components/ui';
import {MapView} from '../../components/map/MapView';
import {useLivePosition} from '../../components/map/useLivePosition';
import {isLongHold} from '../../domain/dk';
import {colors,text} from '../../theme';
import {useStore,openSheet,showToast} from '../../store';
import {propAddr,ownersOf,cname,yearsSince} from '../../domain';
import {TODAY} from '../../data/seed';
import {inDataBounds} from '../../data/properties';
import {useT} from '../../i18n';
export function DK(){
  const s=useStore(s=>s);const router=useRouter();const {l2}=useT();const live=useLivePosition();const gpsAt=!inDataBounds(live)?'p6':s.properties.reduce((closest,p)=>p.ll?.length===2&&Math.hypot(p.ll[0]-live.lat,p.ll[1]-live.lng)<Math.hypot((closest.ll?.[0]??0)-live.lat,(closest.ll?.[1]??0)-live.lng)?p:closest,s.properties.find(p=>p.id==='p6')||s.properties[0]).id;const hold=useRef(0);const started=useRef(false);const toggleGps=(value:boolean)=>{setGps(value);if(value)setAt(gpsAt);showToast(value?`GPS LIVE · ${propAddr(s.properties.find(x=>x.id===gpsAt)||s.properties[0])}`:l2('GPS OFF','GPS 已关闭'))};const [gps,setGps]=useState(true);const [at,setAt]=useState('p6');const current=gps?gpsAt:at;
  const p=s.properties.find(x=>x.id===current)||s.properties[0];
  const owner=ownersOf(s,p).map(cname).join(', ')||p.pfOwner||'No contact yet';
  const first=s.auth?.name.split(' ')[0]||'Owen';
  return <Screen head={<View style={{flexDirection:'row',alignItems:'center',flex:1,gap:10}}><Logo onPress={()=>router.push('/me')}/><Text style={[text,{fontWeight:'700',fontSize:17,flex:1}]}><Text style={{fontSize:11,color:colors.muted}}>{TODAY} · DK{'\n'}</Text>Hi {first}</Text><Text style={[text,{fontSize:11,fontWeight:'700'}]}>{gps?'GPS LIVE':'GPS OFF'}</Text><Switch value={gps} onValueChange={toggleGps} trackColor={{true:colors.green}}/></View>}>
    <Card style={{borderColor:gps?colors.green:colors.line,padding:16}}><Text style={[text,{fontSize:11,color:colors.muted,fontWeight:'700'}]}>{gps?"YOU'RE AT · LIVE GPS":"YOU'RE SEARCHING FOR"}</Text><Pressable onPress={()=>router.push(`/prop/${p.id}`)}><Text style={[text,{fontSize:19,fontWeight:'700'}]}>{propAddr(p)} ›</Text><Text style={[text,{fontSize:12}]}>{owner}{p.lastSale?` · owned ${yearsSince(p.lastSale)} years`:''}</Text><Text style={[text,{color:colors.muted,fontSize:11}]}>Tap address for property and ownership details</Text></Pressable>{!gps&&<Button size="sm" variant="ghost" onPress={()=>toggleGps(true)}>{l2('Back to GPS location','返回 GPS 位置')}</Button>}<Button size="sm" onPress={()=>openSheet({kind:'change',onPick:(id:string)=>{setAt(id);setGps(false)}})}>Change</Button></Card>
    <View style={{alignItems:'center',paddingVertical:28,gap:10}}><Pressable accessibilityLabel="Hold to speak" onPressIn={()=>{hold.current=Date.now();started.current=false}} onLongPress={()=>{started.current=true;openSheet({kind:'cmd',ctx:'dk',propId:p.id,record:true})}} delayLongPress={260} onPressOut={()=>{if(!started.current&& !isLongHold(Date.now()-hold.current))openSheet({kind:'cmd',ctx:'dk',propId:p.id})}} style={{width:104,height:104,borderRadius:52,backgroundColor:colors.mic,alignItems:'center',justifyContent:'center'}}><Icon name="mic" size={40} color="#fff"/></Pressable><Text style={[text,{fontWeight:'700',fontSize:17}]}>Hold to speak</Text><Text style={[text,{color:colors.muted,fontSize:12,textAlign:'center'}]}>Say anything: owner, mobile, tags, notes.{'\n'}{'We’ll file it for you to check.'}</Text></View>
    <Card><Pressable onPress={()=>router.push(`/prop/${p.id}`)}><Text style={[text,{fontWeight:'700'}]}>Tap the property above ›</Text><Text style={[text,{color:colors.muted,fontSize:12}]}>See everything about it, edit the record or add a note</Text></Pressable></Card>
    {s.captures.some(c=>c.status!=='approved')&&<Button onPress={()=>router.push('/captures')}>{s.captures.filter(c=>c.status!=='approved').length} captures waiting for your approval · Review ›</Button>}
    <Pressable onPress={()=>router.push('/fullmap')}><View pointerEvents="none"><MapView properties={s.properties} selected={p.id} onSelect={()=>{}} height={190}/></View><Text style={[text,{fontSize:12,textAlign:'right',color:colors.link}]}>Map · Full screen ↗</Text></Pressable>
  </Screen>;
}
