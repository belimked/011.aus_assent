import AsyncStorage from '@react-native-async-storage/async-storage';
import * as Notifications from 'expo-notifications';
import Constants from 'expo-constants';
import {Platform} from 'react-native';
import type {AppState,Todo} from '../data/types';
import {notificationText} from '../i18n';

const key='ax_todo_notifications', permissionKey='ax_notification_permission_requested', deviceKey='ax_device_push_token';
const pushEnabled=Constants.expoConfig?.extra?.iosPushEnabled===true && Platform.OS==='ios';
let scheduled:Record<string,{id:string;signature:string}>|null=null;
let work=Promise.resolve();
export function reminderDate(todo:Todo):Date|null {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(todo.d)||!/^([01]\d|2[0-3]):[0-5]\d$/.test(todo.time||'')) return null;
  const date=new Date(`${todo.d}T${todo.time}:00`);
  return Number.isFinite(date.getTime())&&date.getTime()>Date.now()&&date.getFullYear()===Number(todo.d.slice(0,4))&&date.getMonth()+1===Number(todo.d.slice(5,7))&&date.getDate()===Number(todo.d.slice(8,10))?date:null;
}
export function reconcileReminders(state:AppState){
  if(Platform.OS==='web')return;
  work=work.then(async()=>{
    if(!scheduled){try{scheduled=JSON.parse(await AsyncStorage.getItem(key)||'{}')}catch{scheduled={}}}
    const current=scheduled!;
    const wanted=new Map<string,{todo:Todo;date:Date;signature:string}>();
    if(state.auth)for(const todo of state.todos){const date=reminderDate(todo);if(!todo.done&&date&&todo.who===state.auth.name)wanted.set(todo.id,{todo,date,signature:`${date.getTime()}|${todo.t}|${todo.cid||''}`});}
    for(const [id,entry] of Object.entries(current))if(wanted.get(id)?.signature!==entry.signature){await Notifications.cancelScheduledNotificationAsync(entry.id);delete current[id];}
    if(!wanted.size){await AsyncStorage.setItem(key,JSON.stringify(current));return;}
    const permissions=await notificationPermission();
    if(!permissions.granted){await AsyncStorage.setItem(key,JSON.stringify(current));return;}
    if(Platform.OS==='android')await Notifications.setNotificationChannelAsync('todos',{name:notificationText('notification_channel'),importance:3});
    for(const [id,{todo,date,signature}] of wanted)if(!current[id]){
      const notificationId=await Notifications.scheduleNotificationAsync({content:{title:notificationText('notification_todo_title'),body:todo.t,data:{todoId:id,contactId:todo.cid||''}},trigger:{type:Notifications.SchedulableTriggerInputTypes.DATE,date,...(Platform.OS==='android'?{channelId:'todos'}:{})}});
      current[id]={id:notificationId,signature};
    }
    await AsyncStorage.setItem(key,JSON.stringify(current));
  }).catch(e=>console.warn('Reminder scheduling failed',e));
}
async function notificationPermission(){
  let permission=await Notifications.getPermissionsAsync();
  if(!permission.granted && !(await AsyncStorage.getItem(permissionKey))){
    await AsyncStorage.setItem(permissionKey,'1');
    permission=await Notifications.requestPermissionsAsync();
  }
  return permission;
}
export function listenForNotification(open:(todoId:string,contactId:string)=>void){
  const handle=(response:Notifications.NotificationResponse)=>{const data=response.notification.request.content.data||{};open(String(data.todoId||''),String(data.contactId||''));};
  const sub=Notifications.addNotificationResponseReceivedListener(handle);
  void Notifications.getLastNotificationResponseAsync().then(response=>{if(response){handle(response);void Notifications.clearLastNotificationResponseAsync()}}).catch(()=>{});
  return ()=>sub.remove();
}
export async function registerDevice(authToken:string,apiUrl:string){
  if(!pushEnabled)return;
  try{
    const permission=await notificationPermission();
    if(!permission.granted)return;
    const device=await Notifications.getDevicePushTokenAsync();
    if(typeof device.data!=='string')return;
    await AsyncStorage.setItem(deviceKey,device.data);
    const response=await fetch(`${apiUrl}/devices`,{method:'POST',headers:{Authorization:`Bearer ${authToken}`,'Content-Type':'application/json'},body:JSON.stringify({platform:'ios',token:device.data})});
    if(!response.ok)throw new Error(`HTTP ${response.status}`);
  }catch(e){console.warn('APNs registration failed',e)}
}
export async function unregisterDevice(authToken:string,apiUrl:string){
  if(!pushEnabled)return;
  try{const device=await AsyncStorage.getItem(deviceKey);if(device){const response=await fetch(`${apiUrl}/devices`,{method:'DELETE',headers:{Authorization:`Bearer ${authToken}`,'Content-Type':'application/json'},body:JSON.stringify({platform:'ios',token:device})});if(response.ok)await AsyncStorage.removeItem(deviceKey);}}catch(e){console.warn('APNs unregister failed',e)}
}
