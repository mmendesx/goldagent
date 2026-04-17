import { test, expect, type Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

// Run axe against the current page and assert no serious/critical WCAG violations.
async function checkA11y(page: Page) {
  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21aa"])
    .analyze();
  const serious = results.violations.filter(
    (v) => v.impact === "serious" || v.impact === "critical"
  );
  if (serious.length > 0) {
    console.error(
      "A11y violations:",
      JSON.stringify(
        serious.map((v) => ({
          id: v.id,
          impact: v.impact,
          description: v.description,
          nodes: v.nodes.map((n) => n.html),
        })),
        null,
        2
      )
    );
  }
  expect(serious).toHaveLength(0);
}

test.describe("WCAG 2.1 AA — Dashboard", () => {
  test("/ redirects and homepage loads", async ({ page }) => {
    await page.goto("/");
    await page.waitForURL(/\/binance/);
    await checkA11y(page);
  });

  test("/binance/chart", async ({ page }) => {
    await page.goto("/binance/chart");
    await page.waitForLoadState("networkidle");
    await checkA11y(page);
  });

  test("/binance/overview — positions", async ({ page }) => {
    await page.goto("/binance");
    // Click the Open Positions tab if present (label used in BinanceView)
    const overviewTab = page.getByRole("tab", { name: /overview|positions/i });
    if ((await overviewTab.count()) > 0) await overviewTab.click();
    await page.waitForLoadState("networkidle");
    await checkA11y(page);
  });

  test("/polymarket/chart", async ({ page }) => {
    await page.goto("/polymarket/chart");
    await page.waitForLoadState("networkidle");
    await checkA11y(page);
  });

  test("/polymarket/overview", async ({ page }) => {
    await page.goto("/polymarket");
    const overviewTab = page.getByRole("tab", { name: /overview/i });
    if ((await overviewTab.count()) > 0) await overviewTab.click();
    await page.waitForLoadState("networkidle");
    await checkA11y(page);
  });
});
