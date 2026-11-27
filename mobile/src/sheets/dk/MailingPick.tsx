import React,{useState} from 'react';
import {Alert,Platform,Text,View} from 'react-native';
import type {SheetProps} from '../types';
import {Banner,Button,Field,ListItem} from '../../components/ui';
import {useStore,update,closeSheet,showToast} from '../../store';
import {cname} from '../../domain';
import {addMailingList,channelLabel,contactLists,pickerOptions,unsubscribePrompt,type MailingList} from '../../domain/mailingLists';
import {useT} from '../../i18n';
import {colors,text} from '../../theme';

// mailingpick params: {kind:'mailingpick', cid}. Lists come from the cached catalogue only.
export function MailingPick({params}:SheetProps){const {l2}=useT();const s=useStore(s=>s);const [q,setQ]=useState('');const c=s.contacts.find(x=>x.id===params.cid);if(!c)return <Banner>{l2('Contact not found','未找到联系人')}</Banner>;
  const items=pickerOptions(s.mailingCatalog?.items||[],contactLists(c),q);
  const add=(l:MailingList)=>{const go=()=>{update(d=>{const x=d.contacts.find(x=>x.id===c.id);if(x)addMailingList(d,x,l.name)});closeSheet();showToast(l2(`Added to ${l.name}`,`已加入 ${l.name}`))};const p=unsubscribePrompt(c,l);if(!p)return go();
    const off=p.channels.map(k=>channelLabel(k,l2)).join(', ');const title=p.dnc?l2('Marked Do not contact','已标记为请勿联系'):l2('Unsubscribed','已退订');const msg=(p.dnc?l2(`${cname(c)} is marked Do not contact (all channels off).`,`${cname(c)} 已标记为请勿联系（所有渠道已关闭）。`):l2(`${cname(c)} has opted out of: ${off}.`,`${cname(c)} 已退订：${off}。`))+'\n'+l2(`Add to “${l.name}” anyway?`,`仍要加入“${l.name}”吗？`);
    if(Platform.OS==='web'){if(globalThis.confirm?.(title+'\n'+msg))go();return}
    Alert.alert(title,msg,[{text:l2('Cancel','取消'),style:'cancel'},{text:l2('Add anyway','仍要加入'),onPress:go}]);};
  return <View style={{gap:10}}><Text style={[text,{fontWeight:'700',fontSize:18}]}>{l2('Mailing lists','邮件列表')} · {cname(c)}</Text><Field autoFocus label={l2('SEARCH','搜索')} value={q} onChangeText={setQ} placeholder={l2('Search mailing lists','搜索邮件列表')}/>
    {items.length?<View>{items.map(l=><ListItem key={l.name} testID={`mailingpick-${l.name}`} title={l.name} subtitle={l.channel?channelLabel(l.channel,l2):undefined} onPress={()=>add(l)}/>)}</View>:<Text style={[text,{color:colors.muted}]}>{q.trim()?l2('No matching mailing lists','没有匹配的邮件列表'):l2('Already on every mailing list','已加入全部邮件列表')}</Text>}
    <Button onPress={closeSheet}>{l2('Cancel','取消')}</Button></View>}
