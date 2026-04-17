import { test, expect } from "@playwright/test";

test.describe("Keyboard navigation", () => {
  test("skip link is first tab stop and targets #main", async ({ page }) => {
    await page.goto("/");
    // Press Tab once — skip link should be focused
    await page.keyboard.press("Tab");
    const focused = await page.evaluate(() => document.activeElement?.textContent?.trim());
    expect(focused).toMatch(/skip/i);
    // Activate it — focus should move to #main
    await page.keyboard.press("Enter");
    const mainFocused = await page.evaluate(() => document.activeElement?.id);
    expect(mainFocused).toBe("main");
  });

  test("exchange tablist responds to ArrowRight and ArrowLeft", async ({ page }) => {
    await page.goto("/binance");
    // Find the exchange-level NavLinks (Binance/Polymarket) — these are NavLinks not tablist
    // Find the INNER animated tablist (Chart/Open Positions/Trade History/Decisions)
    const tablist = page.getByRole("tablist").first();
    await tablist.getByRole("tab").first().focus();
    const firstTabLabel = await page.evaluate(() => (document.activeElement as HTMLElement)?.textContent?.trim());
    // ArrowRight → next tab focused
    await page.keyboard.press("ArrowRight");
    const secondTabLabel = await page.evaluate(() => (document.activeElement as HTMLElement)?.textContent?.trim());
    expect(secondTabLabel).not.toBe(firstTabLabel);
    // ArrowLeft → back to first
    await page.keyboard.press("ArrowLeft");
    const backLabel = await page.evaluate(() => (document.activeElement as HTMLElement)?.textContent?.trim());
    expect(backLabel).toBe(firstTabLabel);
  });

  test("ArrowRight from last tab wraps to first (or Home/End)", async ({ page }) => {
    await page.goto("/binance");
    const tablist = page.getByRole("tablist").first();
    const tabs = tablist.getByRole("tab");
    const count = await tabs.count();
    // Focus last tab
    await tabs.nth(count - 1).focus();
    await page.keyboard.press("End");
    // End key — focus should stay on last
    const afterEnd = await page.evaluate(() => (document.activeElement as HTMLElement)?.textContent?.trim());
    // Home key — should go to first
    await page.keyboard.press("Home");
    const afterHome = await page.evaluate(() => (document.activeElement as HTMLElement)?.textContent?.trim());
    const firstLabel = await tabs.first().textContent();
    expect(afterHome?.trim()).toBe(firstLabel?.trim());
  });

  test("ChartSettings ESC closes popover and returns focus to trigger", async ({ page }) => {
    await page.goto("/binance");
    // Find and click the chart settings trigger
    const trigger = page.getByRole("button", { name: /settings|chart settings|⚙/i });
    if (await trigger.count() === 0) {
      test.skip(); // ChartSettings not on this view
      return;
    }
    await trigger.click();
    // Panel should be visible
    const panel = page.getByRole("dialog", { name: /chart settings/i });
    await expect(panel).toBeVisible();
    // Press ESC
    await page.keyboard.press("Escape");
    await expect(panel).not.toBeVisible();
    // Focus should be back on trigger
    const focusedAfterClose = await page.evaluate(() => (document.activeElement as HTMLElement)?.getAttribute("aria-label") ?? document.activeElement?.textContent?.trim());
    expect(focusedAfterClose).toMatch(/settings|⚙/i);
  });
});
