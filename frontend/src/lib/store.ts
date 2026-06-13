import { writable, get, type Writable } from 'svelte/store';

import { notifyNewMessage } from './notify';
import type {
	Envelope,
	MatchedData,
	PeerLeftData,
	PeerMsgData,
	PeerTypingData,
	WelcomeData,
	ErrorData
} from './proto';
import type { WsClient } from './ws';

export type ViewState = 'connecting' | 'idle' | 'searching' | 'chatting' | 'peer-left';

export interface ChatMessage {
	from: 'you' | 'stranger' | 'system';
	text: string;
	ts: number;
}

export interface ChatStoreState {
	state: ViewState;
	sessionId: string | null;
	tags: string[];
	sharedTags: string[];
	messages: ChatMessage[];
	peerTyping: boolean;
	peerAway: boolean;
	onlineCount: number;
	lastError: ErrorData | null;
	peerLeftReason: 'stop' | 'next' | 'disconnect' | null;
}

const initial: ChatStoreState = {
	state: 'idle',
	sessionId: null,
	tags: [],
	sharedTags: [],
	messages: [],
	peerTyping: false,
	peerAway: false,
	onlineCount: 0,
	lastError: null,
	peerLeftReason: null
};

export interface ChatStore {
	subscribe: Writable<ChatStoreState>['subscribe'];
	snapshot(): ChatStoreState;
	search(tags: string[]): void;
	cancel(): void;
	sendMessage(text: string): void;
	setTyping(active: boolean): void;
	next(tags: string[]): void;
	stop(): void;
	report(): void;
	dismissPeerLeft(): void;
	clearMessages(): void;
}

function formatList(items: string[]): string {
	if (items.length === 0) return '';
	if (items.length === 1) return items[0] ?? '';
	if (items.length === 2) return `${items[0] ?? ''} and ${items[1] ?? ''}`;
	return `${items.slice(0, -1).join(', ')}, and ${items.at(-1) ?? ''}`;
}

