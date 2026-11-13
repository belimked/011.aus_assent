import type {AppState, Contact, Property, Todo} from '../data/types';
import {TODAY} from '../data/seed';
export const byId = <T extends {id:string}>(arr:T[],id:string) => arr.find(x=>x.id===id);
export const propAddr = (p:Property) => `${p.no} ${p.street}, ${p.suburb}`;
export const shortAddr = (p:Property) => `${p.no} ${p.street}`;
export const cname = (c:Contact) => `${c.first} ${c.last}`.trim();
export const money = (n:number) => n>=1e6?'$'+(n/1e6).toFixed(2).replace(/0$/,'')+'m':'$'+Math.round(n/1000)+'k';
export const propColor = (p:Property) => p.flags.dnc?'red':p.pending?'pending':p.ownerIds.length?(p.otherAgent?'sky':'green'):'orange';
export const yearsSince = (ym:string) => ym?2026-parseInt(ym.slice(0,4),10):'';
export const normMobile = (m:string) => (m||'').replace(/\D/g,'');
export const mask = (m:string) => {if(!m)return '—'; const d=normMobile(m);return d.length<8?m:d.slice(0,4)+' '+d.slice(4,7)+' ••••'};
export const maskEmail = (e:string) => {if(!e)return '—';const [u,d]=e.split('@');return u.slice(0,2)+'••••@'+d};
export const maskText = (s:string) => String(s??'').replace(/(^|[^0-9])(0[45]\d{2})[\s-]?(\d{3})[\s-]?\d{3}(?![0-9])/g,'$1$2 $3 ••••').replace(/[\w.+-]+@[\w-]+\.[\w.-]*[A-Za-z]/g,m=>maskEmail(m));
export const TAG_GROUPS = [{id:'g_visit',tags:[['Not at home','steel'],['Left card','steel'],['Friendly','pink'],['Neighbour','blue']]},{id:'g_intent',tags:[['Seller lead','flag'],['Hot','hot'],['Warm','warm'],['Cold','cold'],['Not interested','steel']]},{id:'g_limit',tags:[['Do not contact','ban'],['No junk mail','steel']]},{id:'g_collected',tags:[['Email provided','green'],['Phone provided','green']]},{id:'g_follow',tags:[['Market update','blue'],['Appraisal booked','amber']]},{id:'g_reason',tags:[['Downsizing','blue'],['Upsizing','blue'],['Moving for school','blue'],['Relocating','blue'],['Investor','blue']]}];
export const REASONS=['Downsizing','Upsizing','Moving for school','Relocating','Investor'];
export const REL_TYPES=['Partner of','Co-owner with','Family member of','Tenant of','Previous owner of','Buyer for','Purchaser of','Lawyer / conveyancer for','Trades for'];
export const tagCls=(state:AppState,name:string)=>TAG_GROUPS.flatMap(g=>g.tags).find(([n])=>n===name)?.[1]??(name.startsWith('Area: ')?'zone':'mine');
export const ownersOf=(state:AppState,p:Property)=>p.ownerIds.map(id=>byId(state.contacts,id)).filter((c):c is Contact=>!!c);
export const matchContacts=(state:AppState,{mobile,email}:{mobile?:string;email?:string})=>{const nm=normMobile(mobile||'');const em=(email||'').toLowerCase();const list=state.contacts.flatMap(c=>{const by=[];if(nm.length>=10&&normMobile(c.mobile)===nm)by.push('mobile');if(em&&c.email.toLowerCase()===em)by.push('email');return by.length?[{c,by:by.join('+')}]:[]});return {status:list.length===0?'none':list.length===1?'unique':'multiple',list}};
export function myTodoSets(state:AppState,me=state.auth?.name||'Owen Chen'){const sort=(a:Todo,b:Todo)=>(a.d+(a.time||'')).localeCompare(b.d+(b.time||''));return {mine:state.todos.filter(x=>!x.done&&x.who===me).sort(sort),by:state.todos.filter(x=>!x.done&&(x.by||x.who)===me&&x.who!==me).sort(sort),done:state.todos.filter(x=>x.done&&(x.who===me||(x.by||x.who)===me)).sort(sort).reverse()};}
export function todoLink(state:AppState,x:Todo){const tl=x.t.toLowerCase();return {c:byId(state.contacts,x.cid||'')||state.contacts.find(c=>tl.includes(cname(c).toLowerCase())),p:byId(state.properties,x.propId||'')||state.properties.slice().sort((a,b)=>b.no.length-a.no.length).find(p=>new RegExp('(?:^|\\D)'+shortAddr(p).toLowerCase().replace(/[.*+?^${}()|[\]\\]/g,'\\$&')+'(?:\\b|$)').test(tl))};}
export function audit(state:AppState,action:string,detail:string){state.audit.unshift({d:TODAY+' '+new Date().toTimeString().slice(0,5),a:action,t:detail});state.audit=state.audit.slice(0,50);if(state.rexDown&&!['Sign in','Sign out','Sync'].includes(action))state.syncq.unshift({id:'q'+Date.now(),d:TODAY,what:action+' · '+detail,status:'failed',tries:1,err:'HTTP 503 · api.rexsoftware.com'});}
export function addTodo(state:AppState,o:Partial<Todo>&Pick<Todo,'t'|'d'>){state.todos.push({id:'t'+Date.now(),who:state.auth?.name||'Owen Chen',kind:'call',...o});}
export function noteOn(state:AppState,c:Contact,note:string){c.notes.unshift({d:TODAY,a:state.auth?.name||'Owen Chen',t:note});c.updated={by:state.auth?.name||'Owen Chen',d:TODAY};}
export function upsertContact(state:AppState,fields:Partial<Contact>){let c=state.contacts.find(x=>fields.mobile&&normMobile(x.mobile)===normMobile(fields.mobile!));if(!c){c={id:'c'+Date.now(),first:'',last:'',mobile:'',email:'',owner:state.auth?.name||'Owen Chen',roles:[],tags:[],flags:{},notes:[],...fields};state.contacts.push(c)}else Object.assign(c,fields);return c;}
export function linkOwner(state:AppState,c:Contact,p:Property){if(!p.ownerIds.includes(c.id))p.ownerIds.push(c.id);}
