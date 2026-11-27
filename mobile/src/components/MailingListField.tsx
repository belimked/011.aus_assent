import React,{useEffect} from 'react';
import {Text,View} from 'react-native';
import {Button,Chip,SectionLabel} from './ui';
import {RexSyncLabel} from './RexSyncLabel';
import {useStore,update,openSheet,showToast} from '../store';
import {refreshMailingCatalog} from '../store/sync';
import {contactLists,mailingReady,mailingSync,pickerOptions,removeMailingList} from '../domain/mailingLists';
import type {Contact} from '../data/types';
import {useT} from '../i18n';
import {colors,text} from '../theme';

// Same layout as TagField: label, blue chips (tap = remove), small "+" button that opens the picker sheet.
export function MailingListField({c}:{c:Contact}){const {l2}=useT();const catalog=useStore(s=>s.mailingCatalog);useEffect(()=>{void refreshMailingCatalog()},[]);
  const lists=contactLists(c);const ready=mailingReady(c);const left=pickerOptions(catalog?.items||[],lists,'').length;const muted=[text,{fontSize:12,color:colors.muted}];
  const remove=(name:string)=>{update(d=>{const x=d.contacts.find(x=>x.id===c.id);if(x)removeMailingList(d,x,name)});showToast(l2(`Removed from ${name}`,`已从 ${name} 移除`))};
  const empty=!ready?l2('Not in Rex yet. Mailing lists can be added once this contact is in Rex.','尚未同步到 Rex。联系人进入 Rex 后才能加入邮件列表。'):!catalog?l2('Mailing lists not synced yet. They load the next time you are online.','邮件列表尚未同步，下次联网时自动加载。'):!catalog.items.length?l2(catalog.at?'No mailing lists in Rex.':'Mailing lists not synced from Rex yet.',catalog.at?'Rex 中没有邮件列表。':'邮件列表尚未从 Rex 同步。'):!left?l2('On every mailing list.','已加入全部邮件列表。'):'';
  return <View style={{gap:8}}><SectionLabel>{l2('MAILING LISTS','邮件列表')}</SectionLabel>{lists.length?<View style={{gap:6}}>{lists.map(n=><View key={n} style={{flexDirection:'row',alignItems:'center',flexWrap:'wrap',gap:8}}><Chip kind="blue" testID={`mailing-remove-${n}`} onPress={()=>remove(n)}>{n} ×</Chip>{ready&&<RexSyncLabel value={mailingSync(c,n)}/>}</View>)}</View>:<Text style={muted}>{l2('Not on any mailing list.','未加入任何邮件列表。')}</Text>}
    {empty?<Text style={muted}>{empty}</Text>:<Button size="sm" testID="mailing-add" onPress={()=>openSheet({kind:'mailingpick',cid:c.id})}>{l2('+ Add mailing list','+ 添加邮件列表')}</Button>}</View>;
}
