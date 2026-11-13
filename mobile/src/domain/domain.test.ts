import {seed} from '../data/seed';
import {extract} from './extract';
import {mask,maskText,money,propColor,myTodoSets} from './index';
test('extract recognises seller, phone, timing and tags',()=>{const r=extract('Spoke to Mary Chen, mobile 0412 555 019. Kids moved out, might sell next year.', 'dk');expect(r.first).toBe('Mary');expect(r.mobile).toBe('0412 555 019');expect(r.tags).toContain('Downsizing');expect(r.followup).toBe('2027-01-15')});
test('masks phone numbers everywhere',()=>{expect(mask('0412 555 019')).toBe('0412 555 ••••');expect(maskText('Call 0412 555 019')).toContain('0412 555 ••••')});
test('money formatting',()=>{expect(money(1180000)).toBe('$1.18m');expect(money(29500)).toBe('$30k')});
test('property dot precedence',()=>{const s=seed();expect(propColor(s.properties.find(p=>p.id==='p3')!)).toBe('red');expect(propColor(s.properties.find(p=>p.id==='p5')!)).toBe('pending');expect(propColor(s.properties.find(p=>p.id==='p6')!)).toBe('green')});
test('to-dos split mine, assigned and completed',()=>{const s=seed();const sets=myTodoSets(s);expect(sets.mine.some(t=>t.id==='t7')).toBe(true);expect(sets.by.some(t=>t.id==='t8')).toBe(true);s.todos.find(t=>t.id==='t7')!.done=true;expect(myTodoSets(s).done.some(t=>t.id==='t7')).toBe(true)});
