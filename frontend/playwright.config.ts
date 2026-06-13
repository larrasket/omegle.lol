import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
	testDir: './tests/e2e',
	timeout: 20_000,
	use: { baseURL: 'http://localhost:5173' },
	projects: [{ name: 'chromium', use: devices['Desktop Chrome'] }],
	webServer: [
		{
			command: 'cd ../backend && go run ./cmd/server',
			port: 8080,
			reuseExistingServer: !process.env.CI,
			timeout: 30_000
		},
		{
			command: 'npm run dev',
			port: 5173,
			reuseExistingServer: !process.env.CI,
			timeout: 30_000
		}
	]
});
