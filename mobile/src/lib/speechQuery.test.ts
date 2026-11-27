import {speechQuery} from './speechQuery';

test('trims and drops sentence-ending punctuation in English and Chinese',()=>{
  expect(speechQuery('  Mary Smith.  ')).toBe('Mary Smith');
  expect(speechQuery('who owns 4 Gager St?')).toBe('who owns 4 Gager St');
  expect(speechQuery('玛丽。')).toBe('玛丽');
  expect(speechQuery('王先生？')).toBe('王先生');
  expect(speechQuery('Runcorn!')).toBe('Runcorn');
  expect(speechQuery('张伟！')).toBe('张伟');
  expect(speechQuery('Sunnybank,')).toBe('Sunnybank');
  expect(speechQuery('李娜，')).toBe('李娜');
});
test('drops a run of trailing marks and spaces, keeps punctuation inside the words',()=>{
  expect(speechQuery('0433 123 456. ?')).toBe('0433 123 456');
  expect(speechQuery('Gager St, Runcorn。')).toBe('Gager St, Runcorn');
  expect(speechQuery('mary@example.com.')).toBe('mary@example.com');
  expect(speechQuery('Mr. Chen')).toBe('Mr. Chen');
});
test('leading punctuation is left alone; nothing left means an empty term',()=>{
  expect(speechQuery('?Mary')).toBe('?Mary');
  expect(speechQuery(' 。 ')).toBe('');
  expect(speechQuery('')).toBe('');
});
