import React from 'react';
import {NoteMic} from './NoteMic';
import {showToast} from '../store';
import {useT} from '../i18n';
import {speechQuery} from '../lib/speechQuery';

/** Hold-to-speak mic for a search box: release to finish; the cleaned words replace the query and run the search. */
export function SearchMic({value,onSearch,size=34}:{value:string;onSearch:(term:string)=>void;size?:number}){
  const {l2}=useT();
  const words=(spoken:string)=>{const term=speechQuery(spoken);if(term)onSearch(term);else showToast(l2('Could not hear that','没有听清'))};
  return <NoteMic size={size} value={value} onChangeText={onSearch} onWords={words}/>;
}
