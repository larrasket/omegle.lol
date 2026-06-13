import js from '@eslint/js';
import ts from 'typescript-eslint';
import svelte from 'eslint-plugin-svelte';
import { flatConfigs as impFlatConfigs } from 'eslint-plugin-import-x';
import unicorn from 'eslint-plugin-unicorn';
import prettier from 'eslint-config-prettier';
import svelteParser from 'svelte-eslint-parser';
import tsParser from '@typescript-eslint/parser';
import { createTypeScriptImportResolver } from 'eslint-import-resolver-typescript';
import globals from 'globals';

export default ts.config(
	js.configs.recommended,
	...ts.configs.strictTypeChecked,
	...ts.configs.stylisticTypeChecked,
	...svelte.configs['flat/recommended'],
	impFlatConfigs.recommended,
	// Disable type-checked rules for plain JS config files (no tsconfig project).
	{
		files: ['**/*.js'],
		...ts.configs.disableTypeChecked
	},
	{
		// playwright.config.ts is not part of the app tsconfig — enable type checking for it separately.
		files: ['**/*.{ts,svelte}'],
		ignores: ['playwright.config.ts'],
		plugins: { unicorn },
		languageOptions: {
			globals: { ...globals.browser },
			parserOptions: { project: './tsconfig.json', extraFileExtensions: ['.svelte'] }
		},
		settings: {
			'import-x/resolver-next': [createTypeScriptImportResolver()]
		},
		rules: {
			'no-console': 'error',
			'no-debugger': 'error',
			'no-alert': 'error',
			'no-eval': 'error',
			'no-implied-eval': 'error',
			'no-restricted-syntax': [
				'error',
				{ selector: 'TSEnumDeclaration', message: 'Use const objects + `as const` instead.' }
			],
			eqeqeq: ['error', 'always'],
			curly: ['error', 'all'],
			'prefer-const': ['error', { destructuring: 'all' }],
			'prefer-template': 'error',
			complexity: ['error', 15],
			'max-depth': ['error', 4],
			'@typescript-eslint/no-explicit-any': 'error',
			'@typescript-eslint/no-unnecessary-condition': 'error',
			'@typescript-eslint/strict-boolean-expressions': ['error', { allowNullableBoolean: false }],
			'@typescript-eslint/switch-exhaustiveness-check': 'error',
			'@typescript-eslint/consistent-type-imports': ['error', { fixStyle: 'inline-type-imports' }],
			'@typescript-eslint/no-floating-promises': 'error',
			'@typescript-eslint/no-misused-promises': 'error',
			'@typescript-eslint/require-await': 'error',
			'@typescript-eslint/await-thenable': 'error',
			'@typescript-eslint/promise-function-async': 'error',
			'@typescript-eslint/no-unused-vars': [
				'error',
				{ argsIgnorePattern: '^_', varsIgnorePattern: '^_' }
			],
			'import-x/no-cycle': 'error',
			'import-x/no-self-import': 'error',
			'import-x/no-default-export': 'off',
			'import-x/order': ['error', { 'newlines-between': 'always', alphabetize: { order: 'asc' } }],
			'unicorn/prefer-node-protocol': 'error',
			'unicorn/no-array-for-each': 'off',
			'unicorn/prefer-top-level-await': 'off',
			'unicorn/filename-case': ['error', { cases: { kebabCase: true, pascalCase: true } }]
		}
	},
	{
		files: ['**/*.svelte'],
		languageOptions: { parser: svelteParser, parserOptions: { parser: tsParser } },
		rules: {
			'svelte/no-at-html-tags': 'error',
			'svelte/no-target-blank': 'error',
			'svelte/no-reactive-functions': 'error',
			'svelte/require-each-key': 'error',
			'svelte/valid-compile': 'error',
			'svelte/no-store-async': 'error',
			'svelte/button-has-type': 'error',
			'svelte/no-useless-mustaches': 'error',
			'svelte/sort-attributes': 'warn',
			// Plain <a> tags for static routes are correct; no programmatic navigation needed.
			'svelte/no-navigation-without-resolve': 'off'
		}
	},
	// playwright.config.ts is not part of the app tsconfig — lint without type info.
	{
		files: ['playwright.config.ts'],
		languageOptions: { parserOptions: { project: false } },
		...ts.configs.disableTypeChecked
	},
	{
		files: ['**/*.test.ts', '**/tests/**/*.ts'],
		rules: {
			'@typescript-eslint/no-explicit-any': 'off',
			'@typescript-eslint/no-floating-promises': 'off',
			'no-console': 'off'
		}
	},
	prettier,
	{ ignores: ['build/', '.svelte-kit/', 'node_modules/', 'playwright-report/', 'test-results/'] }
);
