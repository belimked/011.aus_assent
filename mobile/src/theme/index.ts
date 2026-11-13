import {Platform} from 'react-native';
export const colors = {bg:'#F6F7FA',surface:'#FFFFFF',surface2:'#EEF1F6',surface3:'#E4E8EF',line:'#E4E8EF',fg:'#13233F',muted:'#5B6475',brand:'#FFB02E',accent:'#FF6B2C',mic:'#FF6B2C',azure:'#2B95F5',green:'#1DB86A',red:'#F04848',sky:'#5BB8F5',steel:'#8A94A6',link:'#1670C2'};
export const spacing = {xs:4,sm:8,md:12,lg:16,xl:24};
export const radii = {sm:8,md:12,lg:16,pill:999};
export const typography = {small:12,body:14,title:17,large:22,font:Platform.select({web:'Montserrat, system-ui, sans-serif',default:'Montserrat'})};
export const text = {color:colors.fg,fontFamily:typography.font,fontSize:14} as const;
