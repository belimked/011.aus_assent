import { test, expect } from '@playwright/test';
import { login, agentToken, api } from './helpers.js';

test('settings persist after reload; create, assign and rename a team', async ({ page, request }) => {
  const admin = await agentToken(request, 'admin@axleo.local');
  const original = Number((await api(request, admin, 'admin/settings')).sessionHours);
  const changed = original === 12 ? 13 : 12;
  await login(page);
  try {
    await page.locator('nav').getByRole('link', { name: '系统设置' }).click();
    const hours = page.getByRole('spinbutton');
    await expect(hours).toHaveValue(String(original));
    await hours.fill(String(changed));
    await page.getByRole('button', { name: '保存' }).click();
    await expect.poll(async () => (await api(request, admin, 'admin/settings')).sessionHours).toBe(String(changed));
    await page.reload();
    await expect(page.getByRole('spinbutton')).toHaveValue(String(changed));
  } finally {
    // Shared DB: always restore the original value, including when a UI assertion fails.
    await api(request, admin, 'admin/settings', 'PATCH', { sessionHours: String(original) });
  }

  await page.locator('nav').getByRole('link', { name: '团队与权限' }).click();
  const suffix = Date.now();
  const name = `E2E Team ${suffix}`;
  const renamed = `E2E Renamed ${suffix}`;
  const email = `e2e-${suffix}@axleo.local`;
  await page.getByRole('button', { name: '创建团队' }).click();
  const prompt = page.getByRole('dialog').filter({ has: page.getByRole('textbox', { name: '团队名称' }) });
  await prompt.getByRole('textbox', { name: '团队名称' }).fill(name);
  await prompt.getByRole('button', { name: '确定' }).click();
  const team = (await api(request, admin, 'admin/teams')).find(row => row.name === name);
  expect(team).toBeTruthy();
  await expect(page.locator('.team-manage .row').filter({ has: page.getByRole('button', { name: '重命名' }), hasText: name })).toBeVisible();

  await page.getByRole('button', { name: '创建账户' }).first().click();
  const account = page.getByRole('dialog', { name: '创建账户' });
  await account.getByRole('textbox', { name: '邮箱' }).fill(email);
  await account.getByRole('textbox', { name: '姓名' }).fill(`E2E Agent ${suffix}`);
  await account.locator('input[type="password"]').fill('e2e-password-2026');
  await account.getByRole('button', { name: '创建账户' }).click();
  const user = page.locator('.uc').filter({ hasText: email });
  await expect(user).toBeVisible();
  await user.click();
  await page.locator('.team-manage label').filter({ hasText: '团队' }).locator('.el-select__wrapper').click();
  await page.getByRole('option', { name }).click();
  await expect(page.locator('.team-manage .row').filter({ has: page.getByRole('button', { name: '重命名' }), hasText: name })).toContainText('1 人');
  const moved = (await api(request, admin, 'admin/users')).find(row => row.email === email);
  expect(moved.teamId).toBe(team.id);

  await page.locator('.team-manage .row').filter({ has: page.getByRole('button', { name: '重命名' }), hasText: name }).getByRole('button', { name: '重命名' }).click();
  const rename = page.getByRole('dialog').filter({ has: page.getByRole('textbox', { name: '团队名称' }) });
  await rename.getByRole('textbox', { name: '团队名称' }).fill(renamed);
  await rename.getByRole('button', { name: '确定' }).click();
  await expect(page.locator('.team-manage .row').filter({ has: page.getByRole('button', { name: '重命名' }), hasText: renamed })).toContainText('1 人');
  expect((await api(request, admin, 'admin/teams')).find(row => row.id === team.id)?.name).toBe(renamed);
  await page.reload();
  await expect(page.locator('.group-name').filter({ hasText: renamed })).toBeVisible();
  await expect(page.locator('.uc').filter({ hasText: email })).toBeVisible();
});
