import React,{useEffect,useMemo,useRef,useState} from 'react';
import {Alert,ScrollView,Text,View,TextInput} from 'react-native';
import {useRouter} from 'expo-router';
import {mergeSuburbsZone,monitoredSuburbs} from '../domain/suburbs-zone';
import {properties as mapProperties} from '../data/properties';
import {areaProperties,suggestAreaName,tagArea,type AreaProperty} from '../domain/patch';
import {propertiesWithin} from '../store/sync';
import {Screen} from '../components/Screen';
import {MapView} from '../components/map/MapView';
import {areaPoints,canFinish,validAreaName} from '../components/map/patchDrawing';
import {Banner,Button,Card,Chip,SectionLabel} from '../components/ui';
import {colors,text} from '../theme';
import {useStoreKeys,update,showToast} from '../store';
import {useT} from '../i18n';

function AreaResults({pts,local,properties,l2}:{pts:number[][];local:AreaProperty[];properties:import('../data/types').Property[];l2:(en:string,zh:string)=>string}){
 const router=useRouter();const [remote,setRemote]=useState<{items:AreaProperty[];total:number;linked:number}|null>(null);
 const [offline,setOffline]=useState(false),[loading,setLoading]=useState(true);
 const polygon=JSON.stringify(pts);
 useEffect(()=>{let active=true;
  if(pts.length<3)return;
  propertiesWithin(pts).then(result=>{if(active)setRemote(result)}).catch(()=>{if(active)setOffline(true)}).finally(()=>{if(active)setLoading(false)});
  return ()=>{active=false};
 // polygon encodes the coordinates and avoids refetch on unrelated state updates.
 // eslint-disable-next-line react-hooks/exhaustive-deps
 },[polygon]);
 const items=remote?.items||local;const total=remote?.total??local.length;const linked=remote?.linked??local.filter(x=>x.hasContact).length;
 return <View style={{gap:6,marginVertical:8}}><Text style={text}>{l2('All addresses','全部地址')} {total} · {l2('Linked contacts','已关联联系人')} {linked}</Text>
  {(offline||loading)&&<Text style={[text,{color:colors.muted,fontSize:12}]}>{offline?l2('Offline · showing on-device addresses','离线 · 显示手机内置地址'):l2('Loading all addresses…','正在加载全部地址…')}</Text>}
  <ScrollView nestedScrollEnabled style={{maxHeight:280}}>{items.map(item=><Text key={item.id} onPress={()=>{if(properties.some(p=>p.id===item.id))router.push(`/prop/${item.id}`);else Alert.alert(`${item.no} ${item.street}, ${item.suburb}`,`${l2('Read-only address · no local property record','只读地址 · 本地没有房产档案')}\n${l2('Source','来源')}: ${item.source||'—'}\n${item.ll[0].toFixed(5)}, ${item.ll[1].toFixed(5)}`)}} style={[text,{paddingVertical:8,borderBottomWidth:1,borderColor:colors.line}]}>{item.no} {item.street}, {item.suburb}{item.hasContact?' · ●':''}</Text>)}</ScrollView>
  {remote&&items.length<total&&<Button size="sm" onPress={()=>{setLoading(true);propertiesWithin(pts,100,items.length).then(next=>setRemote(v=>v?{...v,items:[...v.items,...next.items]}:next)).catch(()=>setOffline(true)).finally(()=>setLoading(false))}}>{l2('Load more','加载更多')}</Button>}
 </View>
}

