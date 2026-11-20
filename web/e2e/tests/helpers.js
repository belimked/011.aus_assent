import { expect } from '@playwright/test';

export const password = 'web-e2e-password-2026';
export const base = 'http://127.0.0.1:8083';

export async function login(page, email = 'admin@axleo.local') {
  await page.goto('/admin/#/login');
  await page.getByRole('textbox', { name: '邮箱' }).fill(email);
  await page.locator('input[autocomplete="current-password"]').fill(password);
  await page.getByRole('button', { name: '登录' }).click();
  await expect(page).toHaveURL(/#\/dashboard$/);
  await expect(page.locator('nav')).toBeVisible();
}

export async function api(request, token, path, method = 'GET', data) {
  const response = await request.fetch(`${base}/api/v1/${path}`, {
    method, headers: { Authorization: `Bearer ${token}` }, data,
  });
  expect(response.ok(), `${method} ${path}: ${response.status()} ${await response.text()}`).toBeTruthy();
  return response.json();
}

export async function agentToken(request, email = 'john@axleo.local') {
  const response = await request.post(`${base}/api/v1/auth/login`, {
    data: { email, password, device: 'Web E2E' },
  });
  expect(response.ok()).toBeTruthy();
  return (await response.json()).token;
}
