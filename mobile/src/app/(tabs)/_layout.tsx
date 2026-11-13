import React from 'react';
import {Tabs} from 'expo-router';
import {Pressable,Text,View} from 'react-native';
import {useSafeAreaInsets} from 'react-native-safe-area-context';
import {colors,text} from '../../theme';
import {Icon} from '../../components/ui';
import type {IconName} from '../../components/ui/Icon';
function Bar({state,descriptors,navigation}:any){const safe=useSafeAreaInsets();const tabs:{name:string;label:string;icon:IconName}[]=[{name:'index',label:'Home',icon:'house'},{name:'dk',label:'DK',icon:'door'},{name:'oh',label:'OFI',icon:'oh'},{name:'enq',label:'Enquiry',icon:'enq'}];return <View style={{flexDirection:'row',backgroundColor:'#fff',borderTopWidth:1,borderColor:colors.line,paddingBottom:safe.bottom,paddingTop:7,minHeight:62+safe.bottom}}>{state.routes.map((route:any,index:number)=>{const item=tabs.find(t=>t.name===route.name);if(!item)return null;const active=state.index===index;return <Pressable key={route.key} accessibilityRole="tab" accessibilityState={{selected:active}} onPress={()=>navigation.navigate(route.name)} style={{flex:1,alignItems:'center',gap:3}}><View style={{backgroundColor:active?colors.fg:'transparent',width:38,height:34,borderRadius:9,alignItems:'center',justifyContent:'center'}}><Icon name={item.icon} size={21} color={active?'#fff':colors.muted}/></View><Text style={[text,{fontSize:11,color:active?colors.fg:colors.muted,fontWeight:active?'700':'500'}]}>{item.label}</Text></Pressable>})}</View>}
export default function TabLayout(){return <Tabs tabBar={props=><Bar {...props}/>} screenOptions={{headerShown:false}}><Tabs.Screen name="index"/><Tabs.Screen name="dk"/><Tabs.Screen name="oh"/><Tabs.Screen name="enq"/></Tabs>}
