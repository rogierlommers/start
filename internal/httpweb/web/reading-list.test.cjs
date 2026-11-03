const { test } = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')

const html = fs.readFileSync(`${__dirname}/index.html`, 'utf8')
const renderSource = html.slice(
  html.indexOf('async function loadReadingListItems()'),
  html.indexOf('function initReadingListBookmarklet()'),
)
const escapeSource = html.slice(html.indexOf('function escHtml(s)'), html.indexOf('// ── Tabs'))

async function render(items) {
  const list = { innerHTML: '' }
  const load = new Function('document', 'apiFetch', 'fmtDateTime',
    `${escapeSource}; ${renderSource}; return loadReadingListItems()`)
  await load({ getElementById: () => list }, async () => items, value => value)
  return list.innerHTML
}

test('reading list renders request diagnostics without injecting HTML', async () => {
  const output = await render([{
    url: 'https://example.com/', created_at: '2026-11-03', title: 'Example',
    client_ip: '192.0.2.10', user_agent: '<img src=x onerror=alert(1)>',
    referer: 'https://example.net',
  }])
  assert.match(output, /Request details/)
  assert.match(output, /192\.0\.2\.10/)
  assert.match(output, /&lt;img src=x onerror=alert\(1\)&gt;/)
  assert.doesNotMatch(output, /<img src=x/)
  assert.match(output, /https:\/\/example\.net/)
})

test('legacy reading list entries have an explicit no-details state', async () => {
  const output = await render([{ url: 'https://example.com/', created_at: '2026-10-01', title: '' }])
  assert.match(output, /No request details recorded/)
})

test('missing individual request fields are labelled', async () => {
  const output = await render([{ url: 'https://example.com/', created_at: '2026-11-03', client_ip: '192.0.2.10' }])
  assert.match(output, /IP address:<\/dt> <dd[^>]*>192\.0\.2\.10/)
  assert.match(output, /User agent:<\/dt> <dd[^>]*>Not provided/)
  assert.match(output, /Referrer:<\/dt> <dd[^>]*>Not provided/)
})
