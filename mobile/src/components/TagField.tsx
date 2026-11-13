import React from 'react';
import {View} from 'react-native';
import {Chip,Button,SectionLabel} from './ui';
import {openSheet,getState} from '../store';
import {tagCls} from '../domain';

// tagpick params: {kind:'tagpick', sel:string[], onSave:(tags:string[])=>void, who?:string}.
// sel is copied into local sheet state; onSave receives the final array, including custom/area tags.
export type TagPickParams = {sel:string[];onSave:(tags:string[])=>void;who?:string};
export function TagField({tags,onChange,label='Tags'}:{tags:string[];onChange:(tags:string[])=>void;label?:string}){
  return <View style={{gap:8}}><SectionLabel>{label.toUpperCase()}</SectionLabel><View style={{flexDirection:'row',flexWrap:'wrap',gap:6}}>{tags.map(tag=><Chip key={tag} kind={tagCls(getState(),tag)}>{tag}</Chip>)}</View><Button size="sm" onPress={()=>openSheet({kind:'tagpick',sel:tags,onSave:onChange})}>+ Edit tags</Button></View>;
}
