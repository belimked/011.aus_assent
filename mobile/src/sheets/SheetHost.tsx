import React,{useEffect,useRef,useState} from 'react';
import {Keyboard,Modal,KeyboardAvoidingView,Platform,Pressable,ScrollView,Text,View} from 'react-native';
import {colors,text} from '../theme';
import {useSheet,closeSheet,dismissSheetBackdrop} from '../store';
import {Icon,SheetFocusContext} from '../components/ui';
import {registry} from './registry';
import {Toast} from '../components/Toast';
import {TranslateSheet} from './ofi/Translate';
import {sheetStack} from './stack';

function SheetContent({entry,active}:{entry:{kind:string;[key:string]:any};active:boolean}){
  const scroll=useRef<React.ComponentRef<typeof ScrollView>>(null);
  const Component=registry[entry.kind];
  const focus=React.useCallback((target:number)=>{
    setTimeout(()=>scroll.current?.scrollResponderScrollNativeHandleToKeyboard(target,110,true),250);
  },[]);
  return <View style={active?{flexShrink:1}:{display:'none'}}>
    <SheetFocusContext.Provider value={focus}>
      <ScrollView ref={scroll} keyboardShouldPersistTaps="handled" keyboardDismissMode="on-drag" contentContainerStyle={{padding:16,paddingBottom:36}}>
        {Component?<Component params={entry}/>:<Text style={text}>Coming in next build</Text>}
      </ScrollView>
    </SheetFocusContext.Provider>
  </View>;
}

export function SheetHost(){
  const sheet=useSheet();
  const panel=useRef<React.ElementRef<typeof View>>(null);
  const [keyboardOpen,setKeyboardOpen]=useState(false);
  useEffect(()=>{const show=Keyboard.addListener('keyboardDidShow',()=>setKeyboardOpen(true));const hide=Keyboard.addListener('keyboardDidHide',()=>setKeyboardOpen(false));return()=>{show.remove();hide.remove()}},[]);
  const onBackdrop=React.useCallback(()=>{if(keyboardOpen)Keyboard.dismiss();else dismissSheetBackdrop()},[keyboardOpen]);
  useEffect(()=>{if(Platform.OS!=='web'||!sheet||sheet.kind==='translate')return;const outside=(ev:MouseEvent)=>{const rect=(panel.current as unknown as HTMLElement)?.getBoundingClientRect?.();if(ev.clientY<Math.max(rect?.top||0,window.innerHeight*.14)){ev.preventDefault();ev.stopPropagation();onBackdrop()}};document.addEventListener('click',outside,true);return()=>document.removeEventListener('click',outside,true)},[sheet,onBackdrop]);
  // Keep the previous form mounted while a nested picker is open: its onSave callback
  // must still update that form's local state when Save tags returns to it.
  const entries=sheetStack(sheet);
  return <Modal visible={!!sheet} transparent={sheet?.kind!=='translate'} animationType="slide" onRequestClose={closeSheet}>
    {sheet?.kind==='translate'?<TranslateSheet/>:<KeyboardAvoidingView behavior={Platform.OS==='ios'?'padding':undefined} style={{flex:1,justifyContent:'flex-end'}}>
      <Pressable onPress={onBackdrop} style={{flex:1,backgroundColor:'#13233F77'}}/>
      <View ref={panel} style={{maxHeight:'86%',backgroundColor:colors.bg,borderTopLeftRadius:18,borderTopRightRadius:18,paddingTop:8}}>
        <View style={{flexDirection:'row',justifyContent:'center',alignItems:'center',height:32}}><View style={{width:40,height:4,borderRadius:3,backgroundColor:colors.steel}}/><Pressable onPress={closeSheet} style={{position:'absolute',right:15,top:1,padding:6}}><Icon name="x" size={22}/></Pressable></View>
        {entries.map((entry,index)=><SheetContent key={index} entry={entry} active={index===entries.length-1}/>)}
      </View>
    </KeyboardAvoidingView>}
    <Toast bottom={36}/>
  </Modal>;
}
