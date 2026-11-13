import React from 'react';
import {View,TextInput,Pressable} from 'react-native';
import {Icon,SectionLabel} from './ui';
import {colors,text} from '../theme';
import {openSheet} from '../store';
export function MicInput({label,value,onChangeText,multiline=false,placeholder}:{label:string;value:string;onChangeText:(value:string)=>void;multiline?:boolean;placeholder?:string}){return <View style={{gap:5}}><SectionLabel>{label}</SectionLabel><View style={{flexDirection:'row',alignItems:'center',borderWidth:1,borderColor:colors.line,borderRadius:8,backgroundColor:'#fff'}}><TextInput value={value} onChangeText={onChangeText} multiline={multiline} placeholder={placeholder} placeholderTextColor={colors.steel} style={[text,{flex:1,padding:12,minHeight:multiline?75:42,textAlignVertical:'top'}]}/><Pressable accessibilityLabel={`Dictate ${label}`} onPress={()=>openSheet({kind:'dictate',value,onResult:onChangeText})} style={{padding:12}}><Icon name="mic" color={colors.mic}/></Pressable></View></View>}
export default MicInput;
