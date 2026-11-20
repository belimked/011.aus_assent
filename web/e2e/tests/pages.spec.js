import { test, expect } from '@playwright/test';
import { login, agentToken, api } from './helpers.js';

test('map shows cadastral parcels and opens the parcel drawer without external tiles', async ({ page, request }) => {
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.route(/tile\.openstreetmap\.org/, route => route.abort());
  const token = await agentToken(request, 'admin@axleo.local');
  const properties = await api(request, token, 'admin/properties?limit=5');
  const parcel = properties.find(p => p.lotplan) || properties[0];
  await login(page);
  await page.locator('nav').getByRole('link', { name: '区域地图' }).click();
  await expect(page.locator('.leaflet-container')).toBeVisible();
  await expect(page.locator('.leaflet-interactive').first()).toBeVisible();
  await page.getByPlaceholder('地址或地块编号').fill('no-such-parcel-e2e');
  await expect(page.locator('.li')).toHaveCount(0);
  await page.getByPlaceholder('地址或地块编号').clear();
  await page.locator('.chip').last().click();
  await expect(page.locator('.chip.on')).toHaveCount(1);
  await page.goto(`/admin/#/map?parcel=${encodeURIComponent(parcel.id)}`);
  await expect(page.locator('.pdrawer')).toBeVisible();
  await expect(page.locator('.pdrawer')).toContainText(parcel.address);
  await expect(page.locator('.pdrawer')).toContainText(parcel.suburb);
  expect(errors).toEqual([]);
});

test('property list searches, filters, switches view and opens details; leader lacks licensed columns', async ({ page, request }) => {
  const admin = await agentToken(request, 'admin@axleo.local');
  const id = `web-property-${Date.now()}`;
  const street = `${id} Street`;
  const created = await api(request, admin, 'properties', 'POST', { id, no: '42', street, suburb: 'Sunnybank' });
  await login(page);
  await page.locator('nav').getByRole('link', { name: '房产列表' }).click();
  await page.getByPlaceholder('地址或地块编号').fill(street);
  await page.getByPlaceholder('地址或地块编号').press('Enter');
  await expect(page.locator('.el-table__body tr').filter({ hasText: street })).toHaveCount(1);
  await page.locator('.el-table__body tr').filter({ hasText: street }).click();
  await expect(page.locator('.pdrawer')).toContainText(`42 ${street}`);
  await expect(page.locator('.pdrawer')).toContainText('Sunnybank');
  expect((await api(request, admin, `admin/properties?q=${encodeURIComponent(id)}`)).some(p => p.id === created.id)).toBe(true);
  await page.locator('.pdrawer button').first().click();
  await expect(page.locator('.pdrawer')).toBeHidden();
  await page.getByText('卡片', { exact: true }).click();
  await expect(page.locator('.pc').filter({ hasText: street })).toBeVisible();
  const leader = await agentToken(request, 'mary@axleo.local');
  const scoped = await api(request, leader, `admin/properties?q=${encodeURIComponent(id)}`);
  expect(scoped.every(p => !('ownerNames' in p) && !('lastSalePrice' in p))).toBe(true);
});

test('property filters and cursor pagination', async ({ page }) => {
  await login(page);
  await page.locator('nav').getByRole('link', { name: '房产列表' }).click();
  await expect(page.locator('.pg')).toContainText('第 1 页');
  await page.getByRole('button', { name: '下一页' }).click();
  await expect(page.locator('.pg')).toContainText('第 2 页');
  await page.getByRole('button', { name: '上一页' }).click();
  await expect(page.locator('.pg')).toContainText('第 1 页');
  await page.locator('.filter-label').filter({ hasText: '登记状态' }).locator('.el-select').click();
  await page.getByRole('option', { name: '未登记' }).click();
  await expect(page.locator('.pg')).toContainText('第 1 页');
  await expect(page.locator('.el-table__body tr').first()).toBeVisible();
});

test('open-home visitor is listed and admin follow-up persists', async ({ page, request }) => {
  const admin = await agentToken(request, 'admin@axleo.local');
  const users = await api(request, admin, 'admin/users');
  const listings = await api(request, admin, 'admin/listings');
  const cases = [
    ['Natalie Wong', 'john@axleo.local', 'agent'],
    ['Owen Chen', 'mary@axleo.local', 'leader'], // E2E bootstrap promotes Mary without reassigning her listings.
  ];
  const visits = [];
  for (const [displayName, email, role] of cases) {
    const listing = listings.find(l => l.ofi?.length && l.agent === displayName);
    const owner = users.find(u => u.email === email);
    expect(listing).toBeTruthy();
    expect(owner.role).toBe(role);
    expect(listing.ownerId).toBe(owner.id);
    const id = `web-ofi-${Date.now()}-${visits.length}`;
    const agent = await agentToken(request, email);
    const contact = await api(request, agent, 'contacts', 'POST', { id: `${id}-contact`, first: 'Visitor', last: id });
    const attendee = await api(request, agent, 'ofi_attendees', 'POST', { id, listingId: listing.id, eventIndex: 0, contactId: contact.id });
    visits.push({ listing, owner, attendee, id });
  }
  await login(page);
  await page.locator('nav').getByRole('link', { name: '开放参观' }).click();
  for (const { listing, owner, attendee, id } of visits) {
    const event = page.locator('.oc').filter({ hasText: listing.propertyAddress }).first();
    await expect(event).toBeVisible();
    await event.click();
    const visitor = page.locator('.vi').filter({ hasText: `Visitor ${id}` });
    await expect(visitor).toBeVisible();
    await visitor.getByRole('button', { name: '跟进' }).click();
    await expect(visitor).toContainText('已跟进');
    const saved = (await api(request, admin, 'admin/ofi_attendees')).find(a => a.id === attendee.id);
    expect(saved.follow).toBe(true);
    const task = (await api(request, admin, 'admin/todos')).find(todo => todo.id === `ofi-follow-${id}`);
    expect(task?.assigneeId).toBe(owner.id);
  }
});
