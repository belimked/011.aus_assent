import React from 'react';
import {Pressable,Text,View,type ViewStyle} from 'react-native';
import {colors,text} from '../../theme';
import {useT} from '../../i18n';

const FLOAT:ViewStyle={position:'absolute',left:12,bottom:12,zIndex:9,backgroundColor:'rgba(255,255,255,0.95)',borderRadius:12,shadowColor:'#000',shadowOpacity:.14,shadowRadius:8,shadowOffset:{width:0,height:2},elevation:5};

// Same order and labels as prototype v14's full-map legend; the last two
// entries describe the additional markers drawn by mapPage.ts.
// With onToggle the legend is collapsible: closed it is a small "Legend" pill so the map stays visible.
export function MapLegend({onHeight,open=true,onToggle,bottom=12}:{onHeight?:(height:number)=>void;open?:boolean;onToggle?:()=>void;bottom?:number}){
 const {l2}=useT();
 const rows=[
  ['owner',colors.green,l2('Owner contact linked','已关联业主联系人')],
  ['empty',colors.red,l2('No contact yet','尚无联系人')],
  ['agency',colors.sky,l2("Another agent's contact",'其他中介的联系人')],
  ['place',colors.fg,l2('Listed with your agency','本公司挂牌')],
  ['other','#FF9F1A',l2('Listed, other agency','其他中介挂牌')],
  ['pin','#F5A300',l2('Pinned by you','你置顶的')],
  ['dnc','#D7263D',l2('Do not contact','请勿联系')],
  ['pending',colors.steel,l2('Pending','待处理')],
  ['me',colors.fg,l2('You · live','你的实时位置')],
 ] as const;
 const layout=onHeight?(e:{nativeEvent:{layout:{height:number}}})=>onHeight(e.nativeEvent.layout.height):undefined;
 if(onToggle&&!open)return <Pressable testID="map-legend-open" accessibilityRole="button" accessibilityLabel={l2('Show map legend','显示图例')} onLayout={layout} onPress={onToggle} hitSlop={8} style={[FLOAT,{bottom,flexDirection:'row',alignItems:'center',gap:6,paddingHorizontal:12,height:36}]}>
  <View style={{flexDirection:'row'}}>{[colors.green,colors.red,colors.sky].map((c,i)=><View key={c} style={{width:10,height:10,borderRadius:5,backgroundColor:c,borderWidth:1.5,borderColor:'#fff',marginLeft:i?-3:0}}/>)}</View>
  <Text style={[text,{fontSize:13,fontWeight:'700'}]}>{l2('Legend','图例')}</Text><Text style={[text,{fontSize:11,color:colors.muted}]}>▲</Text>
 </Pressable>;
 return <View testID="map-legend" onLayout={layout} style={[FLOAT,{bottom,right:12,paddingTop:onToggle?6:10,paddingBottom:10,paddingHorizontal:12}]}>
  {onToggle&&<Pressable testID="map-legend-close" accessibilityRole="button" accessibilityLabel={l2('Hide map legend','收起图例')} onPress={onToggle} hitSlop={8} style={{flexDirection:'row',alignItems:'center',justifyContent:'space-between',paddingBottom:4}}>
   <Text style={[text,{fontSize:12,fontWeight:'700',color:colors.muted}]}>{l2('Legend','图例')}</Text><Text style={[text,{fontSize:12,color:colors.muted}]}>{l2('Hide','收起')} ▼</Text>
  </Pressable>}
  <View style={{flexDirection:'row',flexWrap:'wrap',columnGap:10,rowGap:4}}>
  {rows.map(([kind,color,label])=><View key={kind} style={{flexBasis:'48%',flexDirection:'row',alignItems:'center',gap:6,minHeight:22}}>
   <View style={{width:22,height:24,alignItems:'center',justifyContent:'center'}}>
    {kind==='pin'?<Text style={{fontSize:18,lineHeight:22,color,fontWeight:'700',textShadowColor:'#fff',textShadowRadius:2}}>★</Text>
    :kind==='dnc'?<View style={{width:22,height:22,borderRadius:11,borderWidth:2,borderColor:color,backgroundColor:'#fff',alignItems:'center',justifyContent:'center'}}><Text style={{color,fontSize:17,lineHeight:19,fontWeight:'900'}}>⊘</Text></View>
    :kind==='me'?<View style={{width:22,height:22,borderRadius:11,backgroundColor:color,borderWidth:2,borderColor:'#fff',alignItems:'center',justifyContent:'center',shadowColor:colors.azure,shadowOpacity:.55,shadowRadius:5}}><View style={{width:7,height:7,borderRadius:4,backgroundColor:'#fff'}}/></View>
    :kind==='place'||kind==='other'?<View style={{width:20,height:20,borderRadius:10,backgroundColor:color,borderWidth:2,borderColor:'#fff',alignItems:'center',justifyContent:'center',shadowColor:'#000',shadowOpacity:.3,shadowRadius:3}}>{kind==='place'&&<Text style={{color:'#fff',fontSize:13,fontWeight:'800',lineHeight:16}}>P</Text>}</View>
    :<View style={{width:14,height:14,borderRadius:10,borderWidth:2,borderStyle:kind==='pending'?'dashed':'solid',borderColor:kind==='pending'?colors.steel:'#fff',backgroundColor:kind==='pending'?'#fff':color}}/>}
   </View><Text style={[text,{fontSize:12,lineHeight:15,flexShrink:1,fontWeight:kind==='pin'?'700':'400'}]}>{label}</Text>
  </View>)}
  </View>
 </View>;
}
