import { test, expect } from '@playwright/test';
import { login, agentToken, api } from './helpers.js';

test('market address review links a candidate and ignores an unrelated event', async ({ page, request }) => {
  const admin = await agentToken(request, 'admin@axleo.local');
  const id = `market-web-${Date.now()}`;
  const street = `${id} Road`;
  await api(request, admin, 'properties', 'POST', { id: `${id}-parcel`, no: '9', street, suburb: 'Sunnybank' });
  const data = `Address,Suburb,Kind,Record ID\n9 ${street} Rear,Sunnybank,listing,${id}-link\n999 ${id} Missing,Sunnybank,listing,${id}-ignore\n`;
  const upload = await request.post('http://127.0.0.1:8083/api/v1/admin/market/imports', {
    headers: { Authorization: `Bearer ${admin}` },
    multipart: { file: { name: `${id}.csv`, mimeType: 'text/csv', buffer: Buffer.from(data) } },
  });
  expect(upload.status(), await upload.text()).toBe(201);
  const pending = await api(request, admin, 'admin/market-review');
  const link = pending.find(e => e.address === `9 ${street} Rear`);
  const ignore = pending.find(e => e.address === `999 ${id} Missing`);
  expect(link && ignore).toBeTruthy();
  await login(page);
  await page.locator('nav').getByRole('link', { name: '审核队列' }).click();
  await page.locator('.ri').filter({ hasText: `9 ${street} Rear` }).click();
  await expect(page.locator('.candidate-row').filter({ hasText: street })).toBeVisible();
  await page.getByRole('button', { name: /确认关联/ }).click();
  await expect(page.locator('.ri').filter({ hasText: `9 ${street} Rear` })).toHaveCount(0);
  await page.locator('.ri').filter({ hasText: `999 ${id} Missing` }).click();
  await page.getByRole('button', { name: '不是同一地址' }).click();
  await expect(page.locator('.ri').filter({ hasText: `999 ${id} Missing` })).toHaveCount(0);
  const after = await api(request, admin, 'admin/market-review');
  expect(after.some(e => e.id === link.id || e.id === ignore.id)).toBe(false);
  for (const [event, action] of [[link, 'link'], [ignore, 'ignore']]) {
    const retry = await request.post(`http://127.0.0.1:8083/api/v1/admin/market-review/${event.id}/${action}`, {
      headers: { Authorization: `Bearer ${admin}` }, data: { propertyId: `${id}-parcel` },
    });
    expect(retry.status()).toBe(409); // Already reviewed, including after a fresh API request.
  }
  const property = (await api(request, admin, `admin/properties?q=${id}-parcel`)).find(p => p.id === `${id}-parcel`);
  expect(property.registered).toBe(true);
});

test('voice capture review creates a persisted reminder for the agent', async ({ page, request }) => {
  const agent = await agentToken(request, 'john@axleo.local');
  const admin = await agentToken(request, 'admin@axleo.local');
  const id = `voice-review-${Date.now()}`;
  await api(request, agent, 'captures', 'POST', { id, text: `Voice capture ${id}`, extracted: { first: 'Voice' } });
  await login(page);
  await page.locator('nav').getByRole('link', { name: '审核队列' }).click();
  await page.getByRole('tab', { name: /AI 采集/ }).click();
  await page.locator('.ri').filter({ hasText: id }).click();
  await expect(page.locator('.rd .transcript')).toContainText(id);
  await page.locator('.rd').getByRole('button', { name: /提醒经纪人/ }).click();
  await expect.poll(async () => (await api(request, admin, 'admin/todos')).some(todo => todo.id === `capture-remind-${id}`)).toBe(true);
});

test('approve and reject separate requests in the review queue', async ({ page, request }) => {
  const token = await agentToken(request);
  const id = `web-review-${Date.now()}`;
  const first = `${id}-first`, second = `${id}-second`;
  await api(request, token, 'contacts', 'POST', { id: first, first: 'Review', last: 'First' });
  await api(request, token, 'contacts', 'POST', { id: second, first: 'Review', last: 'Second' });
  const approved = await api(request, token, 'approvals', 'POST', { kind: 'merge', primaryId: first, secondaryId: second });
  await api(request, token, 'approvals', 'POST', { kind: 'merge', primaryId: second, secondaryId: first });

  await login(page);
  await page.locator('nav').getByRole('link', { name: '审核队列' }).click();
  await page.getByRole('tab', { name: /审批/ }).click();
  const pending = page.getByRole('tab', { name: /审批/ });
  await expect(page.locator('.approval-row')).toHaveCount(2);
  const approveRow = page.locator('.approval-row').filter({ hasText: 'Review First' });
  const rejectRow = page.locator('.approval-row').filter({ hasText: 'Review Second' });
  await approveRow.getByRole('button', { name: '通过' }).click();
  await expect(page.locator('.approval-row')).toHaveCount(1);
  await rejectRow.getByRole('button', { name: '拒绝' }).click();
  await expect(page.locator('.approval-row')).toHaveCount(0);
  await expect(pending).toContainText('(0)');
  // The status filter confirms both decisions persist, not just that rows disappeared.
  await page.locator('.filter-label .el-select').click();
  await page.getByRole('option', { name: '全部' }).click();
  await expect(page.locator('.approval-row').filter({ hasText: 'Review First' })).toContainText('已批准');
  await expect(page.locator('.approval-row').filter({ hasText: 'Review Second' })).toContainText('已拒绝');
});
