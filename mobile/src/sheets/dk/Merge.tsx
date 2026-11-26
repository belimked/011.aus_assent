import React,{useEffect,useState} from 'react';
import {ActivityIndicator,Text,View} from 'react-native';
import {useRouter} from 'expo-router';
import type {SheetProps} from '../types';
import {Banner,Button,Card,Chip,SectionLabel} from '../../components/ui';
import {dismissSheet,showToast} from '../../store';
import {contactMerge,contactMergePreview,isOnline} from '../../store/sync';
import {useT} from '../../i18n';
import {colors,text} from '../../theme';
import {IRREVERSIBLE,confirmLabel,consentLines,dncLine,fieldLabel,fieldReason,fieldValue,offlineMessage,primaryReason,relationLines,rexLine,side,type MergePreview} from '../../domain/mergePreview';

const muted={...text,fontSize:12,color:colors.muted};
const NOTES_SHOWN=5;

/** Merge two contacts: the server's preview is shown in full, the user confirms, the merge cannot be undone. */
export function Merge({params}:SheetProps){
  const {l2}=useT();const router=useRouter();
  const [preview,setPreview]=useState<MergePreview|null>(null);
  const [error,setError]=useState('');const [busy,setBusy]=useState(false);
  const online=isOnline();const [attempt,setAttempt]=useState(0);
  useEffect(()=>{
    if(!online)return;
    let live=true;
    contactMergePreview(String(params.a),String(params.b)).then(p=>{if(live)setPreview(p)}).catch(()=>{if(live)setError(l2('Could not load the merge preview. Check your connection and try again.','无法加载合并预览，请检查网络后重试。'))});
    return()=>{live=false};
  },[online,params.a,params.b,l2,attempt]);
  const retry=()=>{setError('');setAttempt(n=>n+1)};
  if(!online)return <View style={{gap:10}}><Text style={title}>{l2('Merge contacts','合并联系人')}</Text><Banner>{offlineMessage(l2)}</Banner></View>;
  if(error)return <View style={{gap:10}}><Text style={title}>{l2('Merge contacts','合并联系人')}</Text><Banner>{error}</Banner><Button onPress={retry}>{l2('Try again','重试')}</Button></View>;
  if(!preview)return <View style={{gap:10,alignItems:'center',paddingVertical:24}}><ActivityIndicator color={colors.muted}/><Text style={muted}>{l2('Working out the merged record…','正在计算合并结果…')}</Text></View>;
  const p=preview;const [kept,removed]=p.contacts;
  const notes=p.notes.filter(n=>!n.log);const log=p.notes.find(n=>n.log);
  const merge=async()=>{
    if(busy)return;
    if(!isOnline()){showToast(offlineMessage(l2));return;}
    setBusy(true);
    try{
      const result=await contactMerge(p.primaryId,p.secondaryId,p.previewHash);
      if('stale' in result){setPreview(result.stale);showToast(l2('These contacts changed since the preview. Check the updated result.','预览后联系人有变动，请核对最新结果。'));return;}
      dismissSheet();
      showToast(l2('Contacts merged','联系人已合并'));
      router.push(`/contact/${result.primaryId}`);
    }catch{
      showToast(l2('Merge failed. Nothing was changed. Check your connection and try again.','合并失败，未做任何修改。请检查网络后重试。'));
    }finally{setBusy(false);}
  };
  return <View style={{gap:10}}>
    <Text style={title}>{l2('Merge contacts','合并联系人')}</Text>
    <Card>
      <Text style={[text,{fontWeight:'700'}]}>A · {kept.name||'—'} <Text style={muted}>{l2('kept','保留')}</Text></Text>
      <Text style={[text,{fontWeight:'700'}]}>B · {removed.name||'—'} <Text style={muted}>{l2('merged into A, then removed','并入 A 后删除')}</Text></Text>
      <Text style={muted}>{primaryReason(p,l2)}</Text>
    </Card>
    <SectionLabel>{l2('DETAILS · NEWEST VALUE WINS, BLANKS NEVER OVERWRITE','主信息 · 较新的值优先，空值不覆盖')}</SectionLabel>
    {p.fields.map(f=><View key={f.key} style={{flexDirection:'row',justifyContent:'space-between',gap:12,paddingVertical:6,borderBottomWidth:1,borderColor:colors.line}}>
      <Text style={muted}>{fieldLabel(f.key,l2)}</Text>
      <View style={{flexShrink:1,alignItems:'flex-end'}}>
        <Text style={[text,{fontSize:13,fontWeight:'600',textAlign:'right'}]}>{fieldValue(f.key,f.value)}</Text>
        <Text style={[muted,{fontSize:11}]}>{fieldReason(f,p,l2)}{f.reason==='newer'?` · ${l2('was','原为')} ${fieldValue(f.key,side(p,f.from)==='A'?f.b:f.a)}`:''}</Text>
      </View>
    </View>)}
    <SectionLabel>{l2('CONTACT PERMISSIONS · STRICTER CHOICE KEPT','联系许可 · 取更严格的一方')}</SectionLabel>
    <Text style={text}>{dncLine(p,l2)}</Text>
    {consentLines(p,l2).map(line=><Text key={line} style={text}>{line}</Text>)}
    {p.tags.length>0&&<><SectionLabel>{l2('TAGS · COMBINED','标签 · 合并')}</SectionLabel><View style={{flexDirection:'row',flexWrap:'wrap',gap:6}}>{p.tags.map(tag=><Chip key={tag}>{tag}</Chip>)}</View></>}
    {p.roles.length>0&&<><SectionLabel>{l2('ROLES · COMBINED','角色 · 合并')}</SectionLabel><Text style={text}>{p.roles.join(', ')}</Text></>}
    {p.properties.length>0&&<><SectionLabel>{l2('PROPERTIES · NEWEST RELATION WINS','房产关系 · 以最新为准')}</SectionLabel>{relationLines(p,l2).map(line=><Text key={line} style={text}>{line}</Text>)}</>}
    <SectionLabel>{l2(`NOTES · ${notes.length} COMBINED, NEWEST FIRST`,`备注 · 合并 ${notes.length} 条，最新在前`)}</SectionLabel>
    {log&&<Card style={{backgroundColor:colors.surface2}}><Text style={muted}>{l2('Added to A','将添加到 A')} · {log.d}</Text><Text style={text}>{log.t}</Text></Card>}
    {notes.slice(0,NOTES_SHOWN).map((n,i)=><View key={i} style={{gap:2}}><Text style={muted}>{n.d} · {n.a} · {side(p,n.from)}</Text><Text style={text} numberOfLines={3}>{n.t}</Text></View>)}
    {notes.length>NOTES_SHOWN&&<Text style={muted}>{l2(`and ${notes.length-NOTES_SHOWN} more`,`另有 ${notes.length-NOTES_SHOWN} 条`)}</Text>}
    <SectionLabel>REX</SectionLabel>
    <Text style={muted}>{rexLine(p,l2)}</Text>
    <Card style={{borderColor:colors.red}}>
      <Text style={[text,{fontWeight:'700',color:colors.red}]}>{l2(...IRREVERSIBLE)}</Text>
      <Text style={muted}>{l2('B is removed and its notes, to-dos and links move to A. There is no undo; check the result above first.','B 将被删除，其备注、待办和关联转到 A。无法撤销，请先核对上面的结果。')}</Text>
    </Card>
    <Button testID="merge-confirm" variant="danger" disabled={busy} onPress={()=>void merge()}>{confirmLabel(busy,l2)}</Button>
    <Button variant="ghost" disabled={busy} onPress={dismissSheet}>{l2('Cancel','取消')}</Button>
  </View>;
}
const title={...text,fontSize:18,fontWeight:'700' as const};
