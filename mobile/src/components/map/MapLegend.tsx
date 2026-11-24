import React from 'react';
import {Text,View} from 'react-native';
import {colors,text} from '../../theme';
import {useT} from '../../i18n';

// Same order and labels as prototype v14's full-map legend; the last three
// entries describe the additional markers drawn by mapPage.ts.
export function MapLegend({onHeight}:{onHeight?:(height:number)=>void}){
 const {l2}=useT();
 const rows=[
  ['owner',colors.green,l2('Owner contact linked','已关联业主联系人')],
  ['empty',colors.red,l2('No contact yet','尚无联系人')],
  ['agency',colors.sky,l2("Another agent's contact",'其他中介的联系人')],
  ['place',colors.fg,l2('Listed with your agency','本公司挂牌')],
  ['other','#FF9F1A',l2('Listed, other agency','其他中介挂牌')],
  ['pin','#F5A300',l2('Pinned by you','你置顶的')],
  ['dnc','#D7263D',l2('Do not contact','请勿联系')],
  ['pricefinder',colors.steel,l2('Pricefinder parcel','Pricefinder 房产')],
  ['pending',colors.steel,l2('Pending','待处理')],
  ['me',colors.fg,l2('You · live','你的实时位置')],
 ] as const;
 return <View onLayout={onHeight?e=>onHeight(e.nativeEvent.layout.height):undefined} style={{position:'absolute',left:12,right:12,bottom:12,zIndex:9,backgroundColor:'rgba(255,255,255,0.95)',borderRadius:12,paddingVertical:10,paddingHorizontal:12,flexDirection:'row',flexWrap:'wrap',columnGap:10,rowGap:6,shadowColor:'#000',shadowOpacity:.14,shadowRadius:8,shadowOffset:{width:0,height:2},elevation:5}}>
  {rows.map(([kind,color,label])=><View key={kind} style={{flexBasis:'48%',flexDirection:'row',alignItems:'center',gap:6,minHeight:22}}>
   <View style={{width:22,height:28,alignItems:'center',justifyContent:'center'}}>
    {kind==='pin'?<Text style={{fontSize:20,lineHeight:24,color,fontWeight:'700',textShadowColor:'#fff',textShadowRadius:2}}>★</Text>
    :kind==='dnc'?<View style={{width:26,height:26,borderRadius:13,borderWidth:2,borderColor:color,backgroundColor:'#fff',alignItems:'center',justifyContent:'center'}}><Text style={{color,fontSize:21,lineHeight:23,fontWeight:'900'}}>⊘</Text></View>
    :kind==='me'?<View style={{width:26,height:26,borderRadius:13,backgroundColor:color,borderWidth:2,borderColor:'#fff',alignItems:'center',justifyContent:'center',shadowColor:colors.azure,shadowOpacity:.55,shadowRadius:5}}><View style={{width:8,height:8,borderRadius:4,backgroundColor:'#fff'}}/></View>
    :kind==='place'||kind==='other'?<View style={{width:22,height:22,borderRadius:11,backgroundColor:color,borderWidth:2,borderColor:'#fff',alignItems:'center',justifyContent:'center',shadowColor:'#000',shadowOpacity:.3,shadowRadius:3}}>{kind==='place'&&<Text style={{color:'#fff',fontSize:15,fontWeight:'800',lineHeight:18}}>P</Text>}</View>
    :<View style={{width:kind==='pricefinder'?18:14,height:kind==='pricefinder'?18:14,borderRadius:10,borderWidth:2,borderStyle:kind==='pending'?'dashed':'solid',borderColor:kind==='pending'?colors.steel:'#fff',backgroundColor:kind==='pending'?'#fff':color}}/>}
   </View><Text style={[text,{fontSize:12,lineHeight:16,flexShrink:1,fontWeight:kind==='pin'?'700':'400'}]}>{label}</Text>
  </View>)}
 </View>;
}
