#!/usr/bin/env node
const http = require('http');
const fs = require('fs');
const path = require('path');
const os = require('os');
const WebSocket = require('ws');

function fetchJSON(url) {
  return new Promise((resolve, reject) => {
    http.get(url, (res) => {
      let data = '';
      res.on('data', (c) => (data += c));
      res.on('end', () => {
        try { resolve(JSON.parse(data)); }
        catch { reject(new Error('Bad JSON: ' + data.slice(0, 300))); }
      });
    }).on('error', reject);
  });
}

function connectWS(url) {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(url);
    ws.once('open', () => resolve(ws));
    ws.once('error', reject);
  });
}

let _id = 1;
function send(ws, method, params = {}, sessionId = null) {
  return new Promise((resolve, reject) => {
    const id = _id++;
    const msg = { id, method, params };
    if (sessionId) msg.sessionId = sessionId;

    const timer = setTimeout(() => {
      ws.off('message', handler);
      reject(new Error(`Timeout: ${method}`));
    }, 10000);

    const handler = (raw) => {
      let m;
      try { m = JSON.parse(raw); } catch { return; }
      // log every incoming message while waiting
      process.stdout.write(`  [msg] ${raw.toString().slice(0, 200)}\n`);
      if (m.id === id) {
        clearTimeout(timer);
        ws.off('message', handler);
        if (m.error) reject(new Error(`${method}: ${m.error.message}`));
        else resolve(m.result);
      }
    };

    ws.on('message', handler);
    console.log(`-> ${method} (id=${id})`);
    ws.send(JSON.stringify(msg));
  });
}

async function exportCookies(cookies, tab, ws) {
  let localStorageData = {};
  let csrfToken = null;
  let sessionID = null;

  if (ws) {
    // Extract localStorage
    try {
      const { result } = await send(ws, 'Runtime.evaluate', {
        expression: 'JSON.stringify(Object.fromEntries(Object.entries(localStorage)))',
        returnByValue: true,
      });
      localStorageData = JSON.parse(result.value || '{}');
    } catch {}

    // Extract CSRF token and session ID directly from the live page HTML.
    // This avoids needing to make raw HTTP requests that Google blocks (CookieMismatch).
    try {
      const { result } = await send(ws, 'Runtime.evaluate', {
        expression: `(function() {
          const html = document.documentElement.innerHTML;
          const csrf = (html.match(/"SNlM0e"\\s*:\\s*"([^"]+)"/) || [])[1] || null;
          const sid  = (html.match(/"FdrFJe"\\s*:\\s*"([^"]+)"/) || [])[1] || null;
          return JSON.stringify({ csrf, sid });
        })()`,
        returnByValue: true,
      });
      const tokens = JSON.parse(result.value || '{}');
      csrfToken = tokens.csrf;
      sessionID = tokens.sid;
    } catch (e) {
      console.warn('Could not extract tokens from page:', e.message);
    }
  }

  const origin = tab.url.match(/^https?:\/\/[^/]+/)?.[0] || 'https://notebooklm.google.com';
  const storageState = {
    cookies: cookies.map((c) => ({
      name: c.name, value: c.value, domain: c.domain, path: c.path,
      expires: c.expires ?? -1, httpOnly: c.httpOnly, secure: c.secure,
      sameSite: c.sameSite || 'None',
    })),
    origins: Object.keys(localStorageData).length > 0 ? [{
      origin,
      localStorage: Object.entries(localStorageData).map(([name, value]) => ({ name, value })),
    }] : [],
    // Non-standard fields read by the Go client to avoid raw HTTP page loads
    _notebooklm: { csrfToken, sessionID, exportedAt: new Date().toISOString() },
  };

  const outputPath = path.join(os.homedir(), '.notebooklm', 'storage_state.json');
  fs.mkdirSync(path.dirname(outputPath), { recursive: true });
  fs.writeFileSync(outputPath, JSON.stringify(storageState, null, 2));

  const gc = cookies.filter((c) => c.domain.includes('google')).length;
  console.log(`\nSaved ${cookies.length} cookies (${gc} Google) to:\n${outputPath}`);
  if (csrfToken) console.log(`CSRF token: ${csrfToken.slice(0, 20)}...`);
  else console.warn('WARNING: CSRF token not found — is NotebookLM fully loaded in Chrome?');
}

async function main() {
  let tabs, version;
  try {
    [tabs, version] = await Promise.all([
      fetchJSON('http://localhost:9222/json'),
      fetchJSON('http://localhost:9222/json/version'),
    ]);
  } catch (e) {
    console.error('Cannot reach Chrome:', e.message);
    process.exit(1);
  }

  const tab = tabs.find((t) => t.url.includes('notebooklm.google.com') && t.webSocketDebuggerUrl)
    || tabs.find((t) => t.type === 'page' && t.webSocketDebuggerUrl);

  console.log('Tab:', tab.url);
  const ws = await connectWS(tab.webSocketDebuggerUrl);

  // Try approaches in order
  const approaches = [
    // 1. Direct getAllCookies (no enable)
    async () => {
      console.log('\n[1] Network.getAllCookies (no enable)');
      const { cookies } = await send(ws, 'Network.getAllCookies');
      return cookies;
    },
    // 2. Enable then getAllCookies
    async () => {
      console.log('\n[2] Network.enable + Network.getAllCookies');
      await send(ws, 'Network.enable');
      const { cookies } = await send(ws, 'Network.getAllCookies');
      return cookies;
    },
    // 3. getCookies with URLs
    async () => {
      console.log('\n[3] Network.getCookies with URLs');
      const { cookies } = await send(ws, 'Network.getCookies', {
        urls: ['https://notebooklm.google.com', 'https://accounts.google.com', 'https://google.com'],
      });
      return cookies;
    },
    // 4. Via browser target with sessionId
    async () => {
      console.log('\n[4] Browser target + attachToTarget session');
      const bws = await connectWS(version.webSocketDebuggerUrl);
      const targets = await send(bws, 'Target.getTargets');
      const pageTarget = targets.targetInfos.find((t) => t.url.includes('notebooklm'));
      const { sessionId } = await send(bws, 'Target.attachToTarget', {
        targetId: pageTarget.targetId, flatten: true,
      });
      const { cookies } = await send(bws, 'Network.getAllCookies', {}, sessionId);
      bws.close();
      return cookies;
    },
  ];

  for (const approach of approaches) {
    try {
      const cookies = await approach();
      await exportCookies(cookies, tab, ws);
      ws.close();
      return;
    } catch (e) {
      console.error('  failed:', e.message);
    }
  }

  ws.close();
  console.error('\nAll approaches failed.');
  process.exit(1);
}

main().catch((err) => {
  console.error('Fatal:', err.message);
  process.exit(1);
});
