// Service Worker for the Periwiki WASM demo.
// Intercepts fetch requests and routes them through Go HTTP handlers
// running in WebAssembly.

// importScripts can only be called during initial evaluation or install —
// NOT in message/fetch handlers. Load wasm_exec.js up front.
importScripts('wasm_exec.js');
console.log('[sw] wasm_exec.js loaded');

let wasmReady = false;
let wasmLoading = false;
let wasmError = null;
let cookieJar = {}; // name -> cookie string (key=value;...)

self.addEventListener('install', (event) => {
    console.log('[sw] install');
    event.waitUntil(self.skipWaiting());
});

self.addEventListener('activate', (event) => {
    console.log('[sw] activate');
    event.waitUntil(self.clients.claim());
    // Start loading WASM immediately — don't wait for a message
    startWasm();
});

// Handle messages from clients (status checks and fallback init)
self.addEventListener('message', (event) => {
    console.log('[sw] message:', event.data);
    if (event.data && (event.data.type === 'init' || event.data.type === 'status')) {
        if (wasmReady) {
            event.source.postMessage({ type: 'wasm-ready' });
        } else if (wasmError) {
            event.source.postMessage({ type: 'wasm-error', error: wasmError });
        } else {
            // Still loading — start if not already started, reply when done
            startWasm();
            waitForWasm().then(msg => event.source.postMessage(msg));
        }
    }
});

function startWasm() {
    if (wasmLoading || wasmReady) return;
    wasmLoading = true;
    loadWasm().then(() => {
        wasmReady = true;
        console.log('[sw] WASM ready, broadcasting');
        broadcast({ type: 'wasm-ready' });
    }).catch(err => {
        wasmLoading = false;
        wasmError = err.message || String(err);
        console.error('[sw] WASM load failed:', wasmError);
        broadcast({ type: 'wasm-error', error: wasmError });
    });
}

function waitForWasm() {
    return new Promise(resolve => {
        const check = setInterval(() => {
            if (wasmReady) { clearInterval(check); resolve({ type: 'wasm-ready' }); }
            else if (wasmError) { clearInterval(check); resolve({ type: 'wasm-error', error: wasmError }); }
        }, 100);
    });
}

async function loadWasm() {
    console.log('[sw] loadWasm: creating Go instance');
    const go = new Go();

    // Set up the ready callback before instantiating
    const readyPromise = new Promise((resolve) => {
        self.__periwikiReady = resolve;
    });

    console.log('[sw] loadWasm: fetching periwiki.wasm');
    const response = await fetch('periwiki.wasm');
    if (!response.ok) throw new Error('Failed to fetch WASM: ' + response.status);

    console.log('[sw] loadWasm: instantiating WASM (' + response.headers.get('content-length') + ' bytes)');
    const result = await WebAssembly.instantiateStreaming(response, go.importObject);

    console.log('[sw] loadWasm: running Go main()');
    go.run(result.instance); // don't await — it blocks forever

    console.log('[sw] loadWasm: waiting for Go ready signal');
    await readyPromise;
    console.log('[sw] loadWasm: Go ready');
}

function broadcast(msg) {
    self.clients.matchAll().then(clients => {
        console.log('[sw] broadcasting to', clients.length, 'clients:', msg.type);
        clients.forEach(c => c.postMessage(msg));
    });
}

// Parse Set-Cookie headers and store in our cookie jar
function storeCookies(setCookies) {
    for (const header of setCookies) {
        const eqIdx = header.indexOf('=');
        if (eqIdx < 0) continue;
        const name = header.substring(0, eqIdx).trim();

        if (/Max-Age=0/i.test(header)) {
            delete cookieJar[name];
            continue;
        }

        const semiIdx = header.indexOf(';');
        const nameValue = semiIdx >= 0 ? header.substring(0, semiIdx) : header;
        cookieJar[name] = nameValue.trim();
    }
}

function getCookieHeader() {
    const parts = Object.values(cookieJar);
    return parts.length > 0 ? parts.join('; ') : '';
}

self.addEventListener('fetch', (event) => {
    const url = new URL(event.request.url);

    // Let boot assets pass through to the real server
    if (['/index.html', '/sw.js', '/wasm_exec.js', '/periwiki.wasm'].includes(url.pathname)) {
        return;
    }

    if (!wasmReady) {
        if (event.request.mode === 'navigate') {
            event.respondWith(Response.redirect('/', 302));
        } else {
            event.respondWith(new Response('', { status: 503 }));
        }
        return;
    }

    event.respondWith(handleFetch(event.request));
});

async function handleFetch(request) {
    const url = new URL(request.url);

    const headers = {};
    for (const [key, value] of request.headers.entries()) {
        headers[key] = value;
    }

    const cookieStr = getCookieHeader();
    if (cookieStr) {
        headers['Cookie'] = cookieStr;
    }

    let body = null;
    if (request.method === 'POST') {
        body = await request.text();
    }

    let result;
    try {
        result = self.__periwikiHandleRequest({
            method: request.method,
            url: url.pathname + url.search,
            headers: headers,
            body: body,
        });
    } catch (err) {
        console.error('[sw] WASM handler error:', err);
        return new Response('Internal error: ' + err.message, { status: 500 });
    }

    const setCookies = [];
    for (let i = 0; i < result.setCookies.length; i++) {
        setCookies.push(result.setCookies[i]);
    }
    storeCookies(setCookies);

    const respHeaders = new Headers();
    const headerKeys = Object.keys(result.headers);
    for (const key of headerKeys) {
        if (key.toLowerCase() === 'set-cookie') continue;
        respHeaders.set(key, result.headers[key]);
    }

    if (result.status >= 300 && result.status < 400) {
        const location = result.headers['Location'] || result.headers['location'];
        if (location) {
            return Response.redirect(new URL(location, request.url).href, result.status);
        }
    }

    return new Response(result.body, {
        status: result.status,
        headers: respHeaders,
    });
}
