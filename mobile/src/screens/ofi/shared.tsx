import React from 'react';
import {Text,View,Pressable} from 'react-native';
import {Chip,SectionLabel} from '../../components/ui';
import {REQS,type Requests} from '../../domain/ofi';
import {useT} from '../../i18n';
import {colors,text} from '../../theme';
export function RequestPicker({value,onChange}:{value:Requests;onChange:(v:Requests)=>void}){const {t}=useT();return <View style={{gap:4}}><SectionLabel>{t('requests')}</SectionLabel>{REQS.map(([key,label,who])=><Pressable key={key} onPress={()=>onChange({...value,[key]:!value[key]})} style={{minHeight:44,padding:8,borderRadius:8,borderWidth:1,borderColor:colors.line,flexDirection:'row',alignItems:'center',gap:9,backgroundColor:'#fff'}}><Text style={text}>{value[key]?'☑':'□'}</Text><View><Text style={[text,{fontSize:13}]}>{t(label)}</Text><Text style={[text,{fontSize:11,color:colors.muted}]}>{who}</Text></View></Pressable>)}</View>}
export function RequestChips({value}:{value:Requests}){const {t}=useT();return <View style={{flexDirection:'row',flexWrap:'wrap',gap:5}}>{REQS.filter(([k])=>value?.[k]).map(([k,label])=><Chip key={k}>{t(label).split(' (')[0]}</Chip>)}</View>}
export function Label({children}:{children:React.ReactNode}){return <Text style={[text,{fontSize:12,color:colors.muted}]}>{children}</Text>}
