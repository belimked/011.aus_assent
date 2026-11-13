import React from 'react';
import {ScrollView,View,Text,Pressable} from 'react-native';
import {useRouter} from 'expo-router';
import {useSafeAreaInsets} from 'react-native-safe-area-context';
import {colors,text} from '../theme';
import {useStore} from '../store';
import {Icon,Chip} from './ui';
import {TODAY} from '../data/seed';
export function AppHeader({title,head,back=false}:{title?:string;head?:React.ReactNode;back?:boolean}){const router=useRouter();const pending=useStore(s=>s.syncq.length);return <View style={{height:64,backgroundColor:'#fff',borderBottomWidth:1,borderColor:colors.line,flexDirection:'row',alignItems:'center',paddingHorizontal:16,gap:12}}>{back&&<Pressable onPress={()=>router.back()} style={{width:34,height:40,justifyContent:'center'}}><View style={{transform:[{rotate:'180deg'}]}}><Icon name="chev" size={22} color={colors.fg}/></View></Pressable>}{head??<Text numberOfLines={1} style={[text,{fontSize:17,fontWeight:'700',flex:1}]}>{title}</Text>}{pending>0&&<Chip kind="amber">{pending} waiting to sync</Chip>}</View>}
export function Screen({title,head,back=false,children,dock}:{title?:string;head?:React.ReactNode;back?:boolean;children:React.ReactNode;dock?:React.ReactNode}){const safe=useSafeAreaInsets();const name=useStore(s=>s.auth?.name||'Owen Chen');return <View style={{flex:1,backgroundColor:colors.bg,paddingTop:safe.top}}><AppHeader title={title} head={head} back={back}/><ScrollView contentContainerStyle={{padding:16,gap:10,paddingBottom:32}} keyboardShouldPersistTaps="handled">{children}</ScrollView>{dock}<View pointerEvents="none" style={{position:'absolute',left:12,right:12,top:110,bottom:55,justifyContent:'space-around',opacity:.07,transform:[{rotate:'-27deg'}]}}>{Array.from({length:7},(_,i)=><Text key={i} numberOfLines={1} style={{color:colors.fg,fontSize:13}}>{name} · {TODAY} {new Date().toTimeString().slice(0,5)}     {name} · {TODAY}</Text>)}</View></View>}
