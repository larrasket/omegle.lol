import { test, expect } from '@playwright/test';

test('two strangers match on a shared tag and exchange messages', async ({ browser }) => {
	const ctxA = await browser.newContext();
	const ctxB = await browser.newContext();
	const pageA = await ctxA.newPage();
	const pageB = await ctxB.newPage();

	await pageA.goto('/');
	await pageB.goto('/');
	await expect(pageA.getByText('omegle.lol').first()).toBeVisible();
	await expect(pageB.getByText('omegle.lol').first()).toBeVisible();

	// Type a shared tag and start on both.
	for (const p of [pageA, pageB]) {
		await p.getByPlaceholder('tech, music, books').fill('tech');
		await p.keyboard.press('Enter');
		await p.getByRole('button', { name: 'Find someone' }).click();
	}

	// Both should see the shared-tag greeting system message.
	await expect(pageA.getByText('You both like tech. Say hi.')).toBeVisible({ timeout: 5000 });
	await expect(pageB.getByText('You both like tech. Say hi.')).toBeVisible({ timeout: 5000 });

	// A sends a message; B sees it.
	await pageA.getByLabel('Message').fill('hello from A');
	await pageA.getByLabel('Message').press('Enter');
	await expect(pageB.getByText('hello from A')).toBeVisible({ timeout: 5000 });

	// B stops via the Stop button (now in the message row).
	await pageB.getByRole('button', { name: 'Stop' }).click();
	await expect(pageA.getByText('Stranger has disconnected.')).toBeVisible({ timeout: 5000 });

	await ctxA.close();
	await ctxB.close();
});

test('search with no tags eventually matches via fallback', async ({ browser }) => {
	const ctxA = await browser.newContext();
	const ctxB = await browser.newContext();
	const pageA = await ctxA.newPage();
	const pageB = await ctxB.newPage();

	await pageA.goto('/');
	await pageB.goto('/');

	await pageA.getByRole('button', { name: 'Find someone' }).click();
	await pageB.getByRole('button', { name: 'Find someone' }).click();

	await expect(pageA.getByText("You're chatting with a stranger. Say hi.")).toBeVisible({
		timeout: 15_000
	});
	await expect(pageB.getByText("You're chatting with a stranger. Say hi.")).toBeVisible({
		timeout: 15_000
	});

	await ctxA.close();
	await ctxB.close();
});
