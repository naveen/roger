// Renders the pictures in this folder from the concept page in public/.
//
//   cd public && python3 -m http.server 8765 --bind 127.0.0.1 &
//   cd docs/images && npm i --no-save playwright && node capture.mjs
//
// Uses the Google Chrome already on the machine (channel: 'chrome'), so no
// browser download is needed. Output lands next to this script.

import { chromium } from 'playwright';
import { writeFileSync, unlinkSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const out = (name) => resolve(here, name);
const base = process.env.ROGER_SITE ?? 'http://127.0.0.1:8765/';
const browser = await chromium.launch({ channel: 'chrome', args: ['--ignore-gpu-blocklist'] });

// Hide the viewer's toolbar and hint, and stop the turntable, inside a <three-d-stage>.
const quietStage = () => {
  const st = document.querySelector('three-d-stage');
  if (!st) return;
  st._controls.autoRotate = false;
  for (const s of st.shadowRoot.querySelectorAll('.toolbar, .note')) s.style.display = 'none';
};

// ---- the concept page, section by section ----
{
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 }, deviceScaleFactor: 2 });
  const page = await ctx.newPage();
  await page.goto(base + 'index.html', { waitUntil: 'networkidle' });
  await page.waitForTimeout(2500);
  // The floating "Made with Claude Design" badge is not part of the page design.
  await page.evaluate(() => {
    for (const el of document.querySelectorAll('*')) {
      if (el.children.length < 4 && /Made with Claude Design/.test(el.textContent || '') && el.getBoundingClientRect().height < 80) el.style.display = 'none';
    }
  });
  for (const f of page.frames()) if (f !== page.mainFrame()) await f.evaluate(quietStage).catch(() => {});
  await page.waitForTimeout(600);

  const secs = await page.evaluate(() => [...document.querySelectorAll('header, section, footer')].map((e) => {
    const r = e.getBoundingClientRect();
    return { top: Math.round(r.top + scrollY), h: Math.round(r.height) };
  }));
  const clip = (from, to) => ({ x: 0, y: secs[from].top, width: 1280, height: secs[to].top + secs[to].h - secs[from].top });
  await page.screenshot({ path: out('site-hero.png'), fullPage: true, clip: clip(0, 1) });
  await page.screenshot({ path: out('site-how-it-works.png'), fullPage: true, clip: clip(2, 2) });
  await page.screenshot({ path: out('site-heard-on-the-air.png'), fullPage: true, clip: clip(3, 4) });
  await page.screenshot({ path: out('site-for-grown-ups.png'), fullPage: true, clip: clip(5, 5) });
  await ctx.close();

  const ctx1 = await browser.newContext({ viewport: { width: 1280, height: 800 }, deviceScaleFactor: 1 });
  const p1 = await ctx1.newPage();
  await p1.goto(base + 'index.html', { waitUntil: 'networkidle' });
  await p1.waitForTimeout(2500);
  await p1.evaluate(() => {
    for (const el of document.querySelectorAll('*')) {
      if (el.children.length < 4 && /Made with Claude Design/.test(el.textContent || '') && el.getBoundingClientRect().height < 80) el.style.display = 'none';
    }
  });
  for (const f of p1.frames()) if (f !== p1.mainFrame()) await f.evaluate(quietStage).catch(() => {});
  await p1.waitForTimeout(600);
  await p1.screenshot({ path: out('site-full.png'), fullPage: true });
  await ctx1.close();
}

// ---- the concept model from fixed angles ----
const VW = 560, VH = 760;
{
  const ctx = await browser.newContext({ viewport: { width: VW, height: VH }, deviceScaleFactor: 2 });
  const page = await ctx.newPage();
  await page.goto(base + 'model.html', { waitUntil: 'networkidle' });
  await page.waitForTimeout(2000);
  await page.evaluate(quietStage);
  await page.evaluate(() => {
    const st = document.querySelector('three-d-stage');
    st._ground.visible = false; // no cast shadow in the turnaround
    window.__base = st._camera.position.distanceTo(st._controls.target);
  });
  // name, azimuth in degrees (0 faces the front), elevation in degrees, distance factor
  const views = [
    ['front', 0, 6, 0.78],
    ['three-quarter', 38, 16, 0.78],
    ['side', 90, 6, 0.78],
    ['back', 180, 10, 0.78],
    ['top', 30, 58, 0.78],
  ];
  for (const [name, az, el, zoom] of views) {
    await page.evaluate(([az, el, zoom]) => {
      const st = document.querySelector('three-d-stage');
      const THREE = st._THREE, cam = st._camera, ctl = st._controls;
      const t = ctl.target.clone();
      const dist = window.__base * zoom;
      const a = THREE.MathUtils.degToRad(az) + st._object.rotation.y, e = THREE.MathUtils.degToRad(el);
      cam.position.set(t.x + Math.sin(a) * Math.cos(e) * dist, t.y + Math.sin(e) * dist, t.z + Math.cos(a) * Math.cos(e) * dist);
      cam.lookAt(t);
      ctl.update();
    }, [az, el, zoom]);
    await page.waitForTimeout(400);
    await page.screenshot({ path: out(`model-${name}.png`) });
  }
  await ctx.close();
}

// ---- four views side by side ----
{
  const strip = ['front', 'three-quarter', 'side', 'back'];
  const html = `<!doctype html><body style="margin:0;background:#F3ECDF"><div style="display:flex">${strip
    .map((n) => `<img src="file://${out('model-' + n + '.png')}" style="width:${VW}px;height:${VH}px;display:block">`)
    .join('')}</div></body>`;
  const tmp = out('.strip.html');
  writeFileSync(tmp, html);
  const ctx = await browser.newContext({ viewport: { width: VW * strip.length, height: VH }, deviceScaleFactor: 2 });
  const page = await ctx.newPage();
  await page.goto('file://' + tmp, { waitUntil: 'load' });
  await page.waitForTimeout(500);
  await page.screenshot({ path: out('model-turnaround.png') });
  await ctx.close();
  unlinkSync(tmp);
}
await browser.close();
