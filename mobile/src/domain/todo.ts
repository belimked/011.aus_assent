import type {AppState,Contact} from '../data/types';
import {today} from '../lib/today';
import {audit,byId,contactNoteId} from './index';

// Prototype tdreto / tdtick / tdbulkdone: changes apply immediately, note the linked contact, and offer Undo.
const sysNote=(c:Contact,t:string)=>{c.notes.unshift({id:contactNoteId(),d:today(),at:new Date().toISOString(),a:'AxleoCollect',t,sys:'todo'});};

export function reassignTodos(s:AppState,ids:string[],who:string,whoId?:string):number{
  const me=s.auth?.name||'Owen Chen';let n=0;
  for(const x of s.todos.filter(t=>ids.includes(t.id))){
    if(x.who===who)continue;
    x.by=me;x.who=who;if(whoId)x.assigneeId=whoId;n++;
    audit(s,'To-do reassigned',`${x.t} · ${who}`);
    const c=x.cid?byId(s.contacts,x.cid):undefined;if(c)sysNote(c,`To-do reassigned to ${who}: ${x.t}`);
  }
  return n;
}

export function setTodosDone(s:AppState,ids:string[],done:boolean):number{
  let n=0;
  for(const x of s.todos.filter(t=>ids.includes(t.id))){
    if(!!x.done===done)continue;
    x.done=done;n++;
    if(done){x.doneAt=today();audit(s,'To-do done',x.t);const c=x.cid?byId(s.contacts,x.cid):undefined;if(c)sysNote(c,`To-do done: ${x.t}`);}
    else delete x.doneAt;
  }
  return n;
}

/** Undo for the actions above: restores the touched to-dos and their contacts, drops activity added since. Keeps current versions so the restore syncs as a normal edit. */
export function revertTodos(before:AppState,ids:string[]){
  const cids=new Set(before.todos.filter(t=>ids.includes(t.id)).map(t=>t.cid).filter(Boolean) as string[]);
  const known=new Set(before.audit.map(a=>a.id).filter(Boolean));
  return (d:AppState)=>{
    d.todos=d.todos.map(t=>{const o=ids.includes(t.id)?byId(before.todos,t.id):undefined;return o?{...o,version:t.version}:t;});
    d.contacts=d.contacts.map(c=>{const o=cids.has(c.id)?byId(before.contacts,c.id):undefined;return o?{...o,version:c.version}:c;});
    d.audit=d.audit.filter(a=>!a.id||known.has(a.id));
  };
}
