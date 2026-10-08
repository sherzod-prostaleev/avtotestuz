// Renders banner.html to frontend/public/bot/start-banner{,-ru}.jpg (the
// bot's /start photo). Needs the frontend's Playwright and the Baloo 2 /
// Manrope fonts installed locally. Run from frontend/: node scripts/bot-banner/render.mjs
import { createRequire } from "node:module";
import { pathToFileURL, fileURLToPath } from "node:url";
import { statSync } from "node:fs";
import path from "node:path";

const here = path.dirname(fileURLToPath(import.meta.url));
const frontend = path.resolve(here, "../..");
const { chromium } = createRequire(path.join(frontend, "package.json"))("playwright");

const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1280, height: 720 }, deviceScaleFactor: 1 });
for (const lang of ["uz", "ru"]) {
  await page.goto(pathToFileURL(path.join(here, "banner.html")).href + "?lang=" + lang);
  await page.evaluate(() => document.fonts.ready);
  const file = path.join(frontend, "public/bot", lang === "uz" ? "start-banner.jpg" : "start-banner-ru.jpg");
  // Telegram recompresses anyway; q85 keeps each file near 100 KB.
  await page.screenshot({ path: file, type: "jpeg", quality: 85 });
  console.log(file, Math.round(statSync(file).size / 1024) + " KB");
}
await browser.close();
