import React from 'react';
import {Pressable,Text,View} from 'react-native';
import {useToast,hideToast} from '../store';
import {colors,text} from '../theme';
import {useT} from '../i18n';

/** Rendered at the root and again inside the sheet Modal, so feedback is visible while a sheet is open (iOS draws Modals above everything). */
export function Toast({bottom=90}:{bottom?:number}){
  const toast=useToast();const {l2}=useT();
  if(!toast)return null;
  return <View pointerEvents="box-none" style={{position:'absolute',left:20,right:20,bottom,zIndex:1000,elevation:20}}>
    <View accessibilityLiveRegion="polite" style={{backgroundColor:colors.fg,borderRadius:10,padding:14,flexDirection:'row',alignItems:'center',justifyContent:'space-between',gap:12}}>
      <Text style={[text,{color:'white',flex:1}]}>{toast.text}</Text>
      {toast.undo&&<Pressable accessibilityRole="button" hitSlop={10} onPress={()=>{const undo=toast.undo;hideToast();undo?.();}}><Text style={[text,{color:colors.brand,fontWeight:'700'}]}>{l2('Undo','撤销')}</Text></Pressable>}
    </View>
  </View>;
}
