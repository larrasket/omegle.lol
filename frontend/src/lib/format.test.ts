import { describe, it, expect } from 'vitest';

import { linkify, groupMessages } from './format';
import type { ChatMessage } from './store';

const msg = (from: ChatMessage['from'], text: string, ts: number): ChatMessage => ({
	from,
	text,
	ts
});

describe('linkify', () => {
	it('returns single text segment when no URL', () => {
		expect(linkify('hello there')).toEqual([{ kind: 'text', value: 'hello there' }]);
	});

	it('extracts a single URL', () => {
		expect(linkify('see https://example.com please')).toEqual([
			{ kind: 'text', value: 'see ' },
			{ kind: 'link', value: 'https://example.com' },
			{ kind: 'text', value: ' please' }
		]);
	});

	it('extracts multiple URLs', () => {
		expect(linkify('a https://x.com b https://y.com c')).toEqual([
			{ kind: 'text', value: 'a ' },
			{ kind: 'link', value: 'https://x.com' },
			{ kind: 'text', value: ' b ' },
			{ kind: 'link', value: 'https://y.com' },
			{ kind: 'text', value: ' c' }
		]);
	});

	it('handles URL at start', () => {
		expect(linkify('https://x.com text')).toEqual([
			{ kind: 'link', value: 'https://x.com' },
			{ kind: 'text', value: ' text' }
		]);
	});

	it('handles URL at end', () => {
		expect(linkify('text https://x.com')).toEqual([
			{ kind: 'text', value: 'text ' },
			{ kind: 'link', value: 'https://x.com' }
		]);
	});

	it('does not match plain domain without protocol', () => {
		expect(linkify('example.com is not a link')).toEqual([
			{ kind: 'text', value: 'example.com is not a link' }
		]);
	});

	it('preserves whitespace exactly', () => {
		expect(linkify('  hello  ')).toEqual([{ kind: 'text', value: '  hello  ' }]);
	});

	it('handles empty string', () => {
		expect(linkify('')).toEqual([]);
	});
});

describe('groupMessages', () => {
	it('returns empty for empty input', () => {
		expect(groupMessages([])).toEqual([]);
	});

	it('emits one separator before the first group', () => {
		const groups = groupMessages([msg('you', 'a', 1000)]);
		expect(groups[0]?.kind).toBe('separator');
	});

	it('groups consecutive same-sender messages within gap', () => {
		const msgs = [
			msg('stranger', 'a', 1000),
			msg('stranger', 'b', 1100),
			msg('stranger', 'c', 1200)
		];
		const out = groupMessages(msgs);
		const groupEntries = out.filter((g) => g.kind === 'group');
		expect(groupEntries).toHaveLength(1);
		expect(groupEntries[0]).toMatchObject({ from: 'stranger', lines: msgs });
	});

	it('splits on sender change', () => {
		const out = groupMessages([
			msg('stranger', 'a', 1000),
			msg('you', 'b', 1100),
			msg('stranger', 'c', 1200)
		]);
		const groupEntries = out.filter((g) => g.kind === 'group');
		expect(groupEntries).toHaveLength(3);
		const froms = groupEntries.map((g) => g.from);
		expect(froms).toEqual(['stranger', 'you', 'stranger']);
	});

	it('splits on > 5 minute gap', () => {
		const fiveMinPlus = 5 * 60 * 1000 + 1;
		const out = groupMessages([
			msg('stranger', 'a', 1000),
			msg('stranger', 'b', 1000 + fiveMinPlus)
		]);
		const groupEntries = out.filter((g) => g.kind === 'group');
		expect(groupEntries).toHaveLength(2);
	});

	it('emits a separator between gap-separated groups', () => {
		const fiveMinPlus = 5 * 60 * 1000 + 1;
		const out = groupMessages([
			msg('stranger', 'a', 1000),
			msg('stranger', 'b', 1000 + fiveMinPlus)
		]);
		const separators = out.filter((g) => g.kind === 'separator');
		expect(separators).toHaveLength(2);
	});

	it('does NOT emit a separator between same-sender contiguous groups under gap', () => {
		const out = groupMessages([msg('stranger', 'a', 1000), msg('stranger', 'b', 2000)]);
		const separators = out.filter((g) => g.kind === 'separator');
		expect(separators).toHaveLength(1);
	});

	it('does NOT emit a separator between sender-change groups under gap', () => {
		const out = groupMessages([msg('you', 'a', 1000), msg('stranger', 'b', 2000)]);
		const separators = out.filter((g) => g.kind === 'separator');
		expect(separators).toHaveLength(1);
	});

	it('preserves message order within a group', () => {
		const msgs = [msg('you', 'first', 1000), msg('you', 'second', 1100), msg('you', 'third', 1200)];
		const out = groupMessages(msgs);
		const group = out.find((g) => g.kind === 'group');
		expect(group?.kind).toBe('group');
		if (group?.kind === 'group') {
			expect(group.lines.map((m) => m.text)).toEqual(['first', 'second', 'third']);
		}
	});
});
