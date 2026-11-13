import React,{useEffect} from 'react';
import {Stack,useRouter,useSegments} from 'expo-router';
import {StatusBar} from 'expo-status-bar';
import {SafeAreaProvider} from 'react-native-safe-area-context';
import {useFonts} from 'expo-font';
import {hydrate,useReady,useStore,useToast} from '../store';
import {hydrateLanguage} from '../i18n';
import {SheetHost} from '../sheets/SheetHost';
import {Pressable,Text,View} from 'react-native';
import {colors,text} from '../theme';
function Gate(){const router=useRouter();const segments=useSegments();const auth=useStore(s=>s.auth);const ready=useReady();const toast=useToast();useEffect(()=>{void hydrate();void hydrateLanguage()},[]);useEffect(()=>{if(!ready)return;if(!auth&&segments[0]!=='login')router.replace('/login');else if(auth&&segments[0]==='login')router.replace('/(tabs)')},[ready,auth,segments,router]);return <><StatusBar style="dark"/><Stack screenOptions={{headerShown:false}}/><SheetHost/>{toast&&<View style={{position:'absolute',left:20,right:20,bottom:90,backgroundColor:colors.fg,borderRadius:10,padding:14,flexDirection:'row',justifyContent:'space-between'}}><Text style={[text,{color:'white',flex:1}]}>{toast.text}</Text>{toast.undo&&<Pressable onPress={toast.undo}><Text style={{color:colors.brand}}>Undo</Text></Pressable>}</View>}</>}
export default function RootLayout(){useFonts({Montserrat:require('../../assets/fonts/Montserrat.ttf')});return <SafeAreaProvider><Gate/></SafeAreaProvider>}
