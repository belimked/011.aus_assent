import React from 'react';
import {View,TextInput} from 'react-native';
import {SectionLabel} from './ui';
import {NoteMic} from './NoteMic';
import {colors,text} from '../theme';
/** Text box with the prototype's inline dictation mic (hold to dictate, release to finish; words are appended). */
export function MicInput({label,value,onChangeText,multiline=false,placeholder}:{label:string;value:string;onChangeText:(value:string)=>void;multiline?:boolean;placeholder?:string}){return <View style={{gap:5}}><SectionLabel>{label}</SectionLabel><View style={{flexDirection:'row',alignItems:'flex-end',borderWidth:1,borderColor:colors.line,borderRadius:8,backgroundColor:'#fff'}}><TextInput value={value} onChangeText={onChangeText} multiline={multiline} placeholder={placeholder} placeholderTextColor={colors.steel} style={[text,{flex:1,padding:12,minHeight:multiline?75:42,textAlignVertical:'top'}]}/><View style={{padding:4}}><NoteMic size={34} value={value} onChangeText={onChangeText}/></View></View></View>}
export default MicInput;
