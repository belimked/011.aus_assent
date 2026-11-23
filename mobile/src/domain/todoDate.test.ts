import {validTodoDate,validTodoTime,validTodoSchedule} from './todoDate';

test('accepts real ISO calendar dates including leap days',()=>{
  expect(validTodoDate('2024-02-29')).toBe(true);
  expect(validTodoDate('2026-12-31')).toBe(true);
  for(const date of ['20211-1','2026-2-01','2025-02-29','2026-04-31','0000-01-01','2026-13-01','2026-00-01','2026-01-00','2026-01-01x'])expect(validTodoDate(date)).toBe(false);
});
test('accepts only 24-hour HH:MM',()=>{
  expect(validTodoTime('00:00')).toBe(true);
  expect(validTodoTime('23:59')).toBe(true);
  for(const time of ['24:00','9:00','12:60','12:3','12:30x',''])expect(validTodoTime(time)).toBe(false);
  expect(validTodoSchedule('20211-1','16:00')).toBe(false);
});