export function createChatStore(ws: WsClient): ChatStore {
	const store = writable<ChatStoreState>(initial);

	ws.onStatus((s) => {
		if (s === 'open') {
			store.update((cur) =>
				cur.state === 'connecting' || cur.state === 'idle' ? { ...cur, state: 'idle' } : cur
			);
		}
		if (s === 'closed') {
			store.update((cur) => ({ ...cur, state: 'connecting' }));
		}
	});

	ws.on((env: Envelope) => {
		// eslint-disable-next-line @typescript-eslint/switch-exhaustiveness-check -- ws.on only delivers server→client messages; client types never arrive here
		switch (env.type) {
			case 'welcome': {
				const d = env.data as WelcomeData;
				const onlineCount = typeof d.onlineCount === 'number' ? d.onlineCount : 0;
				store.update((cur) => {
					// First welcome of the session — no prior context, just init.
					if (cur.sessionId === null) {
						return { ...cur, sessionId: d.sessionId, onlineCount, state: 'idle' };
					}
					// Welcome with a different sessionId means the WS reconnected
					// after a drop. The server has no memory of the old chat —
					// anything in messages[] is now an orphan. Surface the truth
					// instead of leaving the user typing into a black hole.
					if (d.sessionId !== cur.sessionId) {
						const messages =
							cur.state === 'chatting' || cur.state === 'searching'
								? [
										...cur.messages,
										{
											from: 'system' as const,
											text: 'Connection lost. Press Esc or Find someone to try again.',
											ts: Date.now()
										}
									]
								: cur.messages;
						return {
							...cur,
							sessionId: d.sessionId,
							onlineCount,
							state: 'idle' as const,
							peerTyping: false,
							messages
						};
					}
					return { ...cur, onlineCount, state: 'idle' };
				});
				break;
			}
			case 'searching':
				store.update((cur) => ({ ...cur, state: 'searching' }));
				break;
			case 'matched': {
				const d = env.data as MatchedData;
				// Defensively filter: if anything other than a non-empty trimmed
				// string somehow reaches us (null in the array, whitespace, etc.)
				// the greeting must still read cleanly.
				const shared = Array.isArray(d.sharedTags)
					? d.sharedTags.filter((t): t is string => typeof t === 'string' && t.trim() !== '')
					: [];
				const greeting =
					shared.length > 0
						? `You both like ${formatList(shared)}. Say hi.`
						: "You're chatting with a stranger. Say hi.";
				store.update((cur) => ({
					...cur,
					state: 'chatting',
					sharedTags: shared,
					messages: [{ from: 'system', text: greeting, ts: Date.now() }],
					peerTyping: false,
					peerAway: false,
					peerLeftReason: null
				}));
				break;
			}
			case 'peer_msg': {
				const d = env.data as PeerMsgData;
				if (typeof navigator !== 'undefined' && 'vibrate' in navigator) {
					(navigator.vibrate as (ms: number) => boolean)(8);
				}
				// No-op when the tab is in the foreground.
				notifyNewMessage();
				store.update((cur) => ({
					...cur,
					messages: [...cur.messages, { from: 'stranger', text: d.text, ts: d.ts }],
					peerTyping: false
				}));
				break;
			}
			case 'peer_typing': {
				const d = env.data as PeerTypingData;
				store.update((cur) => ({ ...cur, peerTyping: d.active }));
				break;
			}
			case 'peer_paused': {
				// Suppress any stale "typing…" — a paused peer can't be typing.
				store.update((cur) => ({ ...cur, peerAway: true, peerTyping: false }));
				break;
			}
			case 'peer_resumed': {
				store.update((cur) => ({ ...cur, peerAway: false }));
				break;
			}
			case 'peer_left': {
				const d = env.data as PeerLeftData;
				store.update((cur) => ({
					...cur,
					state: 'peer-left',
					peerTyping: false,
					peerAway: false,
					peerLeftReason: d.reason,
					messages: [
						...cur.messages,
						{ from: 'system', text: 'Stranger has disconnected.', ts: Date.now() }
					]
				}));
				break;
			}
			case 'error': {
				const d = env.data as ErrorData;
				store.update((cur) => ({ ...cur, lastError: d }));
				break;
			}
			// Client-type messages (search, cancel, message, typing, next, stop, pong) and
			// ping are never sent server→client in the handler; ignore them safely.
			default:
				break;
		}
	});

	return {
		subscribe: store.subscribe,
		snapshot: () => get(store),
		search(tags) {
			store.update((cur) => ({ ...cur, tags, state: 'searching', lastError: null }));
			ws.send('search', { tags });
		},
		cancel() {
			store.update((cur) => ({ ...cur, state: 'idle' }));
			ws.send('cancel');
		},
		sendMessage(text) {
			if (!text.trim()) return;
			const msg: ChatMessage = { from: 'you', text, ts: Date.now() };
			store.update((cur) => ({ ...cur, messages: [...cur.messages, msg] }));
			ws.send('message', { text });
		},
		setTyping(active) {
			ws.send('typing', { active });
		},
		next(tags) {
			store.update((cur) => ({
				...cur,
				tags,
				state: 'searching',
				messages: [],
				peerTyping: false
			}));
			ws.send('next', { tags });
		},
		stop() {
			store.update((cur) => ({
				...cur,
				state: 'idle',
				peerTyping: false,
				messages: [...cur.messages, { from: 'system', text: 'You disconnected.', ts: Date.now() }]
			}));
			ws.send('stop', {});
		},
		report() {
			// Optimistic transition — server ends the room and never sends
			// peer_left back to us (reason "stop"), so the UI shouldn't wait
			// for a confirmation envelope.
			store.update((cur) => ({
				...cur,
				state: 'idle',
				peerTyping: false,
				peerAway: false,
				messages: [
					...cur.messages,
					{ from: 'system', text: 'You reported this stranger.', ts: Date.now() }
				]
			}));
			ws.send('report', {});
		},
		dismissPeerLeft() {
			store.update((cur) => ({ ...cur, state: 'idle', messages: [], peerLeftReason: null }));
		},
		clearMessages() {
			store.update((cur) => ({ ...cur, messages: [] }));
		}
	};
}
