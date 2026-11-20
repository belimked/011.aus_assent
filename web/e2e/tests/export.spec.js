import { test, expect } from '@playwright/test';
import { login, agentToken, api, base } from './helpers.js';

// RFC 4180 quoting (including embedded commas/newlines and doubled quotes).
function parseCSV(text) {
  const rows = [], row = [];
  let cell = '', quoted = false;
  for (let i = 0; i < text.length; i++) {
    const char = text[i];
    if (char === '"') {
      if (quoted && text[i + 1] === '"') { cell += '"'; i++; }
      else quoted = !quoted;
    } else if (char === ',' && !quoted) { row.push(cell); cell = ''; }
    else if ((char === '\n' || char === '\r') && !quoted) {
      if (char === '\r' && text[i + 1] === '\n') i++;
      row.push(cell); rows.push([...row]); row.length = 0; cell = '';
    } else cell += char;
  }
  if (cell || row.length) { row.push(cell); rows.push(row); }
  return rows;
}

test('contact flagged DNC through the phone sync API is excluded from export', async ({ request }) => {
  const phone = await agentToken(request, 'mary@axleo.local');
  const admin = await agentToken(request, 'admin@axleo.local');
  const id = `phone-dnc-${Date.now()}`;
  const suburb = `PhoneDNC${Date.now()}`;
  const push = await api(request, phone, 'sync/push', 'POST', { ops: [{
    opId: `${id}-op`, kind: 'contacts', op: 'upsert', id,
    data: { first: 'Phone', last: 'Private', flags: { dnc: true }, consent: { phone: true } },
  }] });
  expect(push.results[0].status).toBe('ok');
  await api(request, admin, 'properties', 'POST', {
    id: `${id}-property`, no: '42', street: 'Sync DNC Street', suburb, ownerIds: [id],
  });
  const response = await request.post(`${base}/api/v1/admin/export`, {
    headers: { Authorization: `Bearer ${admin}` },
    data: { fields: ['address'], suburb, reason: 'Phone sync privacy regression', acknowledge: true, excludeDNC: true },
  });
  expect(response.ok()).toBe(true);
  expect(await response.text()).not.toContain('Sync DNC Street');
});

test('UI CSV download excludes DNC and do-not-knock test fixtures and is audited', async ({ page, request }) => {
  const admin = await agentToken(request, 'admin@axleo.local');
  const leader = await agentToken(request, 'mary@axleo.local');
  const prefix = `E2E${Date.now()}`;
  const suburb = `E2E${Date.now()}Suburb`;
  const contact = await api(request, admin, 'contacts', 'POST', {
    id: `${prefix}-dnc-contact`, first: 'Private', last: prefix, flags: { dnc: true },
    consent: { phone: 1, email: 1 }, mobile: '0499000000',
  });
  // Link the seeded DNC contact to an otherwise exportable parcel as a second fixture.
  const seededDNC = await api(request, admin, 'contacts/c4');
  expect(seededDNC.flags.dnc).toBe(true);
  const normal = `${prefix} Allowed Street`;
  const dnc = `${prefix} DNC Street`;
  const seededDNCAddress = `${prefix} Seeded DNC Street`;
  const excluded = `${prefix} Test Excluded Street`; 
  await api(request, admin, 'properties', 'POST', { id: `${prefix}-ok`, no: '1', street: normal, suburb });
  await api(request, admin, 'properties', 'POST', {
    id: `${prefix}-dnc`, no: '2', street: dnc, suburb, ownerIds: [contact.id], doNotKnock: true,
  });
  await api(request, admin, 'properties', 'POST', {
    id: `${prefix}-seed-dnc`, no: '4', street: seededDNCAddress, suburb, ownerIds: [seededDNC.id],
  });
  await api(request, admin, 'properties', 'POST', {
    id: `${prefix}-test`, no: '3', street: excluded, suburb, doNotKnock: true,
  });
  const forbidden = await request.post(`${base}/api/v1/admin/export`, {
    headers: { Authorization: `Bearer ${leader}` },
    data: { fields: ['address'], reason: 'forbidden', acknowledge: true },
  });
  expect(forbidden.status()).toBe(403);

  await login(page);
  await page.locator('nav').getByRole('link', { name: '数据导出' }).click();
  // Filter to the isolated fixture suburb, so the downloaded rows are deterministic.
  await page.locator('.fields label').filter({ hasText: '区域' }).locator('.el-select__wrapper').click();
  await page.getByRole('option', { name: suburb }).click();
  await page.getByRole('button', { name: '下一步' }).click();
  await page.locator('label.el-checkbox').filter({ hasText: '联系人姓名' }).click();
  await page.getByRole('button', { name: '下一步' }).click();
  const reason = `E2E CSV ${prefix}`;
  await page.locator('.fields textarea').fill(reason);
  await page.locator('label.el-checkbox').filter({ hasText: '确认仅用于已获授权的业务用途' }).click();
  const download = page.waitForEvent('download');
  await page.getByRole('button', { name: '下载 CSV' }).click();
  const file = await download;
  expect(file.suggestedFilename()).toBe('properties.csv');
  const rows = parseCSV((await file.createReadStream().then(async stream => {
    const chunks = []; for await (const chunk of stream) chunks.push(chunk);
    return Buffer.concat(chunks).toString('utf8');
  })).replace(/^\uFEFF/, ''));
  expect(rows[0]).toEqual(['address', 'suburb', 'contactName']);
  expect(rows.slice(1)).toEqual([[`1 ${normal}`, suburb, '']]);
  expect(JSON.stringify(rows)).not.toContain(dnc);
  expect(JSON.stringify(rows)).not.toContain(seededDNCAddress);
  expect(JSON.stringify(rows)).not.toContain(excluded);
  expect(JSON.stringify(rows)).not.toContain(`Private ${prefix}`);
  const logs = await api(request, admin, 'admin/exports');
  expect(logs.some(entry => {
    const diff = JSON.parse(entry.diff);
    return diff.reason === reason && diff.rows === 1 && diff.excludeDNC === true;
  })).toBe(true);
  await expect(page.locator('.records .record').first()).toContainText(reason);
});
