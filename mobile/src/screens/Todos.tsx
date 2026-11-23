import React,{useCallback,useEffect,useMemo,useRef,useState} from 'react';
import {Animated,PanResponder,Pressable,Text,View} from 'react-native';
import {Screen} from '../components/Screen';
import {useRouter} from 'expo-router';
import {Button,Card,Chip,Icon,SectionLabel,Seg} from '../components/ui';
import {colors,text} from '../theme';
import {useStore,update,openSheet,showToast,getState} from '../store';
import {useT} from '../i18n';
import {myTodoSets,todoLink,shortAddr,cname} from '../domain';
import {setTodosDone,revertTodos} from '../domain/todo';
import {validTodoDate} from '../domain/todoDate';
import {TODAY} from '../data/seed';
import type {Todo} from '../data/types';

// Shared by the Home and To-dos lists: opening a row closes the previous one.
let openRow: {key:Animated.Value;close:()=>void}|null=null;
function closeOther(key:Animated.Value){if(openRow&&openRow.key!==key)openRow.close();}
function registerOpen(key:Animated.Value,close:()=>void){closeOther(key);openRow={key,close};}
function unregisterOpen(key:Animated.Value){if(openRow?.key===key)openRow=null;}
function makeRowPan({x,width,enabled,opened,base,draggedAt,close,settle}:{x:Animated.Value;width:number;enabled:boolean;opened:React.RefObject<boolean>;base:React.RefObject<number>;draggedAt:React.RefObject<number>;close:()=>void;settle:(open:boolean)=>void}){
  return PanResponder.create({
    onStartShouldSetPanResponderCapture:()=>{closeOther(x);return false;},
    onMoveShouldSetPanResponderCapture:(_,g)=>enabled&&Math.abs(g.dx)>=8&&Math.abs(g.dx)>Math.abs(g.dy),
    onMoveShouldSetPanResponder:(_,g)=>enabled&&Math.abs(g.dx)>=8&&Math.abs(g.dx)>Math.abs(g.dy),
    onPanResponderGrant:()=>{closeOther(x);base.current=opened.current?-width:0;x.stopAnimation();},
    onPanResponderMove:(_,g)=>{x.setValue(Math.max(-width-12,Math.min(0,base.current+g.dx)));},
    onPanResponderRelease:(_,g)=>{draggedAt.current=Date.now();settle(Math.max(-width-12,Math.min(0,base.current+g.dx))<-width/2);},
    onPanResponderTerminate:()=>{draggedAt.current=Date.now();settle(opened.current);},
    onPanResponderTerminationRequest:()=>false,
  });
}

