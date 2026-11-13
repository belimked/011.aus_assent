import React from 'react';
import {Modal,KeyboardAvoidingView,Platform,Pressable,ScrollView,Text,View} from 'react-native';
import {colors,text} from '../theme';
import {useSheet,closeSheet,dismissSheet} from '../store';
import {Icon} from '../components/ui';
import {registry} from './registry';
export function SheetHost(){const sheet=useSheet();const Component=sheet?registry[sheet.kind]:null;return <Modal visible={!!sheet} transparent animationType="slide" onRequestClose={closeSheet}><KeyboardAvoidingView behavior={Platform.OS==='ios'?'padding':undefined} style={{flex:1,justifyContent:'flex-end'}}><Pressable onPress={dismissSheet} style={{position:'absolute',top:0,left:0,right:0,bottom:0,backgroundColor:'#13233F77'}}/><View style={{maxHeight:'92%',backgroundColor:colors.bg,borderTopLeftRadius:18,borderTopRightRadius:18,paddingTop:8}}><View style={{flexDirection:'row',justifyContent:'center',alignItems:'center',height:32}}><View style={{width:40,height:4,borderRadius:3,backgroundColor:colors.steel}}/><Pressable onPress={closeSheet} style={{position:'absolute',right:15,top:1,padding:6}}><Icon name="x" size={22}/></Pressable></View><ScrollView keyboardShouldPersistTaps="handled" contentContainerStyle={{padding:16,paddingBottom:36}}>{Component&&sheet?<Component params={sheet}/>:<Text style={text}>Coming in next build</Text>}</ScrollView></View></KeyboardAvoidingView></Modal>}
