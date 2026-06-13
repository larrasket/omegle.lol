// Background-tab attention pull. When a new message arrives while the
// tab is hidden, flicker the document title between `(N) new messages`
// and the original, and swap the favicon between the regular cyan tile
// and a variant with a red notification dot. Stops as soon as the tab
// becomes visible — at that point the user is already looking.
//
// Same trick old-school Facebook / Slack / GitHub use to grab the eye
// from a tab strip. Cheap, no permissions required (Notification API
// would need an explicit grant and is overkill for "you have a message").

const NORMAL_FAVICON = '/favicon.svg';
const ALERT_FAVICON = '/favicon-alert.svg';
const TICK_MS = 1000;

let timer: ReturnType<typeof setInterval> | null = null;
let unreadCount = 0;
let originalTitle = '';
let phase = 0;
let listenerAttached = false;

function setFavicon(href: string): void {
	const link = document.querySelector<HTMLLinkElement>('link[rel="icon"][type="image/svg+xml"]');
	if (link !== null) link.href = href;
}

function alertTitleText(): string {
	const noun = unreadCount === 1 ? 'message' : 'messages';
	return `(${String(unreadCount)}) new ${noun}`;
}

function applyPhase(): void {
	if (phase === 0) {
		document.title = alertTitleText();
		setFavicon(ALERT_FAVICON);
	} else {
		document.title = originalTitle;
		setFavicon(NORMAL_FAVICON);
	}
}

function start(): void {
	if (timer !== null) return;
	originalTitle = document.title;
	phase = 0;
	applyPhase();
	timer = setInterval(() => {
		phase = phase === 0 ? 1 : 0;
		applyPhase();
	}, TICK_MS);
}

function stop(): void {
	if (timer === null) {
		unreadCount = 0;
		return;
	}
	clearInterval(timer);
	timer = null;
	unreadCount = 0;
	document.title = originalTitle;
	setFavicon(NORMAL_FAVICON);
}

function attachVisibility(): void {
	if (listenerAttached) return;
	if (typeof document === 'undefined') return;
	document.addEventListener('visibilitychange', () => {
		if (document.visibilityState === 'visible') stop();
	});
	listenerAttached = true;
}

// notifyNewMessage bumps the unread counter and (re)starts the flicker.
// No-op when the tab is foregrounded; the user already sees the chat.
export function notifyNewMessage(): void {
	if (typeof document === 'undefined') return;
	attachVisibility();
	if (document.visibilityState !== 'hidden') return;
	unreadCount += 1;
	if (timer === null) {
		start();
	} else {
		// Refresh title immediately with the new count, regardless of
		// where in the flicker cycle we are.
		phase = 0;
		applyPhase();
	}
}