export function TodoRow({todo,selected,onSelect}:{todo:Todo;selected?:boolean;onSelect?:()=>void}){
  const state=useStore(s=>s);
  const router=useRouter();
  const {c,p}=todoLink(state,todo);
  const {l2}=useT();
  const over=!todo.done&&validTodoDate(todo.d)&&todo.d<TODAY;
  const me=state.auth?.name||'Owen Chen';
  const width=todo.done?76:152;
  const [x]=useState(()=>new Animated.Value(0));
  const opened=useRef(false);
  const draggedAt=useRef(0);
  const base=useRef(0);
  const close=useCallback(()=>{opened.current=false;unregisterOpen(x);Animated.timing(x,{toValue:0,duration:180,easing:t=>1-(1-t)*(1-t),useNativeDriver:true}).start();},[x]);
  const settle=useCallback((open:boolean)=>{
    opened.current=open;
    if(open)registerOpen(x,close);
    else unregisterOpen(x);
    Animated.timing(x,{toValue:open?-width:0,duration:180,easing:t=>1-(1-t)*(1-t),useNativeDriver:true}).start();
  },[close,width,x]);
  useEffect(()=>()=>{unregisterOpen(x);x.stopAnimation();},[x]);
  useEffect(()=>{if(onSelect&&opened.current)close();},[onSelect,close]);
  // Refs are only read by PanResponder's event callbacks, never by the factory itself.
  // eslint-disable-next-line react-hooks/refs
  const pan=useMemo(()=>makeRowPan({x,width,enabled:!onSelect,opened,base,draggedAt,close,settle}),[onSelect,close,settle,width,x]);
  const toggleDone=()=>{const before=getState();const done=!todo.done;update(s=>{setTodosDone(s,[todo.id],done);});close();showToast(done?l2('Done · moved to Completed','已完成 · 移到 Completed'):l2('Reopened','已重新打开'),()=>update(revertTodos(before,[todo.id])));};
  const onRowPress=()=>{
    if(onSelect){onSelect();return;}
    if(Date.now()-draggedAt.current<350)return;
    if(opened.current){close();return;}
    if(!validTodoDate(todo.d)){openSheet({kind:'todo',editId:todo.id});return;}
    if(c)router.push(`/contact/${c.id}`);
    else if(p)router.push(`/prop/${p.id}`);
    else openSheet({kind:'todo',editId:todo.id});
  };
  return <View style={{borderBottomWidth:1,borderColor:colors.line,overflow:'hidden'}}>
    {!onSelect&&<View style={{position:'absolute',top:0,bottom:0,right:0,flexDirection:'row'}}>
      {!todo.done&&<Pressable accessibilityLabel={l2('Reassign','转交')} onPress={()=>{close();openSheet({kind:'tdre',ids:[todo.id]});}} style={{width:76,alignItems:'center',justifyContent:'center',backgroundColor:colors.surface2,gap:2}}><Icon name="swap" size={20}/><Text style={[text,{fontSize:12,fontWeight:'700'}]}>{l2('Reassign','转交')}</Text></Pressable>}
      <Pressable accessibilityLabel={todo.done?l2('Reopen','重新打开'):l2('Done','完成')} onPress={toggleDone} style={{width:76,alignItems:'center',justifyContent:'center',backgroundColor:colors.brand,gap:2}}><Icon name="check" size={20}/><Text style={[text,{fontSize:12,fontWeight:'700'}]}>{todo.done?l2('Reopen','重新打开'):l2('Done','完成')}</Text></Pressable>
    </View>}
    <Animated.View {...(!onSelect?pan.panHandlers:{})} style={{transform:[{translateX:x}],backgroundColor:selected?colors.surface2:colors.surface,...({userSelect:'none'} as object)}}>
      <Pressable onPress={onRowPress} onLongPress={()=>onSelect?.()} style={{minHeight:68,flexDirection:'row',alignItems:'flex-start',gap:10,paddingVertical:10,paddingHorizontal:12}}>
        {onSelect&&<View style={{width:18,height:18,borderWidth:1,borderRadius:4,borderColor:colors.steel,backgroundColor:selected?colors.brand:colors.surface,alignItems:'center',justifyContent:'center'}}>{selected&&<Icon name="check" size={15} color={colors.fg}/>}</View>}
        <View style={{flex:1,gap:4}}><Text style={[text,{fontWeight:'700',textDecorationLine:todo.done?'line-through':'none',color:todo.done?colors.muted:colors.fg}]} numberOfLines={2}>{todo.t}</Text>
          <Text style={[text,{fontSize:12,color:colors.muted}]}>{c?cname(c)+' · ':''}{p?shortAddr(p)+' · ':''}{todo.d===TODAY?l2('Today','今天'):todo.d}{todo.time?' '+todo.time:''}{!c&&!p?' · '+todo.who:''}</Text>
          {todo.done?<Chip kind="green">✓ {l2('Done','已完成')}</Chip>:todo.by&&todo.by!==todo.who&&todo.who===me?<Chip>{l2('From','来自')} {todo.by}</Chip>:todo.by===me&&todo.who!==me?<Chip kind="blue">{l2('Assigned to','已指派给')} {todo.who.split(' ')[0]} · {l2('by you','由你指派')}</Chip>:null}
        </View>
        {over?<Chip kind="red">{l2('Overdue','已逾期')}</Chip>:!onSelect?<Text style={[text,{color:colors.steel,fontSize:20}]}>›</Text>:null}
      </Pressable>
    </Animated.View>
  </View>;
}

