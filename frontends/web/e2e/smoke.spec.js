import { test, expect } from '@playwright/test';

test.describe('frontend smoke', () => {
  test('page loads with title', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveTitle(/ABCMovies/i);
  });

  test('session chip shows anonymous initially', async ({ page }) => {
    await page.goto('/');
    const chip = page.locator('#sessionChip');
    await expect(chip).toHaveText('anonymous');
  });

  test('signup creates a session and shows username', async ({ page }) => {
    await page.goto('/');

    const username = `testuser_${Date.now()}`;
    await page.fill('#username', username);
    await page.fill('#password', 'test-password-123');
    await page.click('#signup');

    const chip = page.locator('#sessionChip');
    await expect(chip).toHaveText(`logged in as ${username}`, {
      timeout: 10_000,
    });
  });

  test('recovery key is shown after signup', async ({ page }) => {
    await page.goto('/');

    const username = `testuser_${Date.now()}`;
    await page.fill('#username', username);
    await page.fill('#password', 'test-password-123');
    await page.click('#signup');

    const recovery = page.locator('#recovery');
    await expect(recovery).toBeVisible({ timeout: 10_000 });
    await expect(recovery).toContainText('Recovery key');
  });

  test('login works after signup', async ({ page }) => {
    await page.goto('/');

    const username = `testuser_${Date.now()}`;
    const password = 'test-password-123';

    // Sign up first
    await page.fill('#username', username);
    await page.fill('#password', password);
    await page.click('#signup');
    await expect(page.locator('#sessionChip')).toHaveText(
      `logged in as ${username}`,
      {
        timeout: 10_000,
      },
    );

    // Log out
    await page.click('#logout');
    await expect(page.locator('#sessionChip')).toHaveText('anonymous');

    // Log back in
    await page.fill('#username', username);
    await page.fill('#password', password);
    await page.click('#login');
    await expect(page.locator('#sessionChip')).toHaveText(
      `logged in as ${username}`,
      {
        timeout: 10_000,
      },
    );
  });
});
