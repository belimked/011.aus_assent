import React from 'react';
import {Text} from 'react-native';
import {Screen} from '../components/Screen';
import {Card,KV} from '../components/ui';
import {useStore} from '../store';
import {useT} from '../i18n';
import {text} from '../theme';
export default function Data(){const s=useStore(x=>x);const {l2}=useT();return <Screen title={l2('Data','数据')} back><Card><Text style={text}>{l2('Data overview','数据概览')}</Text>{([['Contacts',s.contacts.length],['Properties',s.properties.length],['Listings',s.listings.length],['Enquiries',s.enquiries.length],['Leads',s.leads.length],['To-dos',s.todos.length],['Areas',s.zones.length]] as const).map(([key,count])=><KV key={key} label={key} value={String(count)}/>)}</Card></Screen>}
