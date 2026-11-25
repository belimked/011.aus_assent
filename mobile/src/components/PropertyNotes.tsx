import React,{useState} from 'react';
import {Text,TextInput,View} from 'react-native';
import type {Property} from '../data/types';
import {useStore,update,showToast} from '../store';
import {cname,ownersOf} from '../domain';
import {noteIdentity,noteIdentityLabel,hasNoteIdentity,matchingNoteContacts,savePropertyTranscript,type NoteIdentity} from '../domain/propertyNotes';
import {aiExtract} from '../lib/voice';
import {getVoiceLang} from '../lib/voicePref';
import {isOnline} from '../store/sync';
import {useT} from '../i18n';
import {colors,text} from '../theme';
import {Button,Card,SectionLabel} from './ui';
import {NoteMic} from './NoteMic';

export function PropertyNotes({property:p}:{property:Property}){
  const s=useStore(s=>s);const {l2,lang}=useT();const [note,setNote]=useState('');const [all,setAll]=useState(false);
  const [pending,setPending]=useState<{transcript:string;identity:NoteIdentity}|null>(null);
  const [selected,setSelected]=useState('');const [busy,setBusy]=useState(false);
  const notes=[...(p.notes||[]).map(n=>({...n,who:l2('Property','房产')})),...ownersOf(s,p).flatMap(c=>c.notes.map(n=>({...n,at:'',who:`${l2('Owner','业主')}: ${cname(c)}`})))].sort((a,b)=>(b.at||b.d).localeCompare(a.at||a.d));
  const save=(transcript:string,identity?:NoteIdentity,pick?:string)=>{try{update(d=>{savePropertyTranscript(d,p.id,transcript,identity,pick)});setNote('');setPending(null);setSelected('');showToast(l2('Saved','已保存'))}catch(e){showToast(String(e))}};
  const transcribed=async(transcript:string)=>{
    setNote(transcript);setBusy(true);
    let identity=noteIdentity(transcript);
    if(isOnline())try{identity=noteIdentity(transcript,await aiExtract(transcript,'dk',getVoiceLang(lang)))}catch{showToast(l2('Used offline parser','已使用离线解析'))}
    setBusy(false);
    if(hasNoteIdentity(identity)){setPending({transcript,identity});setSelected('')}
    else save(transcript);
  };
  const matches=pending?matchingNoteContacts(s,pending.identity.mobile):[];
  return <><SectionLabel>NOTES</SectionLabel>{notes.length>3&&!all&&<Button onPress={()=>setAll(true)}>See all {notes.length} ›</Button>}
    <View style={{flexDirection:'row',alignItems:'center',gap:6}}><TextInput testID="property-note" accessibilityLabel={l2('Add a note','添加备注')} placeholder={l2('Add a note…','添加备注…')} value={note} onChangeText={setNote} style={[text,{flex:1,minWidth:0,borderWidth:1,borderColor:colors.line,padding:8,borderRadius:8}]}/><NoteMic value={note} onChangeText={setNote} onTranscript={v=>void transcribed(v)}/><Button testID="property-note-save" variant="primary" disabled={busy||!!pending} onPress={()=>{if(note.trim())save(note.trim())}}>Add</Button></View>
    {busy&&<Text style={text}>{l2('Reading transcript…','正在识别语音…')}</Text>}
    {pending&&<Card><Text style={text}>{l2('Contact heard in voice note. Confirm before linking as owner:','语音中识别到联系人，确认后关联为业主：')}</Text><Text style={text}>{noteIdentityLabel(pending.identity)}</Text>
      {matches.map(c=><Button key={c.id} onPress={()=>setSelected(c.id)}>{selected===c.id?'✓ ':''}{cname(c)} · {c.mobile}</Button>)}
      {matches.length>1&&<Text style={text}>{l2('Choose the correct contact','请选择正确的联系人')}</Text>}
      <Button testID="property-note-confirm" variant="primary" disabled={matches.length>1&&!selected} onPress={()=>save(pending.transcript,pending.identity,selected||undefined)}>{matches.length?l2('Link owner and save note','关联业主并保存备注'):l2('Create owner and save note','新建业主并保存备注')}</Button>
      <Button onPress={()=>save(pending.transcript)}>{l2('Save note only','只保存备注')}</Button><Button onPress={()=>setPending(null)}>{l2('Edit note instead','返回编辑备注')}</Button></Card>}
    {(all?notes:notes.slice(0,3)).map((n,i)=><Card key={i}><Text style={[text,{fontSize:11,color:colors.muted}]}>{n.d} · {n.a} · {n.who}</Text><Text style={text}>{n.t}</Text></Card>)}
  </>;
}