export function Todos(){
  const state=useStore(s=>s);
  const sets=myTodoSets(state);
  const {l2}=useT();
  const [tab,setTab]=useState('mine');
  const [select,setSelect]=useState(false);
  const [ids,setIds]=useState<string[]>([]);
  const list=sets[tab as keyof typeof sets];
  const group=(label:string,items:Todo[])=><React.Fragment key={label}>{items.length>0&&<><SectionLabel>{label}</SectionLabel><Card style={{padding:0,overflow:'hidden'}}>{items.map(item=><TodoRow key={item.id} todo={item} selected={ids.includes(item.id)} onSelect={select?()=>setIds(old=>old.includes(item.id)?old.filter(id=>id!==item.id):[...old,item.id]):undefined}/>)}</Card></>}</React.Fragment>;
  return <Screen title="To-dos" back dock={select&&<View style={{flexDirection:'row',gap:10,padding:12,backgroundColor:colors.bg}}><Button style={{flex:1}} disabled={!ids.length} onPress={()=>openSheet({kind:'tdre',ids})}>{l2('Reassign','转交')}</Button><Button variant="primary" style={{flex:1}} disabled={!ids.length} onPress={()=>{const before=getState();const picked=[...ids];let n=0;update(s=>{n=setTodosDone(s,picked,true);});setIds([]);setSelect(false);showToast(l2(`${n} done · moved to Completed`,`已完成 ${n} 条 · 移到 Completed`),()=>update(revertTodos(before,picked)));}}>{l2('Complete','完成')}{ids.length?' '+ids.length:''}</Button></View>}>
    <Seg items={[["mine",`Mine · ${sets.mine.length}`],["by",`Assigned by me · ${sets.by.length}`],["done",'Completed']]} value={tab} onChange={v=>{setTab(v);setSelect(false);setIds([]);}}/>
    {tab!=='done'&&list.length>0&&<View style={{flexDirection:'row',alignItems:'center',gap:12}}><Text style={[text,{color:colors.muted,fontSize:12,flex:1}]}>{select?`${ids.length} ${l2('selected','已选')}`:l2('Swipe left on a to-do: Reassign or Done.','向左滑动待办：转交或完成。')}</Text>{select&&<Pressable onPress={()=>setIds(ids.length===list.length?[]:list.map(item=>item.id))}><Text style={[text,{fontSize:12,color:colors.link,fontWeight:'700'}]}>{ids.length===list.length?l2('Clear all','全不选'):l2('Select all','全选')}</Text></Pressable>}<Pressable onPress={()=>{setSelect(!select);setIds([]);}}><Text style={[text,{fontSize:12,color:colors.link,fontWeight:'700'}]}>{select?l2('Cancel','取消'):l2('Select','多选')}</Text></Pressable></View>}
    {tab==='done'?group('COMPLETED',list):<>{group('OVERDUE',list.filter(item=>validTodoDate(item.d)&&item.d<TODAY))}{group('TODAY',list.filter(item=>item.d===TODAY))}{group('UPCOMING',list.filter(item=>validTodoDate(item.d)&&item.d>TODAY))}{group(l2('INVALID DATE','无效日期'),list.filter(item=>!validTodoDate(item.d)))}</>}
    {!list.length&&<Card><Text style={text}>{l2('Nothing here.','这里没有待办。')}</Text></Card>}
    {!select&&<><Button block variant="ghost" onPress={()=>openSheet({kind:'todo'})}>+ To-do</Button><Text style={[text,{fontSize:12,color:colors.muted}]}>{tab==='done'?l2('Swipe left on a to-do to reopen it.','向左滑动待办可重新打开。'):l2('Tap a to-do to open the contact record.','点待办打开联系人记录。')}</Text></>}
  </Screen>;
}
