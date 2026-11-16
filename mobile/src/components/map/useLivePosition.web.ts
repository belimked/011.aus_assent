import {useEffect} from 'react';
import {DEMO_POSITION} from '../../data/properties';
import {showToast} from '../../store';
let warned=false;
export function useLivePosition(enabled=true){useEffect(()=>{if(enabled&&!warned){warned=true;showToast('Location unavailable on web · showing demo position')}},[enabled]);return DEMO_POSITION}
