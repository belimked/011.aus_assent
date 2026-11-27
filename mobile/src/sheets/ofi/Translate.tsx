import React,{useCallback,useEffect,useRef,useState} from 'react';
import {ActivityIndicator,Platform,Pressable,Text,TextInput,View} from 'react-native';
import AsyncStorage from '@react-native-async-storage/async-storage';
import {useSafeAreaInsets} from 'react-native-safe-area-context';
import NativeRecorder,{type RecorderPhase} from '../../components/Recorder';
import {today} from '../../lib/today';
import {byId,noteOn} from '../../domain';
import {useT} from '../../i18n';
import {speak,stopSpeaking,translate,voiceLang,voiceErrorText,type SheetLang} from '../../lib/voice';
import {closeSheet,showToast,update} from '../../store';
import {Confetti} from './Confetti';

const languages:[SheetLang,string][]=[['en-AU','EN'],['zh-CN','普通话'],['yue-HK','廣東話']];
const honey='#FFB02E',azure='#2B95F5',orange='#FF6B2C',ink='#18283E',muted='#516176';

type Who='visitor'|'agent';
type Stage='idle'|'listening'|'recognising'|'translating'|'speaking';

// Each half faces a different person, so its prompts are written in that half's language, not the app language.
const SIDE_TEXT:Record<SheetLang,{
  hint:string;hold:string;release:string;placeholder:string;send:string;heard:string;replay:string;
  listening:string;recognising:string;translating:string;speaking:string;otherSpeaking:string;
}>={
  'en-AU':{
    hint:'Hold the button and speak English. Release and it is translated for the other person.',
    hold:'Hold · speak English',release:'Release to translate',placeholder:'Or type here',send:'Send',
    heard:'You said',replay:'Play again',
    listening:'Listening… release when you finish',recognising:'Recognising what you said…',
    translating:'Translating…',speaking:'Playing the translation',otherSpeaking:'The other person is speaking…',
  },
  'zh-CN':{
    hint:'按住下面的按钮说普通话，松开后自动翻译给对方。',
    hold:'按住 · 说普通话',release:'松开即翻译',placeholder:'也可以在这里打字',send:'发送',
    heard:'你说的是',replay:'再听一遍',
    listening:'正在听…说完松开',recognising:'正在识别你说的话…',
    translating:'正在翻译…',speaking:'正在播放翻译',otherSpeaking:'对方正在说话…',
  },
  'yue-HK':{
    hint:'撳住下面個掣講廣東話，放手就會翻譯俾對方。',
    hold:'撳住 · 講廣東話',release:'放手即翻譯',placeholder:'都可以喺度打字',send:'發送',
    heard:'你講嘅係',replay:'再聽一次',
    listening:'聽緊…講完放手',recognising:'識別緊你講嘅嘢…',
    translating:'翻譯緊…',speaking:'播緊翻譯',otherSpeaking:'對方講緊嘢…',
  },
};

const GREETING:Record<SheetLang,string>={
  'en-AU':'Hello! Let’s talk.',
  'zh-CN':'你好！这位经纪人还在学中文 😄',
  'yue-HK':'你好！呢位經紀仲喺度學中文 😄',
};

