import React,{useState} from 'react';
import {Text,View} from 'react-native';
import {useLocalSearchParams,useRouter} from 'expo-router';
import {Screen} from '../../components/Screen';
import {Banner,Button,Card,Field,Toggle,Chip} from '../../components/ui';
import {useStore,update,showToast} from '../../store';
import {audit,cname,noteOn,shortAddr} from '../../domain';
import {TODAY} from '../../data/seed';
import {colors,text} from '../../theme';
const channels:[string,string][]=[['sms','Text'],['phone','Call'],['email','Email'],['letter','Letter'],['junk','Junk mail']];
export function Prefs(){const {id}=useLocalSearchParams<{id:string}>();const router=useRouter();const s=useStore(s=>s);const c=s.contacts.find(x=>x.id===id);const [values,setValues]=useState<Record<string,number>>(()=>Object.fromEntries(channels.map(([k])=>[k,c?.consent?.[k]??1])));const [reason,setReason]=useState('');if(!c)return <Screen title="Communication preferences" back><Banner>Contact not found</Banner></Screen>;const p=s.properties.find(p=>p.ownerIds.includes(c.id));return <Screen title="Communication preferences" back>
 <Text style={[text,{fontSize:12,color:colors.muted}]}>{cname(c)}{p?` · ${shortAddr(p)}`:''}</Text><Banner>Tap to opt out. Opted-out channels show in bold red on every record and their buttons are disabled. Logged with date and who changed it.</Banner>
 <Card>{channels.map(([k,label])=><View key={k} style={{flexDirection:'row',alignItems:'center',gap:8,paddingVertical:8,borderBottomWidth:1,borderColor:colors.line}}><View style={{flex:1}}><Text style={[text,{fontWeight:k==='junk'?'700':'400'}]}>{label}</Text>{k==='junk'&&<Text style={[text,{fontSize:11,color:colors.muted}]}>Letterbox drops, flyers, “No junk mail” sign</Text>}</View>{!values[k]&&<Chip kind="red">{k==='junk'?'NO JUNK MAIL':'OPTED OUT'}</Chip>}<Toggle value={!!values[k]} onValueChange={v=>setValues(o=>({...o,[k]:v?1:0}))}/></View>)}</Card>
 {c.prefLog&&<Text style={[text,{fontSize:12,color:colors.muted}]}>{c.prefLog.t} · {c.prefLog.d} · {c.prefLog.by}</Text>}
 <Field label="REASON (OPTIONAL)" value={reason} onChangeText={setReason} placeholder="No junk mail sign on letterbox"/>
 <View style={{flexDirection:'row',gap:8}}><Button style={{flex:1}} onPress={()=>router.back()}>Cancel</Button><Button style={{flex:1}} variant="primary" onPress={()=>{update(d=>{const x=d.contacts.find(x=>x.id===id)!;const changed=channels.filter(([k])=>values[k]!== (x.consent?.[k]??1));x.consent={...x.consent,...values};x.optedOut=channels.some(([k])=>values[k]===0);x.tags=x.tags.filter(t=>t!=='No junk mail');if(!values.junk)x.tags.push('No junk mail');const details=changed.map(([k,label])=>`${label}: ${values[k]?'opted in':'opted out'}`).join(' · ');if(details){x.prefLog={d:TODAY,by:d.auth?.name||'Owen Chen',t:details+(reason?' · '+reason:'')};noteOn(d,x,'Preferences · '+details+(reason?' · '+reason:''));audit(d,'Preferences',cname(x)+' · '+details)}});showToast('Preferences saved');router.back()}}>Save</Button></View>
 </Screen>}
