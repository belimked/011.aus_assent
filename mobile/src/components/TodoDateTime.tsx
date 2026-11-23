import React,{useState} from 'react';
import {Platform,Text,View} from 'react-native';
import DateTimePicker from '@react-native-community/datetimepicker';
import {Button,Field} from './ui';
import {useT} from '../i18n';
import {text} from '../theme';
import {validTodoDate,validTodoTime} from '../domain/todoDate';

/** Native calendar/clock on iOS and Android. Web keeps editable fields for the preview. */
export function TodoDateTime({mode,value,onChange}:{mode:'date'|'time';value:string;onChange:(value:string)=>void}){
  const {l2}=useT();
  const [visible,setVisible]=useState(false);
  const fallback=mode==='date'?'2026-01-01':'16:00';
  const safe=mode==='date'?(validTodoDate(value)?value:fallback):(validTodoTime(value)?value:fallback);
  const parsed=mode==='date'?new Date(`${safe}T12:00:00`):new Date(`2026-01-01T${safe}:00`);
  const format=(date:Date)=>mode==='date'?`${date.getFullYear()}-${String(date.getMonth()+1).padStart(2,'0')}-${String(date.getDate()).padStart(2,'0')}`:`${String(date.getHours()).padStart(2,'0')}:${String(date.getMinutes()).padStart(2,'0')}`;
  if(Platform.OS==='web')return <Field label={mode==='date'?l2('Other date (YYYY-MM-DD)','其他日期'):l2('Other time (HH:MM)','其他时间')} value={value} onChangeText={onChange}/>;
  return <View style={{gap:6}}><Button onPress={()=>setVisible(v=>!v)}>{mode==='date'?l2('Other date','其他日期'):l2('Other time','其他时间')} · {value}</Button>
    {visible&&<View><DateTimePicker value={parsed} mode={mode} display={Platform.OS==='ios'?'inline':'default'} is24Hour onChange={(event,selected)=>{if(Platform.OS!=='ios')setVisible(false);if(event.type==='set'&&selected)onChange(format(selected));}}/>
      {Platform.OS==='ios'&&<Button size="sm" onPress={()=>setVisible(false)}>{l2('Done','完成')}</Button>}</View>}
    {!value&&<Text style={text}>{l2('Choose a date and time','请选择日期和时间')}</Text>}
  </View>;
}
