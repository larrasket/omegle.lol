import type { Envelope, ClientType, ServerType } from './proto';

type Handler = (env: Envelope) => void;

export interface WsClient {
	connect(): void;
	close(): void;
	send(type: ClientType, data?: unknown): void;
	on(handler: Handler): () => void;
	onStatus(handler: (s: 'connecting' | 'open' | 'closed') => void): () => void;
}

export function createWsClient(url: string): WsClient {
	let ws: WebSocket | null = null;
	const msgHandlers = new Set<Handler>();
	const statusHandlers = new Set<(s: 'connecting' | 'open' | 'closed') => void>();
	let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
	let closedByUser = false;
	let visibilityHandler: (() => void) | null = null;

	function fanoutStatus(s: 'connecting' | 'open' | 'closed') {
		for (const h of statusHandlers) h(s);
	}

	function connect() {
		closedByUser = false;
		fanoutStatus('connecting');
		ws = new WebSocket(url);
		ws.onopen = () => {
			fanoutStatus('open');
			// If the tab is already hidden when the socket comes up (rare —
			// happens after a reconnect that completes in the background),
			// tell the server right away so the heartbeat grace kicks in
			// before the first scheduled ping.
			if (typeof document !== 'undefined' && document.visibilityState === 'hidden') {
				ws?.send(JSON.stringify({ type: 'pause', data: {} }));
			}
		};
		ws.onmessage = (e) => {
			try {
				const env = JSON.parse(String(e.data)) as Envelope;
				// Auto-reply to pings.
				if (env.type === ('ping' satisfies ServerType)) {
					ws?.send(JSON.stringify({ type: 'pong', data: {} }));
					return;
				}
				for (const h of msgHandlers) h(env);
			} catch {
				// ignore malformed
			}
		};
		ws.onclose = () => {
			fanoutStatus('closed');
			if (!closedByUser) {
				reconnectTimer = setTimeout(connect, 1000);
			}
		};
		ws.onerror = () => ws?.close();
	}

	// Mobile browsers throttle JS in background tabs and may freeze it
	// entirely when the screen locks. Without a heads-up, the server kicks
	// us on heartbeat timeout and the chat ends. visibilitychange fires
	// reliably across iOS Safari / Android Chrome, so we use it to send a
	// pause/resume that buys ourselves a longer grace window server-side.
	function attachVisibility() {
		if (typeof document === 'undefined' || visibilityHandler !== null) return;
		visibilityHandler = () => {
			const t = document.visibilityState === 'hidden' ? 'pause' : 'resume';
			if (ws?.readyState === WebSocket.OPEN) {
				ws.send(JSON.stringify({ type: t, data: {} }));
			}
		};
		document.addEventListener('visibilitychange', visibilityHandler);
	}

	function close() {
		closedByUser = true;
		if (reconnectTimer) clearTimeout(reconnectTimer);
		if (visibilityHandler !== null && typeof document !== 'undefined') {
			document.removeEventListener('visibilitychange', visibilityHandler);
			visibilityHandler = null;
		}
		ws?.close();
	}

	function send(type: ClientType, data?: unknown) {
		if (ws?.readyState !== WebSocket.OPEN) return;
		ws.send(JSON.stringify({ type, data: data ?? {} }));
	}

	attachVisibility();

	return {
		connect,
		close,
		send,
		on(h) {
			msgHandlers.add(h);
			return () => msgHandlers.delete(h);
		},
		onStatus(h) {
			statusHandlers.add(h);
			return () => statusHandlers.delete(h);
		}
	};
}
