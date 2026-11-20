import { test, expect } from '@playwright/test';
import { login, agentToken, api } from './helpers.js';

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
