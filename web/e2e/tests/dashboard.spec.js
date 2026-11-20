import { test, expect } from '@playwright/test';
import { login, agentToken, api } from './helpers.js';

const labels = { visits: '上门拜访', captures: '新联系人', appraisals: '预约估价', ofi: '开放参观登记', owners: '新关联业主', listings: '新增房源' };

async function checkDashboard(page, metrics) {
  for (const [key, label] of Object.entries(labels)) {
    const card = page.locator('.kpis .kpi').filter({ has: page.locator('.lb', { hasText: label }) });
    await expect(card.locator('.v')).toHaveText(String(metrics.kpis[key].value));
    await expect(card).toContainText(`较上期 ${metrics.kpis[key].previous}`);
  }
  const coverage = Math.round(1000 * metrics.coverage.linked / Math.max(1, metrics.coverage.total)) / 10;
  await expect(page.locator('.kpis .kpi').filter({ hasText: '区域覆盖率' }).locator('.v')).toContainText(String(coverage));
}

test('dashboard and cockpit use scoped metrics; cockpit map renders without errors', async ({ page, request }) => {
  // Create a current-period event in John's other team to prove leader scoping.
  const john = await agentToken(request);
  await api(request, john, 'contacts', 'POST', { id: `e2e-dashboard-${Date.now()}`, first: 'Scope', last: 'Other team' });
  const admin = await agentToken(request, 'admin@axleo.local');
  const leader = await agentToken(request, 'mary@axleo.local');
  const all = await api(request, admin, 'admin/metrics?period=7&team=&agent=');
  const scoped = await api(request, leader, 'admin/metrics?period=7&team=&agent=');
  expect(all.kpis.captures.value).toBeGreaterThan(scoped.kpis.captures.value);

  await login(page);
  await checkDashboard(page, all);
  await page.locator('nav').getByRole('link', { name: '数据驾驶舱' }).click();
  for (const [key, label] of Object.entries(labels)) {
    const card = page.locator('.cockpit .kpi').filter({ has: page.locator('span', { hasText: label }) });
    await expect(card.locator('b')).toHaveText(String(all.kpis[key].value));
  }
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  page.on('console', message => { if (message.type() === 'error') errors.push(message.text()); });
  // External OSM tiles are not a dependency of this test (CI can be offline).
  await page.route('https://*.tile.openstreetmap.org/**', route => route.fulfill({
    contentType: 'image/png', body: Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=', 'base64'),
  }));
  await page.reload();
  await expect(page.locator('.cockpit .map-panel .leaflet-container')).toBeVisible();
  await expect(page.locator('.cockpit .map-panel .leaflet-tile-loaded').first()).toBeVisible();
  await expect(page.locator('.cockpit .map-panel .leaflet-interactive').first()).toBeAttached();
  expect(errors).toEqual([]);

  await login(page, 'mary@axleo.local');
  await checkDashboard(page, scoped);
  await expect(page.locator('.page-head label').filter({ hasText: '团队' })).toHaveCount(0);
  await page.locator('nav').getByRole('link', { name: '数据驾驶舱' }).click();
  await expect(page.locator('.cockpit .kpi').filter({ hasText: '新联系人' }).locator('b')).toHaveText(String(scoped.kpis.captures.value));
});
