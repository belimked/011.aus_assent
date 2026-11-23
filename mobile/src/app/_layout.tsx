import React,{useEffect} from 'react';
import {Stack,useRouter,useSegments} from 'expo-router';
import {StatusBar} from 'expo-status-bar';
import {SafeAreaProvider} from 'react-native-safe-area-context';
import {useFonts} from 'expo-font';
import {hydrate,useReady,useStore,getState,openSheet} from '../store';
import {Toast} from '../components/Toast';
import {hydrateLanguage} from '../i18n';
import {restoreSync} from '../store/sync';
import {SheetHost} from '../sheets/SheetHost';
import {ShakeListener} from '../components/ShakeListener';
import {Platform} from 'react-native';
import {useShareIntent} from 'expo-share-intent';
import {listenForNotification} from '../lib/notifications';
function Gate(){const router=useRouter();const segments=useSegments();const auth=useStore(s=>s.auth);const ready=useReady();const {hasShareIntent,shareIntent,resetShareIntent}=useShareIntent({disabled:Platform.OS!=='android'});useEffect(()=>{if(!ready||!auth||!hasShareIntent)return;const image=shareIntent.files?.find(f=>f.mimeType.startsWith('image/'));router.push({pathname:'/(tabs)/enq',params:{shareText:shareIntent.text||shareIntent.webUrl||'',shareImage:image?.path||''}});resetShareIntent()},[ready,auth,hasShareIntent,shareIntent,resetShareIntent,router]);useEffect(()=>{if(!ready||!auth)return;return listenForNotification((todoId,contactId)=>{if(!getState().auth)return;const todo=getState().todos.find(t=>t.id===todoId);if(todo){router.push('/todos');openSheet({kind:'todo',editId:todo.id});}else if(contactId&&getState().contacts.some(c=>c.id===contactId))router.push({pathname:'/contact/[id]',params:{id:contactId}});});},[router,ready,auth]);useEffect(()=>{void hydrate().then(restoreSync);void hydrateLanguage()},[]);useEffect(()=>{if(!ready)return;if(!auth&&segments[0]!=='login')router.replace('/login');else if(auth&&segments[0]==='login')router.replace('/(tabs)')},[ready,auth,segments,router]);return <><StatusBar style="dark"/><Stack screenOptions={{headerShown:false}}/><SheetHost/><ShakeListener/><Toast/></>}
export default function RootLayout(){useFonts(Platform.OS==='web'?{Montserrat:require('../../assets/fonts/Montserrat-Variable.ttf')}:{});return <SafeAreaProvider><Gate/></SafeAreaProvider>}
