// Mirror of backend/internal/proto/messages.go.
// IMPORTANT: keep in sync — there is no codegen for MVP.

export type ClientType =
	| 'search'
	| 'cancel'
	| 'message'
	| 'typing'
	| 'next'
	| 'stop'
	| 'pong'
	| 'pause'
	| 'resume'
	| 'report';
export type ServerType =
	| 'welcome'
	| 'searching'
	| 'matched'
	| 'peer_msg'
	| 'peer_typing'
	| 'peer_left'
	| 'peer_paused'
	| 'peer_resumed'
	| 'error'
	| 'ping';

export interface Envelope<T = unknown> {
	type: ClientType | ServerType;
	data: T;
}

export interface SearchData {
	tags: string[];
}
export interface MessageData {
	text: string;
}
export interface TypingData {
	active: boolean;
}
export interface NextData {
	tags: string[];
}
export interface WelcomeData {
	sessionId: string;
	onlineCount: number;
}
export interface MatchedData {
	sharedTags: string[];
}
export interface PeerMsgData {
	text: string;
	ts: number;
}
export interface PeerTypingData {
	active: boolean;
}
export interface PeerLeftData {
	reason: 'stop' | 'next' | 'disconnect';
}
export interface ErrorData {
	code: string;
	message: string;
}

export const Err = {
	INVALID_JSON: 'invalid_json',
	INVALID_TYPE: 'invalid_type',
	INVALID_STATE: 'invalid_state',
	TOO_MANY_TAGS: 'too_many_tags',
	TAG_TOO_LONG: 'tag_too_long',
	INVALID_TAG: 'invalid_tag',
	MESSAGE_TOO_LARGE: 'message_too_large',
	RATE_LIMITED: 'rate_limited',
	ORIGIN_DENIED: 'origin_denied',
	CONNECT_RATE_LIMITED: 'connect_rate_limited'
} as const;