export function Patch(){
 const s=useStoreKeys('properties','contacts','zones','patch'),{l2}=useT(),router=useRouter();
 const [sub,setSub]=useState(''),[mode,setMode]=useState<'idle'|'draw'|'radius'>('idle');
 const [radius,setRadius]=useState(300),[center,setCenter]=useState<[number,number]|null>(null);
 const [locked,setLocked]=useState(false),[corners,setCorners]=useState<number[][]>([]);
 const [zoneName,setZoneName]=useState(''),[naming,setNaming]=useState(false),[opened,setOpened]=useState<string|null>(null);
 const nameRef=useRef<React.ElementRef<typeof TextInput>>(null);
 const [subs,setSubs]=useState(()=>monitoredSuburbs(s.zones,s.patch.suburbs));
 const all=[...new Set(s.properties.map(p=>p.suburb).concat(mapProperties.map(p=>p.suburb)))].filter(v=>!subs.includes(v));
 const pts=useMemo(()=>areaPoints(corners,center,radius),[corners,center,radius]);
 const local=useMemo(()=>areaProperties(s,pts),[s,pts]);
 const count=canFinish(corners,center)?local.length:0;
 const reset=()=>{setMode('idle');setCenter(null);setCorners([]);setLocked(false);setNaming(false)};
 const save=()=>{const name=zoneName.trim();if(!validAreaName(name)){showToast(l2('Area name needs at least one letter (not just numbers or emoji)','区域名称至少要包含一个字母或汉字，不能只有数字或表情'));nameRef.current?.focus();return}
  const commit=()=>{let tagged=0;update(d=>{d.zones.push({id:'z'+Date.now(),name,pts});tagged=tagArea(d,name,pts)});reset();setZoneName('');showToast(l2(`Area saved · ${name} · ${tagged} tagged`,`区域已保存 · ${name} · 已标记 ${tagged}`))};
  if(count===0)Alert.alert(l2('No properties in this area','区域内没有房产'),l2('Save this area with 0 properties?','仍要保存这个 0 套房产的区域吗？'),[{text:l2('Cancel','取消'),style:'cancel'},{text:l2('Save anyway','仍要保存'),onPress:commit}]);else commit();
 };
 return <Screen title="My patch" back>
  <Banner>{l2('Tap Draw area to mark corners, or Radius to select a centre.','点绘制区域标记角点，或选半径设置圆心。')}</Banner>
  <MapView properties={s.properties} onSelect={()=>{}} height={240} drawing={mode==='draw'?{points:corners,finished:locked}:mode==='radius'?{points:[],...(center?{radius:{center,metres:radius}}:{})}:undefined} onTap={(lat,lng)=>{if(locked)return;if(mode==='radius')setCenter([lat,lng]);else if(mode==='draw')setCorners(v=>[...v,[lat,lng]])}}/>
  {mode!=='idle'&&<Banner>{mode==='radius'?center?l2(`Radius ${radius} m · ${count} properties`,`半径 ${radius} 米 · ${count} 套房产`):l2('Tap the map to set the centre','点地图定圆心'):`${l2('Tap corners on the map','在地图上点选角点')} · ${corners.length} · ${count} ${l2('properties','套房产')}`}</Banner>}
  <View style={{flexDirection:'row',flexWrap:'wrap',gap:6}}>
   <Button onPress={()=>{reset();setMode('draw')}}>{l2('Draw area','绘制区域')}</Button>
   <Button onPress={()=>{reset();setMode('radius')}}>{l2('◯ Radius','◯ 半径')}</Button>
   {mode==='draw'&&corners.length>0&&!locked&&<Button onPress={()=>setCorners(v=>v.slice(0,-1))}>{l2('Undo point','撤销角点')}</Button>}
   <Button disabled={!canFinish(corners,center)||locked} variant="primary" onPress={()=>{setZoneName(suggestAreaName(s,pts));setLocked(true);setNaming(true);setTimeout(()=>nameRef.current?.focus(),100)}}>{mode==='radius'?l2(`Finish · ${radius} m radius`,`完成 · ${radius} 米半径`):l2(`Finish · ${corners.length} corners`,`完成 · ${corners.length} 个角点`)}</Button>
   <Button onPress={reset}>{l2('Cancel','取消')}</Button>
  </View>
  {mode==='radius'&&<View style={{flexDirection:'row',gap:6,flexWrap:'wrap'}}>{[200,300,500,1000].map(r=><Chip key={r} kind={radius===r?'blue':'steel'} onPress={()=>setRadius(r)}>{r===1000?'1 km':r+' m'}</Chip>)}</View>}
  {naming&&<AreaResults key={JSON.stringify(pts)} pts={pts} local={local} properties={s.properties} l2={l2}/>}
  {naming&&<Card><Text style={text}>{count} {l2('properties','套房产')}{mode==='radius'?` · ${radius} m`:` · ${corners.length} ${l2('corners','角点')}`}</Text><TextInput ref={nameRef} value={zoneName} onChangeText={setZoneName} placeholder={l2('Area name','区域名称')} style={[text,{backgroundColor:'#fff',padding:12,borderWidth:1,borderColor:colors.line,borderRadius:8}]}/><Button variant="primary" onPress={save}>{l2('Save patch','保存片区')}</Button></Card>}
  <Card>{s.zones.filter(z=>z.type!=='suburbs').map(z=>{const inside=areaProperties(s,z.pts||[]);const junk=inside.filter(p=>s.properties.find(x=>x.id===p.id)?.ownerIds?.some(id=>s.contacts.some(c=>c.id===id&&c.tags.includes('No junk mail')))).length;return <View key={z.id} style={{flexDirection:'row',alignItems:'center',gap:8}}><View style={{flex:1}}><Text style={text} onPress={()=>setOpened(opened===z.id?null:z.id)}>{z.name} · {inside.length} {l2('homes','户')} ▾</Text>{junk>0&&<Text style={[text,{color:colors.red,fontWeight:'700'}]}>{junk} NO JUNK MAIL</Text>}{opened===z.id&&<AreaResults key={z.id} pts={z.pts||[]} local={inside} properties={s.properties} l2={l2}/>}</View><Button size="sm" variant="ghost" onPress={()=>{const removed=z;const tagged=s.contacts.filter(c=>c.tags.includes('Area: '+z.name)).map(c=>c.id);update(d=>{d.zones=d.zones.filter(x=>x.id!==z.id);d.contacts.forEach(c=>{if(tagged.includes(c.id))c.tags=c.tags.filter(t=>t!=='Area: '+z.name)})});showToast(l2('Area deleted','区域已删除')+' · '+z.name,()=>update(d=>{d.zones.push(removed);d.contacts.forEach(c=>{if(tagged.includes(c.id)&&!c.tags.includes('Area: '+z.name))c.tags.push('Area: '+z.name)})}))}}>{l2('Delete','删除')}</Button></View>})}</Card>
  <SectionLabel>SUBURBS</SectionLabel><View style={{flexDirection:'row',gap:6,flexWrap:'wrap'}}>{subs.map(x=><Chip key={x} kind="blue" onPress={()=>setSubs(v=>v.filter(y=>y!==x))}>{x} ×</Chip>)}</View><TextInput value={sub} onChangeText={setSub} placeholder={l2('Add suburb','添加 suburb')} style={[text,{backgroundColor:'#fff',padding:12,borderWidth:1,borderColor:colors.line,borderRadius:8}]}/><View style={{flexDirection:'row',gap:6,flexWrap:'wrap'}}>{all.filter(x=>x.toLowerCase().includes(sub.toLowerCase())).map(x=><Chip key={x} onPress={()=>{setSubs(v=>[...v,x]);setSub('')}}>{x} +</Chip>)}</View><Text style={[text,{fontSize:12,color:colors.muted}]}>{l2('Watching new listings and new sales from Pricefinder, checked daily.','监控 Pricefinder 的新挂牌和新成交，每天检查。')}</Text><Button variant="primary" block onPress={()=>{update(d=>{d.patch.suburbs=subs;d.zones=mergeSuburbsZone(d.zones,d.auth?.email||'offline',subs)});showToast(l2('Patch saved · Pricefinder watches it daily','片区已保存 · Pricefinder 每天检查'));router.back()}}>{l2('Save patch','保存片区')}</Button>
 </Screen>
}
