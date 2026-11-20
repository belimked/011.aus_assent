import { test, expect } from '@playwright/test';
import { login } from './helpers.js';

test('admin audit, Rex, market import and data browser routes render', async ({ page }) => {
  await login(page);
  for (const [link, heading] of [
    ['审计日志', '审计日志'], ['Rex 队列', 'Rex 队列'],
    ['市场数据', '市场数据'], ['数据浏览', '数据浏览'],
  ]) {
    await page.locator('nav').getByRole('link', { name: link }).click();
    await expect(page.getByRole('heading', { name: heading })).toBeVisible();
    await expect(page.getByRole('alert')).toHaveCount(0);
  }
});
