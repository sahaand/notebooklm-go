#!/usr/bin/env node
// Checks whether the exported cookies actually authenticate with NotebookLM.
const https = require('https');
const fs = require('fs');
const os = require('os');
const zlib = require('zlib');

const storagePath = process.argv[2] || `${os.homedir()}/.notebooklm/storage_state.json`;
const data = JSON.parse(fs.readFileSync(storagePath, 'utf8'));

const cookies = data.cookies.filter((c) =>
  c.domain.includes('google')
);

const cookieHeader = cookies.map((c) => `${c.name}=${c.value}`).join('; ');

console.log(`Loaded ${cookies.length} Google cookies`);
console.log('Key cookies present:');
const keyNames = ['SID','HSID','SSID','APISID','SAPISID','__Secure-1PSID','__Secure-3PSID','NID'];
for (const k of keyNames) {
  const found = cookies.find(c => c.name === k);
  console.log(`  ${k}: ${found ? 'OK (domain=' + found.domain + ')' : 'MISSING'}`);
}

console.log('\nMaking request to https://notebooklm.google.com/ ...');

const options = {
  hostname: 'notebooklm.google.com',
  path: '/',
  method: 'GET',
  headers: {
    'Cookie': cookieHeader,
    'User-Agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36',
    'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8',
    'Accept-Language': 'en-US,en;q=0.9',
  },
};

function request(opts, redirectCount = 0) {
  if (redirectCount > 5) {
    console.error('Too many redirects');
    return;
  }

  https.get(opts, (res) => {
    console.log(`Status: ${res.statusCode}`);
    console.log(`Final URL: ${opts.hostname}${opts.path}`);

    if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
      const loc = res.headers.location;
      console.log(`Redirect -> ${loc}`);
      const url = new URL(loc.startsWith('http') ? loc : `https://${opts.hostname}${loc}`);
      return request({ hostname: url.hostname, path: url.pathname + url.search, method: 'GET',
        headers: opts.headers }, redirectCount + 1);
    }

    const chunks = [];
    res.on('data', (c) => chunks.push(c));
    res.on('end', () => {
      const raw = Buffer.concat(chunks);
      let body;
      if (res.headers['content-encoding'] === 'gzip') {
        body = zlib.gunzipSync(raw).toString('utf8');
      } else {
        body = raw.toString('utf8');
      }

      if (body.includes('accounts.google.com/signin') || body.includes('accounts.google.com/v3/signin')) {
        console.log('\nResult: REDIRECTED TO LOGIN — cookies are invalid or expired');
      } else if (body.includes('SNlM0e') || body.includes('NotebookLM')) {
        console.log('\nResult: SUCCESS — cookies are valid');
        const m = body.match(/"SNlM0e"\s*:\s*"([^"]+)"/);
        if (m) console.log('CSRF token found:', m[1].slice(0, 20) + '...');
      } else {
        console.log('\nResult: UNEXPECTED RESPONSE');
        console.log('Body preview:', body.slice(0, 500));
      }
    });
  }).on('error', (e) => console.error('Request error:', e.message));
}

request(options);
