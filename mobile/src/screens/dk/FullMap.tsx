import React,{useMemo,useState} from 'react';
import {Pressable,ScrollView,Text,TextInput,View,type ViewStyle} from 'react-native';
import {useRouter,useLocalSearchParams} from 'expo-router';
import {useSafeAreaInsets} from 'react-native-safe-area-context';
import {AppHeader,Watermark} from '../../components/Screen';
import {MapView} from '../../components/map/MapView';
import {MapLegend} from '../../components/map/MapLegend';
import {useLivePosition} from '../../components/map/useLivePosition';
import {Button,Card,Chip,ListItem,Seg} from '../../components/ui';
import {useStoreKeys,openSheet,update,showToast} from '../../store';
import {ensureParcelRecord,findParcelRecord} from '../../domain/map';
import {colors,text} from '../../theme';
import {cname,ownersOf,propAddr,propColor,yearsSince} from '../../domain';
import {matchesAddress,houseNumberOrder} from '../../domain/dk';
import {properties as parcels,DEMO_POSITION} from '../../data/properties';
import {useT} from '../../i18n';
// Prototype v14 full-screen map: search + Map/Satellite/List in the header, the map fills the rest,
// and the preview card, legend and buttons float over it.
const SHADOW:ViewStyle={shadowColor:'#000',shadowOpacity:.18,shadowRadius:8,shadowOffset:{width:0,height:3},elevation:5};
const PREVIEW:ViewStyle={position:'absolute',top:12,left:12,right:12,zIndex:10,paddingRight:40,...SHADOW};
const CLOSE:ViewStyle={position:'absolute',top:6,right:6,width:32,height:32,alignItems:'center',justifyContent:'center'};
const FLOAT:ViewStyle={position:'absolute',right:12,zIndex:10,backgroundColor:'#fff',borderRadius:10,height:40,paddingHorizontal:12,flexDirection:'row',alignItems:'center',...SHADOW};
const NEAR_DEG=.015; // about 1.5 km: no Pricefinder parcel this close means the phone is outside the data area
// Collapsed by default; remembered while the app runs.
let legendOpen=false;
export function FullMap(){const s=useStoreKeys('properties','contacts');const router=useRouter();const safe=useSafeAreaInsets();const params=useLocalSearchParams<{selected?:string}>();const live=useLivePosition().position;const [locate,setLocate]=useState(0);const [legend,setLegend]=useState(legendOpen);const [legendHeight,setLegendHeight]=useState(36);const [mapHeight,setMapHeight]=useState(0);const [mode,setMode]=useState('map');const [query,setQuery]=useState('');const [street,setStreet]=useState('');const [streetOpen,setStreetOpen]=useState(false);const {l2}=useT();const streets=useMemo(()=>[...new Set(s.properties.map(p=>p.street+', '+p.suburb))],[s.properties]);const [selected,setSelected]=useState(params.selected||'');const background=selected.startsWith('q-')?parcels.find(x=>x.id===selected.slice(2)):undefined;const p=s.properties.find(p=>p.id===selected)||(background?findParcelRecord(s,background):undefined);const openBackground=(note:boolean)=>{if(!background)return;let id='';update(d=>{id=ensureParcelRecord(d,background).id});openSheet(note?{kind:'note',propId:id}:{kind:'quickadd',propId:id})};const filtered=useMemo(()=>s.properties.filter(p=>matchesAddress(propAddr(p),query)||matchesAddress(`${p.street}, ${p.suburb}`,query)),[s.properties,query]);const list=useMemo(()=>filtered.filter(p=>!street||p.street+', '+p.suburb===street).slice().sort(houseNumberOrder),[filtered,street]);const search=()=>{const found=filtered[0];if(found){setSelected(found.id);setStreet(found.street+', '+found.suburb)}else showToast(l2('No address found','没有找到地址'))};const owner=p&&ownersOf(s,p)[0];
  const toggleLegend=()=>{legendOpen=!legend;setLegend(legendOpen)};
  const liveKey=live?`${live.lat.toFixed(3)},${live.lng.toFixed(3)}`:'';
  // Real GPS far from the imported suburbs shows an empty street map; offer a jump to the data instead.
  const outside=useMemo(()=>!!live&&!parcels.some(q=>Math.abs(q.lat-live.lat)<NEAR_DEG&&Math.abs(q.lng-live.lng)<NEAR_DEG),
  // eslint-disable-next-line react-hooks/exhaustive-deps
  [liveKey]);
  const goToData=()=>{const own=s.properties.filter(x=>x.ll?.length===2).sort((a,b)=>Math.hypot(a.ll![0]-DEMO_POSITION.lat,a.ll![1]-DEMO_POSITION.lng)-Math.hypot(b.ll![0]-DEMO_POSITION.lat,b.ll![1]-DEMO_POSITION.lng))[0];
    if(own){setQuery('');setSelected(own.id);return}
    const q=parcels.slice().sort((a,b)=>Math.hypot(a.lat-DEMO_POSITION.lat,a.lng-DEMO_POSITION.lng)-Math.hypot(b.lat-DEMO_POSITION.lat,b.lng-DEMO_POSITION.lng))[0];if(q){setQuery('');setSelected('q-'+q.id)}};
  const close=<Pressable testID="map-preview-close" accessibilityLabel={l2('Close','关闭')} hitSlop={12} onPress={()=>setSelected('')} style={CLOSE}><Text style={[text,{fontSize:18,color:colors.muted}]}>✕</Text></Pressable>;
  const head=<View style={{flex:1,flexDirection:'row',alignItems:'center',gap:8}}>
    <TextInput testID="map-search" accessibilityLabel={l2('Search address or street','搜索地址或街道')} placeholder={l2('Search address','搜索地址')} placeholderTextColor={colors.steel} value={query} onChangeText={setQuery} onSubmitEditing={search} returnKeyType="search" clearButtonMode="always" style={[text,{flex:1,minWidth:0,height:40,backgroundColor:'#fff',borderWidth:1,borderColor:colors.line,borderRadius:8,paddingHorizontal:10}]}/>
    <View style={{width:186}}><Seg items={[["map",l2('Map','地图')],["sat",l2('Satellite','卫星')],["list",l2('List','列表')]]} value={mode} onChange={setMode}/></View>
  </View>;
  const sideBottom=legendHeight+24+safe.bottom;
  return <View style={{flex:1,backgroundColor:colors.bg,paddingTop:safe.top,overflow:'hidden'}}>
  <AppHeader back head={head}/>
  {mode==='list'?<ScrollView contentContainerStyle={{padding:16,gap:10,paddingBottom:32+safe.bottom}} keyboardShouldPersistTaps="handled"><Card><Text style={text}>{l2('Street','街道')}</Text><Button onPress={()=>setStreetOpen(v=>!v)}>{street||l2('All streets','所有街道')}  ▾</Button>{streetOpen&&<Card><ListItem title={l2('All streets','所有街道')} onPress={()=>{setStreet('');setStreetOpen(false)}}/>{streets.map(key=><ListItem key={key} title={key} onPress={()=>{setStreet(key);setStreetOpen(false)}}/>)}</Card>}{list.map(x=><ListItem key={x.id} title={propAddr(x)} subtitle={`${ownersOf(s,x).map(cname).join(', ')||x.pfOwner||l2('No contact yet','尚无联系人')} · ${x.lastSale?yearsSince(x.lastSale)+' '+l2('years','年'):'—'}`} right={x.source==='rex_import'?<Chip kind="steel">{l2('Rex sandbox','Rex 沙箱')}</Chip>:<Chip kind={propColor(x)}>●</Chip>} onPress={()=>{setSelected(x.id);router.push(`/prop/${x.id}`)}}/>)}</Card><Button block onPress={()=>router.back()}>{l2('Exit full screen','退出全屏')}</Button></ScrollView>
  :<View style={{flex:1}} onLayout={e=>setMapHeight(Math.round(e.nativeEvent.layout.height))}>
  {mapHeight>0&&<MapView properties={filtered} selected={selected||undefined} onSelect={setSelected} satellite={mode==='sat'} height={mapHeight} square locate={locate}/>}
  {outside&&!selected&&!query&&<Pressable testID="map-outside" onPress={goToData} style={[PREVIEW,{backgroundColor:'#fff',borderRadius:12,padding:12,paddingRight:12,flexDirection:'row',alignItems:'center',gap:10}]}><Text style={[text,{flex:1,fontSize:13}]}>{l2('No patch data near you. Imported data covers Sunnybank and nearby suburbs.','你附近没有片区数据，目前导入的是 Sunnybank 一带。')}</Text><Text style={[text,{fontWeight:'700',color:colors.link,fontSize:13}]}>{l2('Go there ›','前往 ›')}</Text></Pressable>}
  {!!query&&!selected&&<View style={[PREVIEW,{paddingRight:0,flexDirection:'row'}]}><Chip kind="blue" onPress={()=>setQuery('')}>{l2(`Showing “${query}” · ${filtered.length} · Clear ✕`,`仅显示“${query}”· ${filtered.length} 个 · 清除 ✕`)}</Chip></View>}
  <Pressable testID="map-exit" accessibilityRole="button" onPress={()=>router.back()} style={[FLOAT,{bottom:sideBottom+48}]}><Text style={[text,{fontWeight:'700',fontSize:13}]}>↙ {l2('Exit full screen','退出全屏')}</Text></Pressable>
  <Pressable testID="map-locate" accessibilityRole="button" disabled={!live} onPress={()=>{setSelected('');setLocate(n=>n+1)}} style={[FLOAT,{bottom:sideBottom,opacity:live?1:.5}]}><Text style={[text,{fontWeight:'700',fontSize:13,color:colors.link}]}>◎ {live?l2('My location','我的位置'):l2('Locating…','正在定位…')}</Text></Pressable>
  <MapLegend open={legend} onToggle={toggleLegend} onHeight={setLegendHeight} bottom={12+safe.bottom}/>
  {background&&!p&&<Card style={PREVIEW}>{close}<Text style={[text,{fontWeight:'700',fontSize:16}]}>{background.number} {background.street}, {background.suburb}</Text><Text style={text}>{l2('Pricefinder owner','Pricefinder 业主')}: — · {l2('Years held','持有年限')}: —</Text><Text style={text}>{l2('Last sale','上次成交')}: —</Text><View style={{flexDirection:'row',gap:8,flexWrap:'wrap'}}><Button onPress={()=>openBackground(true)}>+ {l2('Note','备注')}</Button><Button variant="primary" onPress={()=>{openBackground(false)}}>{l2('New contact','新建联系人')} ›</Button></View></Card>}{p&&<Card testID="map-preview" style={PREVIEW}>{close}<Text style={[text,{fontWeight:'700',fontSize:16}]}>{propAddr(p)}</Text>{p.flags?.dnc&&<Text style={[text,{backgroundColor:'#D7263D',color:'#fff',fontWeight:'800',padding:8,borderRadius:6}]}>{l2('⊘ DO NOT CONTACT','⊘ 请勿联系')}</Text>}{p.source==='rex_import'&&<Chip kind="steel">{l2('Rex sandbox','Rex 沙箱')}</Chip>}<Text style={text}>{l2('Pricefinder owner','Pricefinder 业主')}: {p.market?.owners||p.pfOwner||'—'} · {l2('Years held','持有年限')}: {p.market?.lastSaleDate?yearsSince(p.market.lastSaleDate):p.lastSale?yearsSince(p.lastSale):'—'}</Text><Text style={text}>{l2('Last sale','上次成交')}: {p.market?.lastSaleDate||p.lastSale||'—'}</Text><Text style={text}>{ownersOf(s,p).map(cname).join(', ')||l2('No linked contact','暂无关联联系人')}</Text><Text style={[text,{fontSize:12,color:colors.muted}]}>{owner?.tags?.find(tag=>['Hot','Warm','Cold'].includes(tag))||'—'}{owner?.notes?.[0]?.d?` · ${l2('last contact','最近联系')} ${owner.notes[0].d}`:''}</Text><View style={{flexDirection:'row',flexWrap:'wrap',gap:8}}><Button variant="primary" onPress={()=>router.push(`/prop/${p.id}`)}>{l2('Open record','打开记录')} ›</Button><Button onPress={()=>openSheet({kind:'note',propId:p.id})}>+ {l2('Note','备注')}</Button>{!owner&&<Button onPress={()=>openSheet({kind:'quickadd',propId:p.id})}>{l2('New contact','新建联系人')}</Button>}</View></Card>}
  </View>}
  <Watermark/>
</View>}
