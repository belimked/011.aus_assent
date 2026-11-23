import React,{useMemo,useState,useRef} from 'react';
import {Platform,Text,View,Pressable,Switch} from 'react-native';
import {useRouter} from 'expo-router';
import {Screen} from '../../components/Screen';
import {Button,Card,Icon,Logo} from '../../components/ui';
import {MapView} from '../../components/map/MapView';
import {useLivePosition,retryLocation,openLocationSettings,canOpenSettings,distanceM,fmtDistance} from '../../components/map/useLivePosition';
import {ListenOverlay} from '../../components/ListenOverlay';
import {useHoldRecorder} from '../../components/useHoldRecorder';
import {voiceErrorText} from '../../lib/voice';
import {getVoiceLang} from '../../lib/voicePref';
import {colors,text} from '../../theme';
import {useStore,openSheet,showToast} from '../../store';
import {propAddr,ownersOf,cname,yearsSince} from '../../domain';
import {TODAY} from '../../data/seed';
import {useT} from '../../i18n';
export function DK(){
  const s=useStore(s=>s);const router=useRouter();const {t,l2,lang:appLang}=useT();const rec=useHoldRecorder();const holding=useRef(false);const loc=useLivePosition();const live=loc.position;
  // Nearest property to the phone's real GPS fix; nothing until we have one (no demo fallback).
  const nearest=useMemo(()=>{if(!live)return null;let id='',d=Infinity;for(const p of s.properties){if(p.ll?.length!==2)continue;const m=distanceM(live,{lat:p.ll[0],lng:p.ll[1]});if(m<d){d=m;id=p.id}}return id?{id,d}:null},[live,s.properties]);
  const [gps,setGps]=useState(true);const [at,setAt]=useState<string|null>(null);
  const toggleGps=(value:boolean)=>{setGps(value);if(value){if(nearest)setAt(nearest.id);if(loc.status!=='live')retryLocation()}else if(!at&&nearest)setAt(nearest.id);
    const np=nearest&&s.properties.find(x=>x.id===nearest.id);showToast(value?(np?`${l2('GPS LIVE','GPS 实时')} · ${propAddr(np)}`:l2('GPS on · finding your location…','GPS 已开 · 正在定位…')):l2('GPS OFF','GPS 已关闭'))};
  const current=gps?nearest?.id:at;
  const p=current?s.properties.find(x=>x.id===current):undefined;
  const owner=p?ownersOf(s,p).map(cname).join(', ')||p.pfOwner||l2('No contact yet','尚无联系人'):'';
  const far=gps&&!!p&&!!nearest&&nearest.d>150;
  const openChange=()=>openSheet({kind:'change',onPick:(id:string)=>{setAt(id);setGps(false)}});
  const first=s.auth?.name.split(' ')[0]||'Owen';
  // Prototype: tap opens "Say it or type it"; hold >=260 ms listens in place, release transcribes and opens the same sheet with the words.
  const openCmd=(extra:{text?:string;err?:string}={})=>{if(!p){showToast(l2('Pick the address first','请先选择地址'));openChange();return}openSheet({kind:'cmd',ctx:'dk',propId:p.id,...extra})};
  const release=async()=>{const lang=getVoiceLang(appLang);try{const words=await rec.finish(lang);if(words===null)return;openCmd(words?{text:words}:{err:t('nothing_heard')})}catch(e){openCmd({err:voiceErrorText(e,l2)})}};
  const startHold=async()=>{if(Platform.OS==='web'||!p){openCmd();return}const r=await rec.start();
    if(r==='ok'){if(!holding.current)void release();return}
    if(r==='asked'){showToast(l2('Microphone ready. Hold the button again to speak.','麦克风已开启，请再次按住说话。'));return}
    openCmd({err:r==='offline'?l2('Sign in with your email to use voice. You can type instead.','请用邮箱登录后使用语音，也可以直接输入。'):r==='denied'?l2('Microphone access is off. Enable it in Settings, or type instead.','麦克风权限已关闭，请在设置中开启，或直接输入。'):l2('Could not start recording. Try again, or type instead.','无法开始录音，请重试或直接输入。')})};
  return <><Screen head={<View style={{flexDirection:'row',alignItems:'center',flex:1,gap:10}}><Logo testID="open-me" onPress={()=>router.push('/me')}/><Text style={[text,{fontWeight:'700',fontSize:17,flex:1}]}><Text style={{fontSize:11,color:colors.muted}}>{TODAY} · DK{'\n'}</Text>{l2('Hi','你好')} {first}</Text><Text style={[text,{fontSize:11,fontWeight:'700'}]}>{gps?(loc.status==='live'?l2('GPS LIVE','GPS 实时'):l2('GPS …','GPS …')):l2('GPS OFF','GPS 已关闭')}</Text><Switch value={gps} onValueChange={toggleGps} trackColor={{true:colors.green}}/></View>}>
    <Card style={{borderColor:gps&&loc.status==='live'?colors.green:colors.line,padding:16}}><Text style={[text,{fontSize:11,color:colors.muted,fontWeight:'700'}]}>{gps?(far?l2('NEAREST PROPERTY · LIVE GPS','最近的房产 · 实时 GPS'):l2("YOU'RE AT · LIVE GPS",'你的位置 · 实时 GPS')):l2("YOU'RE SEARCHING FOR",'正在搜索')}</Text>{gps&&!p&&<View style={{gap:4}}><Text style={[text,{fontSize:17,fontWeight:'700'}]}>{loc.status==='denied'?l2('Location permission needed','需要定位权限'):loc.status==='blocked'?l2('Location access is off','定位权限已关闭'):loc.status==='off'?l2('Location Services are off','手机定位服务已关闭'):loc.status==='error'?l2('Could not get your location','无法获取你的位置'):live?l2('No property nearby','附近没有房产'):l2('Finding your location…','正在定位…')}</Text><Text style={[text,{color:colors.muted,fontSize:12}]}>{loc.status==='denied'?l2('Allow location so we can show the property you are standing at.','允许定位后，我们会显示你所在的房产。'):loc.status==='blocked'||loc.status==='off'?(canOpenSettings?l2('Turn on location for AxleoCollect in Settings, or pick the address yourself.','请在设置中为 AxleoCollect 开启定位，或手动选择地址。'):l2('Allow location for this site in your browser, or pick the address yourself.','请在浏览器中允许本站定位，或手动选择地址。')):loc.status==='error'?l2('Move to open sky and try again, or pick the address yourself.','请到开阔处重试，或手动选择地址。'):l2('Or pick the address yourself.','也可以手动选择地址。')}</Text>{(loc.status==='denied'||loc.status==='error')&&<Button size="sm" variant="primary" onPress={retryLocation}>{loc.status==='denied'?l2('Allow location','允许定位'):l2('Try again','重试')}</Button>}{(loc.status==='blocked'||loc.status==='off')&&canOpenSettings&&<Button size="sm" variant="primary" onPress={openLocationSettings}>{l2('Open Settings','打开设置')}</Button>}</View>}{!gps&&!p&&<Text style={[text,{fontSize:17,fontWeight:'700'}]}>{l2('Pick an address','请选择地址')}</Text>}{p&&<Pressable testID="dk-address" onPress={()=>router.push(`/prop/${p.id}`)}><Text style={[text,{fontSize:19,fontWeight:'700'}]}>{propAddr(p)} ›</Text><Text style={[text,{fontSize:12}]}>{owner}{p.lastSale?` · ${l2('owned','持有')} ${yearsSince(p.lastSale)} ${l2('years','年')}`:''}</Text>{far&&<Text style={[text,{color:colors.accent,fontSize:12,fontWeight:'700'}]}>{l2(`You are ${fmtDistance(nearest!.d)} away · tap Change if this is not the right address`,`距你 ${fmtDistance(nearest!.d)} · 如地址不对请点“更改”`)}</Text>}<Text style={[text,{color:colors.muted,fontSize:11}]}>{l2('Tap address for property and ownership details','点击地址查看房产和业主详情')}</Text></Pressable>}{!gps&&<Button size="sm" variant="ghost" onPress={()=>toggleGps(true)}>{l2('Back to GPS location','返回 GPS 位置')}</Button>}<Button size="sm" onPress={openChange}>{l2('Change','更改')}</Button></Card>
    <View style={{alignItems:'center',paddingVertical:28,gap:10}}><Pressable testID="dk-capture" accessibilityRole="button" accessibilityLabel={l2('Hold to speak','按住说话')} accessibilityHint={t('tap_hint')} disabled={rec.phase==='transcribing'} onPressIn={()=>{holding.current=true}} onPress={()=>openCmd()} onLongPress={()=>void startHold()} delayLongPress={260} onPressOut={()=>{holding.current=false;void release()}} style={{width:104,height:104,borderRadius:52,backgroundColor:colors.mic,alignItems:'center',justifyContent:'center',transform:[{scale:rec.phase==='recording'?1.08:1}],borderWidth:rec.phase==='recording'?6:0,borderColor:'#FFD3BF'}}><Icon name="mic" size={40} color="#fff"/></Pressable><Text style={[text,{fontWeight:'700',fontSize:17}]}>{t('speak')}</Text><Text style={[text,{color:colors.muted,fontSize:12,textAlign:'center'}]}>{l2('Say anything: owner, mobile, tags, notes.','说出业主、手机号码、标签、备注等。')}{'\n'}{l2('We’ll file it for you to check.','我们会整理好供你确认。')}</Text></View>
    {p&&<Card><Pressable onPress={()=>router.push(`/prop/${p.id}`)}><Text style={[text,{fontWeight:'700'}]}>{l2('Tap the property above','点击上方房产')} ›</Text><Text style={[text,{color:colors.muted,fontSize:12}]}>{l2('See everything about it, edit the record or add a note','查看全部信息、编辑记录或添加备注')}</Text></Pressable></Card>}
    {s.captures.some(c=>c.status!=='approved')&&<Button onPress={()=>router.push('/captures')}>{s.captures.filter(c=>c.status!=='approved').length} {l2('captures waiting for your approval · Review','条记录等待你确认 · 查看')} ›</Button>}
    <Pressable testID="dk-fullmap" onPress={()=>router.push('/fullmap')}><View pointerEvents="none"><MapView properties={s.properties} selected={far?undefined:p?.id} follow={gps} onSelect={()=>{}} height={190}/></View><Text style={[text,{fontSize:12,textAlign:'right',color:colors.link}]}>{l2('Map · Full screen','地图 · 全屏')} ↗</Text></Pressable>
  </Screen>{rec.phase!=='idle'&&<ListenOverlay phase={rec.phase} lang={getVoiceLang(appLang)} durationMs={rec.durationMs} metering={rec.metering}/>}</>;
}
