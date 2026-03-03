import { chromium } from "playwright";
import * as path from "path";
import * as fs from "fs";

const OUTPUT_DIR = path.join(__dirname, "output");
const BASE_URL = "http://localhost:3080";

async function main() {
  fs.mkdirSync(OUTPUT_DIR, { recursive: true });

  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    viewport: { width: 1280, height: 720 },
    colorScheme: "dark",
    recordVideo: {
      dir: OUTPUT_DIR,
      size: { width: 1280, height: 720 },
    },
  });

  const page = await context.newPage();

  // 1. Open the app
  console.log("Opening app...");
  await page.goto(BASE_URL, { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(1000);

  // 2. Wait for SSE connection
  console.log("Waiting for SSE connection...");
  await page.locator("#sse-status.connected").waitFor({ timeout: 10000 });
  await page.waitForTimeout(500);

  // 3. Wait for trace rows to appear
  console.log("Waiting for trace rows...");
  await page.locator("tr[data-trace-id]").first().waitFor({ timeout: 15000 });
  await page.waitForTimeout(1000);

  // 4. Find the POST trace with the most spans and click it
  //    Table columns: Method, Path, Status, Duration, Spans(idx=4), Types
  console.log("Clicking trace row...");
  const bestRowIndex = await page.evaluate(() => {
    const rows = Array.from(document.querySelectorAll("tr[data-trace-id]"));
    let bestIdx = -1;
    let bestSpans = -1;
    for (let i = 0; i < rows.length; i++) {
      const isPost = rows[i].classList.contains("method-post");
      const spanCount = parseInt(rows[i].querySelectorAll("td")[4]?.textContent || "0", 10);
      // Prefer POST, then most spans
      if (
        bestIdx === -1 ||
        (isPost && !rows[bestIdx].classList.contains("method-post")) ||
        (isPost === rows[bestIdx].classList.contains("method-post") && spanCount > bestSpans)
      ) {
        bestIdx = i;
        bestSpans = spanCount;
      }
    }
    return bestIdx;
  });
  const targetRow = page.locator("tr[data-trace-id]").nth(bestRowIndex >= 0 ? bestRowIndex : 0);
  await targetRow.scrollIntoViewIfNeeded();
  await targetRow.click();

  // 5. Wait for detail panel to be visible (showDetail fetches from API)
  console.log("Waiting for detail panel...");
  await page.locator("#detail-panel:not(.hidden)").waitFor({ timeout: 10000 });
  await page.waitForTimeout(2000);

  // 6. Click a SQL span in the span tree
  console.log("Clicking SQL span...");
  const sqlSpan = page.locator(".span-tree-item:has(.badge-sql)").first();
  if ((await sqlSpan.count()) > 0) {
    await sqlSpan.scrollIntoViewIfNeeded();
    await sqlSpan.click();
    await page.waitForTimeout(1500);

    // 7. Click EXPLAIN button and wait for result
    console.log("Clicking EXPLAIN...");
    const explainBtn = page.locator("#explain-btn");
    if ((await explainBtn.count()) > 0) {
      await explainBtn.scrollIntoViewIfNeeded();
      await explainBtn.click();
      await page.locator(".explain-result").waitFor({ timeout: 10000 });
      await page.waitForTimeout(2500);
    }
  }

  // 8. Close the detail panel
  console.log("Closing detail panel...");
  await page.locator("#close-btn").click();
  await page.locator("#detail-panel.hidden").waitFor({ state: "attached", timeout: 5000 });
  await page.waitForTimeout(1500);

  // Finish recording — must close context to flush video
  console.log("Finishing recording...");
  await page.close();
  await context.close();
  await browser.close();

  // Rename the video file to demo.webm
  const webmFiles = fs
    .readdirSync(OUTPUT_DIR)
    .filter((f) => f.endsWith(".webm"))
    .map((f) => ({
      name: f,
      time: fs.statSync(path.join(OUTPUT_DIR, f)).mtimeMs,
    }))
    .sort((a, b) => a.time - b.time);
  if (webmFiles.length > 0) {
    const src = path.join(OUTPUT_DIR, webmFiles[webmFiles.length - 1].name);
    const dst = path.join(OUTPUT_DIR, "demo.webm");
    if (src !== dst) {
      fs.renameSync(src, dst);
    }
    console.log(`Recording saved to ${dst}`);
  } else {
    console.error("No webm file found in output directory");
    process.exit(1);
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
