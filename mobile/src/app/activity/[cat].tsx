import React from 'react';
import {useLocalSearchParams} from 'expo-router';
import {Activity} from '../../screens/Reviews';
export default function Route(){const {cat,per}=useLocalSearchParams<{cat:string;per?:string}>();return <Activity cat={cat||''} per={per}/>};
