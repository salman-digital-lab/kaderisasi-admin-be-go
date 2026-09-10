import { chromium } from "../node_modules/playwright/index.mjs";
import { expect } from "../node_modules/@playwright/test/index.mjs";
import { mkdirSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
const webRequire = createRequire(
  new URL("../../kaderisasi-web-fe/package.json", import.meta.url),
);
async function checkContrast(page) {
  await page.evaluate(() => {
    for (const animation of document.getAnimations()) {
      if (animation.effect?.getComputedTiming().iterations !== Infinity)
        animation.finish();
    }
  });
  await page.addScriptTag({ path: webRequire.resolve("axe-core/axe.min.js") });
  const violations = await page.evaluate(async () => {
    const result = await window.axe.run(document, {
      runOnly: ["color-contrast"],
    });
    return result.violations.map(({ id, nodes }) => ({
      id,
      nodes: nodes.map(({ target, failureSummary }) => ({
        target,
        failureSummary,
      })),
    }));
  });
  expect(violations).toEqual([]);
}
const output = ".artifacts/approval-ui";
const fixtureUrl = new URL(
  "/tests/browser/approval.html",
  process.env.CERTIFICATE_APPROVAL_UI_ORIGIN || "http://localhost:3005",
).href;
mkdirSync(output, { recursive: true });
const browser = await chromium.launch({ headless: true });
const results = [];
try {
  for (const [name, viewport] of [
    ["desktop", { width: 1440, height: 1000 }],
    ["mobile", { width: 390, height: 844 }],
  ]) {
    const context = await browser.newContext({
      viewport,
      isMobile: name === "mobile",
      hasTouch: name === "mobile",
      locale: "id-ID",
      timezoneId: "Asia/Jakarta",
    });
    const page = await context.newPage();
    const errors = [];
    page.on("pageerror", (e) => errors.push(e.message));
    await page.goto(fixtureUrl);
    await expect(
      page.getByText("Peserta Uji 1", { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Kirim 2 permintaan persetujuan" }),
    ).toBeEnabled();
    await checkContrast(page);
    await page
      .getByRole("button", { name: "Kirim 2 permintaan persetujuan" })
      .click();
    await expect(
      page.getByText("Pilih penandatangan.", { exact: true }),
    ).toBeVisible();
    await page.getByLabel("Penandatangan", { exact: true }).click();
    await page.getByLabel("Penandatangan", { exact: true }).press("ArrowDown");
    await page.getByLabel("Penandatangan", { exact: true }).press("Enter");
    await page.getByLabel("Jabatan pada sertifikat").fill("Ketua kegiatan");
    await page
      .getByRole("button", { name: "Kirim 2 permintaan persetujuan" })
      .click();
    await expect(
      page.getByText("Permintaan terkirim", { exact: true }),
    ).toBeVisible();
    await page.screenshot({
      path: `${output}/${name}-requests.png`,
      fullPage: true,
      animations: "disabled",
    });
    await page
      .getByRole("checkbox", { name: /Pilih (permintaan )?Peserta Uji 1/ })
      .check();
    await page
      .getByRole("checkbox", { name: /Pilih (permintaan )?Peserta Uji 2/ })
      .check();
    await page
      .getByRole("button", { name: "Tinjau 2 permintaan", exact: true })
      .click();
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    const approve = dialog.getByRole("button", {
      name: "Setujui & terbitkan 2 sertifikat",
      exact: true,
    });
    await expect(approve).toBeDisabled();
    await dialog
      .locator(".ant-select")
      .filter({ has: page.getByLabel("Sertifikat yang ditinjau") })
      .click();
    await dialog.getByLabel("Sertifikat yang ditinjau").press("ArrowDown");
    await dialog.getByLabel("Sertifikat yang ditinjau").press("Enter");
    await expect(
      dialog.getByText("Peserta Uji 2", { exact: true }),
    ).toBeVisible();
    await dialog.getByLabel("Sertifikat yang ditinjau").press("Tab");
    await dialog.getByRole("checkbox").check();
    await checkContrast(page);
    await expect(page.locator(".ant-select-dropdown:visible")).toHaveCount(0);
    await page.screenshot({
      path: `${output}/${name}-review.png`,
      fullPage: false,
      animations: "disabled",
    });
    await approve.click();
    await expect(dialog).not.toBeVisible();
    await expect(
      page.getByText("2 permintaan disetujui dan sertifikat diterbitkan."),
    ).toBeVisible();
    await page
      .locator(".ant-select")
      .filter({ has: page.getByLabel("Status persetujuan") })
      .click();
    await page.getByLabel("Status persetujuan").press("ArrowDown");
    await page.getByLabel("Status persetujuan").press("Enter");
    await expect(
      page.getByRole("link", { name: "Lihat sertifikat" }),
    ).toHaveCount(2);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await page.goto(fixtureUrl);
    await page
      .getByRole("button", { name: "Tinjau", exact: true })
      .first()
      .click();
    await expect(
      dialog.getByRole("button", { name: "Tolak 1 permintaan", exact: true }),
    ).toBeDisabled();
    await dialog
      .getByLabel("Catatan (wajib untuk penolakan)")
      .fill("Perbaiki jabatan");
    await dialog
      .getByRole("button", { name: "Tolak 1 permintaan", exact: true })
      .click();
    await expect(
      page.getByText("1 permintaan ditolak.", { exact: true }),
    ).toBeVisible();
    await page
      .locator(".ant-select")
      .filter({ has: page.getByLabel("Status persetujuan") })
      .click();
    await page
      .locator(".ant-select-item-option-content")
      .filter({ hasText: /^Ditolak$/ })
      .click();
    await page
      .getByRole("button", { name: "Tinjau", exact: true })
      .first()
      .click();
    await expect(dialog.getByText(/^Ditolak\nPenandatangan Uji/)).toBeVisible();
    await expect(
      dialog.getByRole("button", { name: /Setujui & terbitkan/ }),
    ).toHaveCount(0);
    await dialog
      .getByRole("button", { name: "Tutup", exact: true })
      .press("Escape");
    await expect(dialog).not.toBeVisible();
    await page.goto(`${fixtureUrl}?mode=cancel`);
    await page
      .getByRole("button", { name: "Tinjau", exact: true })
      .first()
      .click();
    await dialog
      .getByRole("button", { name: "Batalkan permintaan", exact: true })
      .click();
    await expect(
      page.getByText("1 permintaan dibatalkan.", { exact: true }),
    ).toBeVisible();
    await page.goto(`${fixtureUrl}?mode=empty`);
    await expect(
      page.getByText("Belum ada penandatangan aktif", { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByText("Tidak ada permintaan persetujuan pada status ini."),
    ).toBeVisible();
    await checkContrast(page);
    await page.goto(`${fixtureUrl}?mode=error`);
    await expect(
      page.getByText("Penandatangan gagal dimuat. Coba lagi.", { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByText(
        "Permintaan persetujuan gagal dimuat. Muat ulang untuk mencoba lagi.",
        { exact: true },
      ),
    ).toBeVisible();
    await page
      .getByRole("button", { name: "Muat ulang", exact: true })
      .first()
      .click();
    await expect(
      page.getByText("Penandatangan gagal dimuat. Coba lagi.", { exact: true }),
    ).toBeVisible();
    await checkContrast(page);
    expect(errors).toEqual([]);
    results.push({ name, passed: true, contrast: "passed" });
    await context.close();
  }
} finally {
  writeFileSync(`${output}/results.json`, JSON.stringify(results, null, 2));
  await browser.close();
}
console.log(results);