export function TranslateSheet(){
  const {t,l2}=useT();
  const insets=useSafeAreaInsets();
  const [warming,setWarming]=useState(false);
  const [langs,setLangs]=useState<Record<Who,SheetLang>>({visitor:'zh-CN',agent:'en-AU'});
  const [typed,setTyped]=useState<Record<Who,string>>({visitor:'',agent:''});
  // heard[who] = what that person said; shown[who] = translation shown on that person's half.
  const [heard,setHeard]=useState<Record<Who,string>>({visitor:'',agent:''});
  const [shown,setShown]=useState<Record<Who,string>>({visitor:'',agent:''});
  const [status,setStatus]=useState<{who:Who;stage:Stage}>({who:'agent',stage:'idle'});
  const working=useRef(false);
  const alive=useRef(true);

  useEffect(()=>{
    alive.current=true;
    let timer:ReturnType<typeof setTimeout>;
    void AsyncStorage.getItem('ax_translate_seen').then(seen=>{
      if(!alive.current||seen)return;
      setWarming(true);
      void AsyncStorage.setItem('ax_translate_seen','1');
      timer=setTimeout(()=>{if(alive.current)setWarming(false)},1500);
    });
    return()=>{alive.current=false;clearTimeout(timer);stopSpeaking()};
  },[]);

  const busy=status.stage!=='idle';
  const other=(who:Who):Who=>who==='visitor'?'agent':'visitor';
  const errorToast=(error:unknown)=>showToast(voiceErrorText(error,l2));
  const setStage=(who:Who,stage:Stage)=>{if(alive.current)setStatus({who,stage})};

  const onPhase=useCallback((who:Who,phase:RecorderPhase)=>{
    if(working.current)return;
    if(phase==='recording'){stopSpeaking();setStatus({who,stage:'listening'});}
    else if(phase==='uploading')setStatus({who,stage:'recognising'});
    else setStatus(s=>s.who===who&&(s.stage==='listening'||s.stage==='recognising')?{who,stage:'idle'}:s);
  },[]);
  const onVisitorPhase=useCallback((p:RecorderPhase)=>onPhase('visitor',p),[onPhase]);
  const onAgentPhase=useCallback((p:RecorderPhase)=>onPhase('agent',p),[onPhase]);

  const send=async(who:Who,raw:string)=>{
    const text=raw.trim();
    if(!text||working.current)return;
    working.current=true;
    stopSpeaking();
    const to=other(who);
    const fromLang=voiceLang(langs[who]),toLang=voiceLang(langs[to]);
    setHeard(h=>({...h,[who]:text}));
    setShown(s=>({...s,[to]:''}));
    setStage(who,'translating');
    try{
      const result=await translate(text,fromLang,toLang);
      if(!alive.current)return;
      setShown(s=>({...s,[to]:result.text}));
      setStage(who,'speaking');
      try{await speak(result.text,toLang)}catch(e){if(alive.current)errorToast(e)}
    }catch(e){
      if(alive.current)errorToast(e);
    }finally{
      working.current=false;
      setStage(who,'idle');
    }
  };

  const replay=(text:string,lang:SheetLang)=>{void speak(text,voiceLang(lang)).catch(errorToast)};

  const save=()=>{
    const content=[shown.agent,shown.visitor].filter(Boolean).join(' / ');
    if(!content)return;
    let filed=false;
    update(s=>{
      const l=s.listings.find(x=>x.ofi.some(o=>o.d===today()));
      const attendee=l?.ofi.find(o=>o.d===today())?.attendees[0];
      const c=attendee&&byId(s.contacts,attendee.contactId);
      if(c){noteOn(s,c,'Talk to me: '+content);filed=true}
    });
    if(filed){closeSheet();showToast(t('filed_ok'))}
    else showToast(l2('No open-home attendee to save to.','没有可保存备注的开放日访客。'));
  };

  if(warming)return <View style={{flex:1}}>
    <Pressable onPress={()=>setWarming(false)} style={{flex:1,backgroundColor:honey,alignItems:'center',justifyContent:'center',gap:18}}>
      <Text style={{fontSize:52}}>🫨</Text>
      <Text style={{fontSize:35,fontWeight:'700',color:'#29221B'}}>{l2('Whoa, steady!','哇，稳住！')}</Text>
      <Text style={{fontSize:18,color:'#29221B'}}>{t('tr_warming')}</Text>
      <Text style={{marginTop:30,color:'#29221B'}}>{t('tr_skip')}</Text>
    </Pressable>
    <Confetti run/>
  </View>;

  const side=(who:Who)=>{
    const top=who==='visitor';
    const color=top?azure:orange;
    const lang=langs[who];
    const s=SIDE_TEXT[lang];
    const mine=status.who===who;
    // Status line on this half: my own progress, or a heads-up that the other person is talking.
    const statusText=!busy?'':mine
      ?{listening:s.listening,recognising:s.recognising,translating:s.translating,speaking:'',idle:''}[status.stage]
      :status.stage==='speaking'?s.speaking:s.otherSpeaking;
    const waitingForMe=busy&&!mine&&status.stage!=='speaking';
    return <View style={{flex:1,backgroundColor:top?'#E7F4FF':'#FFF0E7',padding:18,justifyContent:'space-between',transform:top?[{rotate:'180deg'}]:[]}}>
      <View style={{gap:6}}>
        <View style={{flexDirection:'row',justifyContent:'space-between',alignItems:'center'}}>
          <Text style={{fontSize:18,fontWeight:'700',color}}>{top?t('tr_visitor'):t('tr_agent')}</Text>
          <View style={{flexDirection:'row',gap:4}}>
            {languages.map(([code,label])=><Pressable testID={`translate-${who}-${code}`} key={code} disabled={busy} hitSlop={4}
              onPress={()=>setLangs(v=>({...v,[who]:code}))}
              style={{paddingVertical:6,paddingHorizontal:8,borderRadius:8,backgroundColor:lang===code?color:'#fff'}}>
              <Text style={{fontSize:12,color:lang===code?'#fff':'#253248'}}>{label}</Text>
            </Pressable>)}
          </View>
        </View>
        <Text style={{fontSize:13,color:muted}}>{s.hint}</Text>
      </View>

      <View style={{gap:6}}>
        {waitingForMe
          ?<View style={{flexDirection:'row',alignItems:'center',gap:10}}><ActivityIndicator color={color}/><Text style={{fontSize:22,fontWeight:'700',color:muted}}>{s.otherSpeaking}</Text></View>
          :<Text style={{fontSize:26,fontWeight:'700',color:ink}}>{shown[who]||GREETING[lang]}</Text>}
        {!!shown[who]&&!waitingForMe&&<Pressable accessibilityRole="button" accessibilityLabel={s.replay} onPress={()=>replay(shown[who],lang)} hitSlop={8}>
          <Text style={{fontSize:17,color}}>🔊 {s.replay}</Text>
        </Pressable>}
        {!!heard[who]&&<Text style={{fontSize:14,color:muted}}>{s.heard}: {heard[who]}</Text>}
      </View>

      <View style={{gap:8}}>
        {!!statusText&&<View style={{flexDirection:'row',alignItems:'center',gap:8,alignSelf:'center',backgroundColor:'#fff',borderRadius:16,paddingVertical:6,paddingHorizontal:12}}>
          {status.stage==='listening'&&mine?<View style={{width:10,height:10,borderRadius:5,backgroundColor:'#E5484D'}}/>:<ActivityIndicator size="small" color={color}/>}
          <Text style={{fontSize:14,fontWeight:'600',color:ink}}>{statusText}</Text>
        </View>}
        <View style={{flexDirection:'row',alignItems:'center',gap:8}}>
          <TextInput style={{flex:1,fontSize:17,borderBottomWidth:2,borderColor:color,padding:8,color:ink}}
            placeholder={s.placeholder} value={typed[who]} editable={!busy}
            onChangeText={v=>setTyped(x=>({...x,[who]:v}))}
            onSubmitEditing={()=>{void send(who,typed[who]);setTyped(x=>({...x,[who]:''}))}}/>
          <Pressable accessibilityRole="button" accessibilityLabel={s.send} disabled={busy||!typed[who].trim()}
            onPress={()=>{void send(who,typed[who]);setTyped(x=>({...x,[who]:''}))}}
            style={{backgroundColor:color,borderRadius:12,paddingVertical:10,paddingHorizontal:14,opacity:busy||!typed[who].trim()?0.5:1}}>
            <Text style={{color:'#fff',fontWeight:'700'}}>{s.send}</Text>
          </Pressable>
        </View>
        {Platform.OS==='web'
          ?<Text style={{color:muted}}>{t('tr_type_hint')}</Text>
          :<NativeRecorder lang={voiceLang(lang)} onText={text=>void send(who,text)} onError={errorToast}
            onPhase={top?onVisitorPhase:onAgentPhase} label={s.hold} activeLabel={s.release}
            disabled={busy&&!(mine&&status.stage==='listening')} wide color={color}/>}
      </View>
    </View>;
  };

  return <View style={{flex:1,paddingTop:insets.top,paddingBottom:insets.bottom,backgroundColor:honey}}>
    {side('visitor')}
    <View style={{height:60,backgroundColor:honey,flexDirection:'row',justifyContent:'space-between',alignItems:'center',paddingHorizontal:12}}>
      <Pressable onPress={save} hitSlop={12} style={{minWidth:52,height:48,justifyContent:'center'}}>
        <Text style={{fontWeight:'700',fontSize:16}}>{t('tr_save')}</Text>
      </Pressable>
      <Text style={{fontSize:14,fontWeight:'700'}}>{l2('Talk to me','和我说')}</Text>
      <Pressable testID="translate-close" accessibilityRole="button" accessibilityLabel={t('close')} onPress={closeSheet} hitSlop={12}
        style={({pressed})=>({width:48,height:48,borderRadius:24,alignItems:'center',justifyContent:'center',backgroundColor:pressed?'rgba(41,34,27,0.22)':'rgba(41,34,27,0.12)'})}>
        <Text style={{fontSize:32,lineHeight:34,fontWeight:'700',color:'#29221B'}}>×</Text>
      </Pressable>
    </View>
    {side('agent')}
    <Text style={{backgroundColor:honey,textAlign:'center',padding:10,color:'#29221B',fontSize:12}}>{t('tr_privacy')}</Text>
    <Confetti run={!warming}/>
  </View>;
}
