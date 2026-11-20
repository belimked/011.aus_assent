import { test, expect } from '@playwright/test';
import { login, base } from './helpers.js';

test('missing session redirects protected pages to login', async ({ page }) => {
  await page.goto('/admin/#/properties');
  await expect(page).toHaveURL(/#\/login$/);
  await login(page);
  await page.evaluate(() => sessionStorage.removeItem('axleo_admin'));
  await page.reload();
  await expect(page).toHaveURL(/#\/login$/);
});

test('revoked session redirects to login on the next API request', async ({ page, request }) => {
  await login(page);
  const token = await page.evaluate(() => sessionStorage.getItem('axleo_admin'));
  const revoke = await request.post(`${base}/api/v1/auth/logout`, { headers: { Authorization: `Bearer ${token}` } });
  expect(revoke.ok()).toBe(true);
  await page.locator('nav').getByRole('link', { name: '房产列表' }).click();
  await expect(page).toHaveURL(/#\/login$/);
  await expect(page.evaluate(() => sessionStorage.getItem('axleo_admin'))).resolves.toBeNull();
});

test('login, wrong password, role-specific navigation and logout', async ({ page }) => {
  await page.goto('/admin/#/login');
  await page.getByRole('textbox', { name: '邮箱' }).fill('admin@axleo.local');
  await page.locator('input[autocomplete="current-password"]').fill('not-the-password');
  await page.getByRole('button', { name: '登录' }).click();
  await expect(page.getByRole('alert')).toBeVisible();
  await expect(page).toHaveURL(/#\/login$/);

  await page.locator('input[autocomplete="current-password"]').fill('web-e2e-password-2026');
  await page.getByRole('button', { name: '登录' }).click();
  await expect(page).toHaveURL(/#\/dashboard$/);
  await expect(page.locator('nav').getByRole('link', { name: '数据导出' })).toBeVisible();
  await page.locator('.top .me').click();
  await page.getByRole('menuitem', { name: '退出登录' }).click();
  await expect(page).toHaveURL(/#\/login$/);
  await expect(page.evaluate(() => sessionStorage.getItem('axleo_admin'))).resolves.toBeNull();

  await login(page, 'mary@axleo.local');
  await expect(page.locator('nav').getByRole('link', { name: '跟进任务' })).toBeVisible();
  await expect(page.locator('nav').getByRole('link', { name: '数据导出' })).toHaveCount(0);
  await page.goto('/admin/#/export');
  await expect(page).toHaveURL(/#\/dashboard$/);
});
