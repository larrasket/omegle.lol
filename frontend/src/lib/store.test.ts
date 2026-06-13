import { describe, it, expect, beforeEach, vi } from 'vitest';

import type { Envelope } from './proto';
import { createChatStore } from './store';

function mockWs() {
	const handlers: ((e: Envelope) => void)[] = [];
	const sent: { type: string; data: unknown }[] = [];
	return {
		client: {
			connect: vi.fn(),
			close: vi.fn(),
			send: vi.fn((type: string, data: unknown) => sent.push({ type, data })),
			on: vi.fn((h: (e: Envelope) => void) => {
				handlers.push(h);
				return () => {
					/* unsubscribe stub */
				};
			}),
			onStatus: vi.fn(() => () => {
				/* unsubscribe stub */
			})
		},
		emit(env: Envelope) {
			for (const h of handlers) h(env);
		},
		sent
	};
}

describe('chat store', () => {
	let mock: ReturnType<typeof mockWs>;

	beforeEach(() => {
		mock = mockWs();
	});

	it('starts in idle', () => {
		const store = createChatStore(mock.client);
		expect(store.snapshot().state).toBe('idle');
	});

	it('search() transitions to searching and sends search', () => {
		const store = createChatStore(mock.client);
		store.search(['tech']);
		expect(store.snapshot().state).toBe('searching');
		expect(mock.sent).toEqual([{ type: 'search', data: { tags: ['tech'] } }]);
	});

	it('matched event transitions to chatting', () => {
		const store = createChatStore(mock.client);
		store.search([]);
		mock.emit({ type: 'matched', data: { sharedTags: ['tech'] } });
		const s = store.snapshot();
		expect(s.state).toBe('chatting');
		expect(s.sharedTags).toEqual(['tech']);
		expect(s.messages).toHaveLength(1);
		expect(s.messages[0]).toMatchObject({ from: 'system' });
	});

	it('sendMessage appends to local messages and emits', () => {
		const store = createChatStore(mock.client);
		store.search([]);
		mock.emit({ type: 'matched', data: { sharedTags: [] } });
		store.sendMessage('hi');
		const s = store.snapshot();
		expect(s.messages.at(-1)).toMatchObject({ from: 'you', text: 'hi' });
		expect(mock.sent.at(-1)).toEqual({ type: 'message', data: { text: 'hi' } });
	});

	it('peer_msg appends stranger message', () => {
		const store = createChatStore(mock.client);
		store.search([]);
		mock.emit({ type: 'matched', data: { sharedTags: [] } });
		mock.emit({ type: 'peer_msg', data: { text: 'yo', ts: 123 } });
		expect(store.snapshot().messages.at(-1)).toMatchObject({ from: 'stranger', text: 'yo' });
	});

	it('stop sends stop and returns to idle, preserving messages with system append', () => {
		const store = createChatStore(mock.client);
		store.search([]);
		mock.emit({ type: 'matched', data: { sharedTags: [] } });
		store.sendMessage('hi');
		store.stop();
		const s = store.snapshot();
		expect(s.state).toBe('idle');
		// system "matched" + "hi" + system "You disconnected."
		expect(s.messages).toHaveLength(3);
		expect(s.messages[1]).toMatchObject({ from: 'you', text: 'hi' });
		expect(s.messages[2]).toMatchObject({ from: 'system', text: 'You disconnected.' });
		expect(mock.sent.at(-1)).toEqual({ type: 'stop', data: {} });
	});

	it('next sends next with new tags and returns to searching', () => {
		const store = createChatStore(mock.client);
		store.search(['a']);
		mock.emit({ type: 'matched', data: { sharedTags: [] } });
		store.next(['b']);
		expect(store.snapshot().state).toBe('searching');
		expect(mock.sent.at(-1)).toEqual({ type: 'next', data: { tags: ['b'] } });
	});

	it('peer_left shows banner state and appends system message', () => {
		const store = createChatStore(mock.client);
		store.search([]);
		mock.emit({ type: 'matched', data: { sharedTags: [] } });
		mock.emit({ type: 'peer_left', data: { reason: 'stop' } });
		const s = store.snapshot();
		expect(s.state).toBe('peer-left');
		// system "matched" + system "Stranger has disconnected."
		expect(s.messages).toHaveLength(2);
		expect(s.messages[1]).toMatchObject({ from: 'system', text: 'Stranger has disconnected.' });
	});

	it('matched with no shared tags shows generic greeting', () => {
		const store = createChatStore(mock.client);
		store.search([]);
		mock.emit({ type: 'matched', data: { sharedTags: [] } });
		const s = store.snapshot();
		expect(s.messages).toHaveLength(1);
		expect(s.messages[0]).toMatchObject({
			from: 'system',
			text: "You're chatting with a stranger. Say hi."
		});
	});

	it('matched with one shared tag mentions it', () => {
		const store = createChatStore(mock.client);
		store.search(['tech']);
		mock.emit({ type: 'matched', data: { sharedTags: ['tech'] } });
		const s = store.snapshot();
		expect(s.messages[0]).toMatchObject({
			from: 'system',
			text: 'You both like tech. Say hi.'
		});
	});

	it('matched with two shared tags joins with "and"', () => {
		const store = createChatStore(mock.client);
		store.search(['tech', 'music']);
		mock.emit({ type: 'matched', data: { sharedTags: ['tech', 'music'] } });
		const s = store.snapshot();
		expect(s.messages[0]).toMatchObject({
			from: 'system',
			text: 'You both like tech and music. Say hi.'
		});
	});

	it('matched with three+ shared tags uses Oxford comma', () => {
		const store = createChatStore(mock.client);
		store.search(['a', 'b', 'c']);
		mock.emit({ type: 'matched', data: { sharedTags: ['a', 'b', 'c'] } });
		const s = store.snapshot();
		expect(s.messages[0]).toMatchObject({
			from: 'system',
			text: 'You both like a, b, and c. Say hi.'
		});
	});

	it('first welcome stores sessionId without appending system message', () => {
		const store = createChatStore(mock.client);
		mock.emit({ type: 'welcome', data: { sessionId: 'sess-1' } });
		const s = store.snapshot();
		expect(s.sessionId).toBe('sess-1');
		expect(s.state).toBe('idle');
		expect(s.messages).toEqual([]);
	});

	it('reconnect with a new sessionId mid-chat appends Connection-lost', () => {
		const store = createChatStore(mock.client);
		mock.emit({ type: 'welcome', data: { sessionId: 'sess-1' } });
		store.search(['tech']);
		mock.emit({ type: 'matched', data: { sharedTags: ['tech'] } });
		store.sendMessage('hello');
		mock.emit({ type: 'welcome', data: { sessionId: 'sess-2' } });
		const s = store.snapshot();
		expect(s.state).toBe('idle');
		expect(s.sessionId).toBe('sess-2');
		expect(s.peerTyping).toBe(false);
		expect(s.messages).toHaveLength(3);
		expect(s.messages.at(-1)).toMatchObject({
			from: 'system',
			text: 'Connection lost. Press Esc or Find someone to try again.'
		});
	});

	it('reconnect from fresh idle does not add Connection-lost', () => {
		const store = createChatStore(mock.client);
		mock.emit({ type: 'welcome', data: { sessionId: 'sess-1' } });
		mock.emit({ type: 'welcome', data: { sessionId: 'sess-2' } });
		const s = store.snapshot();
		expect(s.sessionId).toBe('sess-2');
		expect(s.messages).toEqual([]);
	});
});
