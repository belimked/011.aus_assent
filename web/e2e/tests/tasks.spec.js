import { test, expect } from '@playwright/test';
import { login, agentToken, api, base } from './helpers.js';

test('create, assign and complete a task; leader cannot access another team', async ({ page, request }) => {
  await login(page);
  await page.locator('nav').getByRole('link', { name: '跟进任务' }).click();
  const title = `Web E2E follow-up ${Date.now()}`;
  await page.getByRole('button', { name: '新建跟进' }).click();
  const dialog = page.getByRole('dialog', { name: '新建跟进' });
  await dialog.getByRole('textbox', { name: '内容' }).fill(title);
  await dialog.locator('.el-form-item').filter({ hasText: '经纪人' }).locator('.el-select__wrapper').click();
  await page.getByRole('option', { name: 'Mary' }).click();
  await dialog.getByRole('combobox', { name: '日期' }).fill('2030-06-15');
  await dialog.getByRole('combobox', { name: '日期' }).press('Enter');
  await dialog.getByRole('button', { name: '创建' }).click();
  const card = page.locator('.tk').filter({ hasText: title });
  await expect(card).toBeVisible();
  await expect(card).toContainText('Mary');
  await card.locator('.el-select__wrapper').click();
  await page.getByRole('option', { name: 'John' }).click();
  await expect(card).toContainText('John');
  const leader = await agentToken(request, 'mary@axleo.local');
  const tasks = await api(request, leader, 'admin/todos');
  expect(tasks.some(task => task.t === title)).toBe(false);
  const adminToken = await agentToken(request, 'admin@axleo.local');
  const adminTasks = await api(request, adminToken, 'admin/todos');
  const task = adminTasks.find(task => task.t === title);
  expect(task).toBeTruthy();
  const forbidden = await request.patch(`${base}/api/v1/admin/todos/${task.id}`, {
    headers: { Authorization: `Bearer ${leader}` }, data: { version: task.version, done: true },
  });
  expect(forbidden.status()).toBe(403);
  await card.locator('label.el-checkbox').filter({ hasText: '完成' }).click();
  await expect(card).toHaveCount(0);
  await page.locator('label.el-checkbox').filter({ hasText: '显示已完成' }).click();
  await expect(page.locator('.tk.done').filter({ hasText: title })).toBeVisible();
});
