import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vitest/config';

export default defineConfig({
	plugins: [sveltekit()],
	server: {
		proxy: {
			'/api': 'http://localhost:8080',
			'/ws': { target: 'ws://localhost:8080', ws: true },
			'/metrics': 'http://localhost:8080',
			'/healthz': 'http://localhost:8080',
			'/readyz': 'http://localhost:8080'
		}
	},
	test: {
		environment: 'jsdom',
		globals: true,
		include: ['src/**/*.{test,spec}.ts']
	}
});
