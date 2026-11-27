import {saleRows} from './MarketPanel';
import type {Property} from '../data/types';
jest.mock('./ui',()=>({}));
jest.mock('@react-native-async-storage/async-storage',()=>({__esModule:true,default:{setItem:jest.fn(async()=>{}),getItem:jest.fn(async()=>null)}}));
const prop=(x:Partial<Property>)=>({id:'p',no:'1',street:'A St',suburb:'B',ownerIds:[],pfOwner:'',lastSale:'',flags:{},...x}) as Property;
const market={lastSalePrice:850000,lastSaleDate:'2019-05-12',source:'rpdata',updatedAt:'2026-11-01'};

test('RP Data sale row replaces the duplicate "Last sale" row',()=>{
  const r=saleRows(prop({market,rpdataAttributes:{salePrice:'$850,000',saleDate:'2019-05-12',saleType:'Normal Sale'}}));
  expect(r.showLastSale).toBe(false);
  expect(r.rpSaleRow).toEqual(['$850,000','2019-05-12','Normal Sale']);
});
test('RP Data without a sale shows the market sale in its row',()=>{
  const r=saleRows(prop({market,rpdataAttributes:{saleType:'Normal Sale'}}));
  expect(r.showLastSale).toBe(false);
  expect(r.rpSaleRow).toEqual(['$'+(850000).toLocaleString('en-AU'),'2019-05-12','Normal Sale']);
});
test('a different sale from another source is still shown',()=>{
  expect(saleRows(prop({market,rpdataAttributes:{salePrice:'$1,200,000',saleDate:'2024-02-01'}})).showLastSale).toBe(true);
});
test('Pricefinder-only or market-only keeps "Last sale"',()=>{
  expect(saleRows(prop({market,pricefinderAttributes:{Bedrooms:'3'}})).showLastSale).toBe(true);
  expect(saleRows(prop({market})).showLastSale).toBe(true);
  expect(saleRows(prop({})).showLastSale).toBe(false);
});
