import React from 'react';
import {Linking,Text} from 'react-native';
import {Property} from '../data/types';
import {Button,KV} from './ui';
import {colors,text} from '../theme';
import {useT} from '../i18n';

const display=(value:unknown)=>value===undefined||value===null||value===''||value==='-'?'—':String(value);
// One sale row: the RP row carries the market sale when RP has none; a separate "Last sale" only when it is a different sale.
export const saleRows=(p:Property)=>{const rp=p.rpdataAttributes as Record<string,string>|undefined;const marketPrice=p.market?.lastSalePrice!=null?'$'+p.market.lastSalePrice.toLocaleString('en-AU'):undefined;const rpSale=!!(rp&&(rp.salePrice||rp.saleDate));return {marketPrice,rpSaleRow:rpSale?[rp!.salePrice,rp!.saleDate,rp!.saleType]:[marketPrice,p.market?.lastSaleDate,rp?.saleType],showLastSale:!!p.market&&(!rp||(rpSale&&!!p.market.lastSaleDate&&p.market.lastSaleDate!==rp.saleDate))}};
export function MarketPanel({property:p}:{property:Property}){
 const {l2}=useT();
 const rp=p.rpdataAttributes as Record<string,string>|undefined;
 const pf=p.pricefinderAttributes as Record<string,string>|undefined;
 const a=rp||pf;
 const {marketPrice,rpSaleRow,showLastSale}=saleRows(p);
 const label=p.market?.source?.toLowerCase()==='rpdata'||rp?'RP Data':p.market?.source?.toLowerCase()==='pricefinder'||pf?'Pricefinder':display(p.market?.source);
 const fields: [string,string|undefined][] = rp ? [
  [l2('Property type','房型'),rp.propertyType], [l2('Bed / bath / car','卧室 / 浴室 / 车位'),[rp.beds,rp.baths,rp.cars].map(display).join(' / ')],
  [l2('Land / floor area','土地 / 建筑面积'),[rp.land,rp.floorSize].map(display).join(' / ')],
  [l2('Year built','建成年份'),rp.yearBuilt],[l2('Land use','用途'),rp.landUse],[l2('Zone','分区'),rp.developmentZone],
  [l2('Parcel','地块'),rp.parcel],[l2('Last sale price / date / type','最近成交价 / 日期 / 类型'),rpSaleRow.map(display).join(' · ')],
  [l2('Agency / agent','中介公司 / 经纪人'),[rp.agency,rp.agent].map(display).join(' · ')],
 ] : pf ? [[l2('Bed / bath / car','卧室 / 浴室 / 车位'),[pf.Bedrooms,pf.Bathrooms,pf['Car Parks']].map(display).join(' / ')],[l2('Land use','用途'),pf['Land Use']],[l2('Zone','分区'),pf.Zoning]] : [];
 return <>{fields.map(([name,value])=><KV key={name} label={name} value={display(value)}/>)}
 {p.market&&showLastSale&&<KV label={l2('Last sale','上次成交')} value={`${marketPrice||'—'} · ${display(p.market.lastSaleDate)}`}/>}
 {(p.market||a)&&<Text style={[text,{fontSize:11,color:colors.muted}]}>{l2('Source: ','来源：')}{label} · {l2('updated','更新于')} {display(rp?.updatedAt?.slice(0,10)||p.market?.updatedAt?.slice(0,10))}</Text>}
 {rp?.corelogicUrl?.startsWith('https://rpp.corelogic.com.au/property/')&&<Button onPress={()=>void Linking.openURL(rp.corelogicUrl)}>{l2('Open in RP Data','在 RP Data 中打开')}</Button>}
 </>;
}
