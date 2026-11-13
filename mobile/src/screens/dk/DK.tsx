import React,{useState} from 'react';
import {Text,View,Pressable,Switch} from 'react-native';
import {useRouter} from 'expo-router';
import {Screen} from '../../components/Screen';
import {Button,Card,Icon} from '../../components/ui';
import {MapView} from '../../components/map/MapView';
import {colors,text} from '../../theme';
import {useStore,openSheet} from '../../store';
import {propAddr,ownersOf,cname,yearsSince} from '../../domain';
import {TODAY} from '../../data/seed';
export function DK(){
  const s=useStore(s=>s);const router=useRouter();const [gps,setGps]=useState(true);const [at,setAt]=useState('p6');
  const p=s.properties.find(x=>x.id===at)||s.properties[0];
  const owner=ownersOf(s,p).map(cname).join(', ')||p.pfOwner||'No contact yet';
  const first=s.auth?.name.split(' ')[0]||'Owen';
  return <Screen head={<View style={{flexDirection:'row',alignItems:'center',flex:1,gap:10}}><Text style={[text,{fontWeight:'800',fontSize:17,flex:1}]}><Text style={{fontSize:11,color:colors.muted}}>{TODAY} · DK{'\n'}</Text>Hi {first}</Text><Text style={[text,{fontSize:11,fontWeight:'700'}]}>{gps?'GPS LIVE':'GPS OFF'}</Text><Switch value={gps} onValueChange={setGps} trackColor={{true:colors.green}}/></View>}>
    <Card style={{borderColor:gps?colors.green:colors.line,padding:16}}><Text style={[text,{fontSize:11,color:colors.muted,fontWeight:'700'}]}>{gps?"YOU'RE AT · LIVE GPS":"YOU'RE SEARCHING FOR"}</Text><Pressable onPress={()=>router.push(`/prop/${p.id}`)}><Text style={[text,{fontSize:19,fontWeight:'800'}]}>{propAddr(p)} ›</Text><Text style={[text,{fontSize:12}]}>{owner}{p.lastSale?` · owned ${yearsSince(p.lastSale)} years`:''}</Text><Text style={[text,{color:colors.muted,fontSize:11}]}>Tap address for property and ownership details</Text></Pressable><Button size="sm" onPress={()=>openSheet({kind:'change',onPick:setAt})}>Change</Button></Card>
    <View style={{alignItems:'center',paddingVertical:28,gap:10}}><Pressable accessibilityLabel="Hold to speak" onPress={()=>openSheet({kind:'cmd',ctx:'dk',propId:p.id})} style={{width:104,height:104,borderRadius:52,backgroundColor:colors.mic,alignItems:'center',justifyContent:'center'}}><Icon name="mic" size={40} color="#fff"/></Pressable><Text style={[text,{fontWeight:'800',fontSize:17}]}>Hold to speak</Text><Text style={[text,{color:colors.muted,fontSize:12,textAlign:'center'}]}>Say anything: owner, mobile, tags, notes.{'\n'}{'We’ll file it for you to check.'}</Text></View>
    <Card><Pressable onPress={()=>router.push(`/prop/${p.id}`)}><Text style={[text,{fontWeight:'700'}]}>Tap the property above ›</Text><Text style={[text,{color:colors.muted,fontSize:12}]}>See everything about it, edit the record or add a note</Text></Pressable></Card>
    {s.captures.length>0&&<Button onPress={()=>router.push('/captures')}>{s.captures.length} captures waiting for your approval · Review ›</Button>}
    <Pressable onPress={()=>router.push('/fullmap')}><MapView properties={s.properties} selected={p.id} onSelect={id=>router.push(`/prop/${id}`)} height={190}/><Text style={[text,{fontSize:12,textAlign:'right',color:colors.link}]}>Map · Full screen ↗</Text></Pressable>
  </Screen>;
}
