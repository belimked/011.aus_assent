import React from 'react';
import {View,Text} from 'react-native';
import type {Property} from '../../data/types';
import {Card,ListItem} from '../ui';
import {propAddr,ownersOf,cname} from '../../domain';
import {useStore} from '../../store';
export function MapView({properties,onSelect,height=320}:{properties:Property[];selected?:string;onSelect:(id:string)=>void;satellite?:boolean;height?:number;onTap?:(lat:number,lng:number)=>void}){
  const state=useStore(s=>s);
  return <View style={{height,overflow:'scroll'}}><Card><Text>Map is available on iOS and Android · List view</Text>{properties.slice(0,12).map(p=><ListItem key={p.id} title={propAddr(p)} subtitle={ownersOf(state,p).map(cname).join(', ')||p.pfOwner||'No contact yet'} onPress={()=>onSelect(p.id)}/>)}</Card></View>;
}
