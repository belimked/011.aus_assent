import React from 'react';
import {Text,View} from 'react-native';
import {colors,text} from '../../theme';
import {useT} from '../../i18n';

// Match mapPage.ts: circles, outlined pending, competitor disc, Place badge and DNC ban.
export function MapLegend(){const {l2}=useT();
 const rows:[string,string,string][]= [
  ['owner',colors.green,l2('Owner linked','已关联业主')],
  ['empty',colors.accent,l2('No contact yet','尚无联系人')],
  ['pricefinder',colors.steel,l2('Pricefinder parcel · tap to preview','Pricefinder 房产 · 点击预览')],
  ['agency',colors.sky,l2('Other agency owner','其他中介的业主')],
  ['pending',colors.steel,l2('Pending','待处理')],
  ['dnc','#D7263D',l2('Do not contact','请勿联系')],
  ['pin','#F5A300',l2('Pinned by you','你置顶的')],
  ['place',colors.fg,l2('Place listing','Place 挂牌房源')],
  ['other','#FF9F1A',l2('Listed with other agency','其他中介挂牌')],
  ['me',colors.fg,l2('Your live position','你的实时位置')],
 ];
 return <View style={{flexDirection:'row',flexWrap:'wrap',gap:10}}>{rows.map(([kind,color,label])=><View key={kind} style={{flexDirection:'row',alignItems:'center',gap:6,minWidth:'45%'}}><View style={{width:28,height:28,alignItems:'center',justifyContent:'center'}}>
 {kind==='pin'?<Text style={{fontSize:23,color,fontWeight:'700',textShadowColor:'#fff',textShadowRadius:2}}>★</Text>:kind==='dnc'?<View style={{width:26,height:26,borderRadius:13,borderWidth:2,borderColor:color,backgroundColor:'#fff',alignItems:'center',justifyContent:'center'}}><Text style={{color,fontSize:21,lineHeight:23,fontWeight:'900'}}>⊘</Text></View>:kind==='me'?<View style={{width:24,height:24,borderRadius:12,backgroundColor:color,borderWidth:2,borderColor:'#fff',alignItems:'center',justifyContent:'center',shadowColor:colors.azure,shadowOpacity:.55,shadowRadius:4}}><View style={{width:8,height:8,borderRadius:4,backgroundColor:'#fff'}}/></View>:kind==='place'||kind==='other'?<View style={{width:22,height:22,borderRadius:11,backgroundColor:color,borderWidth:2,borderColor:'#fff',alignItems:'center',justifyContent:'center'}}>{kind==='place'&&<Text style={{color:'#fff',fontSize:14,fontWeight:'800',lineHeight:17}}>P</Text>}</View>:<View style={{width:kind==='pricefinder'?18:14,height:kind==='pricefinder'?18:14,borderRadius:11,borderWidth:kind==='pending'?2:2,borderStyle:kind==='pending'?'dashed':'solid',borderColor:kind==='pending'?colors.steel:'#fff',backgroundColor:kind==='pending'?'#fff':color}}/>}
 </View><Text style={[text,{fontSize:12,flexShrink:1}]}>{label}</Text></View>)}</View>;
}
