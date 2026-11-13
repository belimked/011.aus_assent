import React from 'react';
import {Text,View} from 'react-native';
import {useLocalSearchParams,useRouter} from 'expo-router';
import {Screen} from '../../components/Screen';
import {Banner,Button,Card,Chip,KV,SectionLabel,Toggle,ListItem} from '../../components/ui';
import {TagField} from '../../components/TagField';
import {useStore,update,openSheet} from '../../store';
import {audit,cname,mask,maskEmail,propAddr} from '../../domain';
import {duplicatesOf} from '../../domain/dk';
import {colors,text} from '../../theme';
export function Contact(){const {id}=useLocalSearchParams<{id:string}>();const s=useStore(s=>s);const router=useRouter();const c=s.contacts.find(c=>c.id===id);if(!c)return <Screen title="Contact" back><Banner>Contact not found</Banner></Screen>;const props=s.properties.filter(p=>p.ownerIds.includes(c.id));const off=Object.entries(c.consent||{}).filter(([,v])=>v===0).map(([k])=>k);const dup=duplicatesOf(s,c);
 return <Screen title={cname(c)} back><View style={{flexDirection:'row',alignItems:'center'}}><Text style={[text,{color:colors.muted,flex:1,fontSize:12}]}>Contact · {props[0]?`owner of ${propAddr(props[0])}`:''}{'\n'}Last updated {c.updated?.by}, {c.updated?.d}</Text><Text style={[text,{fontSize:11}]}>SELLER LEAD</Text><Toggle value={c.tags.includes('Seller lead')} onValueChange={v=>update(d=>{const x=d.contacts.find(x=>x.id===id)!;x.tags=v?[...new Set([...x.tags,'Seller lead'])]:x.tags.filter(t=>t!=='Seller lead');audit(d,'Seller lead',cname(x))})}/></View>
 <Card>{(['first','last','mobile','email','address'] as const).map(k=><ListItem key={k} title={k[0].toUpperCase()+k.slice(1)} subtitle={k==='mobile'?mask(c.mobile):k==='email'?maskEmail(c.email):c[k]|| (k==='address'&&props[0]?propAddr(props[0]):'—')} onPress={()=>openSheet({kind:'efield',cid:c.id,k})}/>)}</Card><Text style={[text,{fontSize:12,color:colors.muted}]}>Tap any field to edit · saved to mock Rex</Text>
 {off.length>0&&<Banner>OPTED OUT: {off.join(' · ').toUpperCase()}</Banner>}
 <SectionLabel>CONTACT PREFERENCES</SectionLabel><Card>{['sms','phone','email','letter','junk'].map(k=><View key={k} style={{flexDirection:'row',alignItems:'center'}}><Text style={[text,{flex:1}]}>{k==='junk'?'Junk mail / letterbox drops':k[0].toUpperCase()+k.slice(1)}</Text>{c.consent?.[k]===0&&<Chip kind="red">{k==='junk'?'NO JUNK MAIL':'OPTED OUT'}</Chip>}<Toggle value={c.consent?.[k]!==0} onValueChange={()=>router.push(`/prefs/${c.id}`)}/></View>)}</Card>
 <View style={{flexDirection:'row',gap:6,flexWrap:'wrap'}}><Button disabled={c.consent?.phone===0} onPress={()=>openSheet({kind:'call',cid:c.id})}>Call</Button><Button disabled={c.consent?.sms===0} onPress={()=>openSheet({kind:'text',cid:c.id})}>Text</Button><Button disabled={!c.email||c.consent?.email===0} onPress={()=>openSheet({kind:'email',cid:c.id})}>Email</Button><Button onPress={()=>openSheet({kind:'remind',cid:c.id,propId:props[0]?.id})}>+ To-do</Button></View>
 {dup.length>0&&<Card><SectionLabel>POSSIBLE DUPLICATE · ALSO YOURS</SectionLabel>{dup.map(x=><View key={x.id}><Text style={text}>{cname(x)} · {x.notes.length} notes · {x.updated?.d}</Text><Button variant="honey" onPress={()=>openSheet({kind:'merge',a:c.id,b:x.id})}>Compare and merge</Button></View>)}</Card>}
 <Card><TagField tags={c.tags} onChange={tags=>update(d=>{const x=d.contacts.find(x=>x.id===id)!;x.tags=tags;audit(d,'Tags',cname(x))})}/></Card>
 <Card><SectionLabel>HISTORY</SectionLabel>{[...c.notes.map(n=>({d:n.d,t:n.t})),...(c.history||[])].sort((a,b)=>b.d.localeCompare(a.d)).map((n,i)=><KV key={i} label={n.d} value={n.t}/>)}</Card>
 <Card><SectionLabel>RELATIONSHIPS</SectionLabel>{(c.rels||[]).map((r:{type:string;name:string},i:number)=><KV key={i} label={r.type} value={r.name}/>)}<Button onPress={()=>openSheet({kind:'rel',cid:c.id})}>+ Add relationship</Button></Card>
 <Card><SectionLabel>LINKED PROPERTIES</SectionLabel>{props.map(p=><ListItem key={p.id} title={propAddr(p)} onPress={()=>router.push(`/prop/${p.id}`)}/>)}</Card>
 </Screen>}
