import React,{useState} from 'react';
import {Text,View} from 'react-native';
import {useRouter} from 'expo-router';
import {Screen} from '../components/Screen';
import {Button,Card,Chip,ListItem} from '../components/ui';
import {useStore,openSheet} from '../store';
import {useT} from '../i18n';
import {byId,cname,ownersOf,propAddr} from '../domain';
import {colors,text} from '../theme';

export function Data(){
 const s=useStore(x=>x),router=useRouter(),{t,l2}=useT();
 const [tab,setTab]=useState('schema');
 const tabs:[string,string][]=[['schema',l2('Schema','数据结构')],['contacts',l2('Contacts','联系人')],['leads',l2('Leads','线索')],['properties',l2('Properties','房产')],['listings',l2('Listings','挂牌')],['contracts',l2('Contracts','合同')]];
 const address=(id:string)=>{const p=byId(s.properties,id);return p?propAddr(p):'—'};
 const person=(id:string)=>{const c=byId(s.contacts,id);return c?cname(c):'—'};
 return <Screen title={l2('Data','数据')} back><View style={{flexDirection:'row',flexWrap:'wrap',gap:6}}>{tabs.map(([key,label])=><Chip key={key} kind={tab===key?'blue':'steel'} onPress={()=>setTab(key)}>{label}</Chip>)}</View>
 {tab==='schema'&&<><Card><Text style={[text,{fontWeight:'700'}]}>{l2('How records connect','记录关联')}</Text><Text style={text}>{l2('Contact → Contract → Listing → Property','联系人 → 合同 → 挂牌 → 房产')}</Text><Text style={[text,{color:colors.muted}]}>{l2('Buyers have a match profile and purchase history. Sellers link to properties and leads. OFI attendees and enquiries become contacts and leads after approval.','买家有匹配资料和购买记录。卖家关联房产和线索。开放日访客及咨询经审批后成为联系人和线索。')}</Text></Card><Card><Text style={[text,{fontWeight:'700'}]}>{l2('Rex integration','Rex 集成')}</Text><Text style={text}>{l2('Changes are queued for sync. Nothing reaches Rex until the agent approves it.','更改会排队等待同步。未经经纪人批准，任何内容都不会写入 Rex。')}</Text></Card></>}
 {tab==='contacts'&&<Card>{s.contacts.map(c=><ListItem key={c.id} title={cname(c)} subtitle={`${c.roles.join(', ')} · ${c.mobile} · ${c.tags.join(', ')}${c.optedOut?' · '+l2('OPTED OUT','已退订'):''}`} onPress={()=>router.push(`/contact/${c.id}`)}/>)}</Card>}
 {tab==='leads'&&<Card>{s.leads.map(l=><ListItem key={l.id} title={`${l2(l.type==='seller'?'Seller':'Buyer',l.type==='seller'?'卖家':'买家')} · ${person(l.contactId)}`} subtitle={`${l.propertyId?address(l.propertyId):'—'} · ${l.note||''} · ${l.source||''} · ${l.d||''}`} onPress={()=>router.push(`/contact/${l.contactId}`)}/>)}{!s.leads.length&&<Text style={text}>{t('none')}</Text>}</Card>}
 {tab==='properties'&&<Card>{s.properties.map(p=><ListItem key={p.id} title={propAddr(p)} subtitle={`${ownersOf(s,p).map(cname).join(', ')||'—'} · PF: ${p.pfOwner||'—'} · ${p.lastSale||'—'}`} onPress={()=>router.push(`/prop/${p.id}`)}/>)}</Card>}
 {tab==='listings'&&<Card>{s.listings.map(l=><View key={l.id} style={{flexDirection:'row',alignItems:'center',gap:8,borderBottomWidth:1,borderColor:colors.line}}><View style={{flex:1}}><ListItem title={`${address(l.propertyId)} · ${t(l.status)}`} subtitle={`${l.price} · ${l.agent} · OFI ×${l.ofi.length} · ${l.ofi.reduce((n:number,o:{attendees?:unknown[]})=>n+(o.attendees?.length||0),0)} ${t('attendees')}`} onPress={()=>router.push(`/prop/${l.propertyId}`)}/></View>{l.status==='listed'&&<Button size="sm" onPress={()=>openSheet({kind:'exchange',listingId:l.id})}>{t('exchange')}</Button>}</View>)}</Card>}
 {tab==='contracts'&&<Card>{s.contracts.map(k=>{const listing=byId(s.listings,k.listingId);const buyer=byId(s.contacts,k.buyerId);return <View key={k.id} style={{paddingVertical:10,borderBottomWidth:1,borderColor:colors.line,gap:4}}><ListItem title={`${listing?address(listing.propertyId):'—'} · ${k.price}`} subtitle={`${k.d} · ${t(k.status)}`} onPress={()=>listing&&router.push(`/prop/${listing.propertyId}`)}/><Text style={[text,{fontSize:12}]}>{t('buyer')} → {buyer?cname(buyer):'—'} ({t('history_buyer')}: {buyer?.history?.length||0})</Text><Text style={[text,{fontSize:12}]}>{t('seller')} → {k.sellerIds?.map(person).join(', ')||k.sellerName||'—'} → {t('prev_owner')}</Text>{buyer&&<Button size="sm" onPress={()=>router.push(`/contact/${buyer.id}`)}>{l2('Open buyer','查看买家')}</Button>}</View>})}{!s.contracts.length&&<Text style={text}>{t('none')}</Text>}</Card>}
 </Screen>;
}
