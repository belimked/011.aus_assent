import React,{useEffect,useState} from 'react';
import {Text,View} from 'react-native';
import type {Property} from '../data/types';
import {KV} from './ui';
import {colors,text} from '../theme';
import {useT} from '../i18n';
import {fetchOwnerNames} from '../store/sync';
import {cachedOwnerNames,ownerNamesView,type OwnerNamesView} from '../domain/ownerNames';

/** RP Data registered owner, read on demand (UAT 3 item 6). Shown only, never used to create a contact. */
export function RegisteredOwner({property:p}:{property:Property}){
 const {l2}=useT();
 const has=!!p.market?.hasOwners;
 const initial=():OwnerNamesView|null=>{if(!has)return null;const hit=cachedOwnerNames(p.id);return hit?{status:'ok',data:hit}:{status:'loading'};};
 const key=`${p.id}|${has}`;
 const [loaded,setLoaded]=useState<{key:string;view:OwnerNamesView|null}|null>(null);
 useEffect(()=>{let live=true;void ownerNamesView(p.id,has,fetchOwnerNames).then(v=>{if(live)setLoaded({key,view:v});});return ()=>{live=false};},[key,p.id,has]);
 const view=loaded?.key===key?loaded.view:initial();
 if(!view)return null;
 const small=[text,{fontSize:11,color:colors.muted}];
 if(view.status==='unavailable')return <Text style={small}>{l2('Registered owner unavailable offline','离线时无法显示登记业主')}</Text>;
 if(view.status==='ok'&&!view.data.owners)return null;
 return <View><KV label={l2('Registered owner','登记业主')} value={view.status==='ok'?view.data.owners:l2('Loading…','加载中…')}/><Text style={small}>{l2('RP Data · not for direct marketing','RP Data · 不可用于营销')}</Text></View>;
}
