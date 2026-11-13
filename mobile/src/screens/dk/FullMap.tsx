import React,{useState} from 'react';
import {Text,TextInput,View,Platform} from 'react-native';
import {useRouter} from 'expo-router';
import {Screen} from '../../components/Screen';
import {MapView} from '../../components/map/MapView';
import {Button,Card,Chip,Dot,ListItem,Seg} from '../../components/ui';
import {useStore,openSheet} from '../../store';
import {colors,text} from '../../theme';
import {cname,ownersOf,propAddr,propColor,yearsSince} from '../../domain';
export function FullMap(){const s=useStore(s=>s);const router=useRouter();const [mode,setMode]=useState(Platform.OS==='web'?'list':'map');const [query,setQuery]=useState('');const [selected,setSelected]=useState('');const p=s.properties.find(p=>p.id===selected);const filtered=s.properties.filter(p=>propAddr(p).toLowerCase().includes(query.toLowerCase()));return <Screen title="Map" back>
  <TextInput accessibilityLabel="Search address or street" placeholder="Search address or street" value={query} onChangeText={setQuery} style={[text,{backgroundColor:'#fff',borderWidth:1,borderColor:colors.line,borderRadius:8,padding:12}]}/>
  <Seg items={[["map","Map"],["sat","Satellite"],["list","List"]]} value={mode} onChange={setMode}/>
  {mode==='list'?<Card>{filtered.map(x=><ListItem key={x.id} title={propAddr(x)} subtitle={`${ownersOf(s,x).map(cname).join(', ')||x.pfOwner||'No contact yet'} · ${x.lastSale?yearsSince(x.lastSale)+' years':'—'}`} right={<Dot color={propColor(x)}/>} onPress={()=>{setSelected(x.id);router.push(`/prop/${x.id}`)}}/>)}</Card>:<MapView properties={filtered} selected={selected||'p6'} onSelect={setSelected} satellite={mode==='sat'} height={520}/>}
  {p&&<Card><Text style={[text,{fontWeight:'800',fontSize:16}]}>{propAddr(p)}</Text><Text style={text}>{ownersOf(s,p).map(cname).join(', ')||p.pfOwner||'No contact yet'} · {p.lastSale?yearsSince(p.lastSale)+' years':'—'}</Text><View style={{flexDirection:'row',gap:8}}><Button variant="primary" onPress={()=>router.push(`/prop/${p.id}`)}>Open record ›</Button><Button onPress={()=>openSheet({kind:'note',propId:p.id})}>+ Note</Button></View></Card>}
  <View style={{flexDirection:'row',flexWrap:'wrap',gap:8}}>{[['green','Owner linked'],['orange','No contact yet'],['sky','Other agency'],['pending','Listed with your agency'],['red','Do not contact']].map(([kind,label])=><Chip key={kind} kind={kind}>{label}</Chip>)}<Chip kind="amber">★ Pinned by you</Chip></View>
  <Button block onPress={()=>router.back()}>Exit full screen</Button>
</Screen>}
