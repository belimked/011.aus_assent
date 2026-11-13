import React from 'react';
import {useLocalSearchParams} from 'expo-router';
import {Sales} from '../../screens/Reviews';
export default function Route(){const {cat}=useLocalSearchParams<{cat:string;per?:string}>();return <Sales cat={cat||''} />};
