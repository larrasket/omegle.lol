<script lang="ts">
	// eslint-disable-next-line import-x/no-duplicates -- svelte and svelte/store share a type definition file; this is a false positive
	import { onMount, onDestroy, tick } from 'svelte';
	// eslint-disable-next-line import-x/no-duplicates -- svelte and svelte/store share a type definition file; this is a false positive
	import { fromStore } from 'svelte/store';

	import Footer from '$lib/components/Footer.svelte';
	import Intro from '$lib/components/Intro.svelte';
	import MessageInput from '$lib/components/MessageInput.svelte';
	import MessageList from '$lib/components/MessageList.svelte';
	import StatusBar from '$lib/components/StatusBar.svelte';
	import TagInput from '$lib/components/TagInput.svelte';
	import TypingIndicator from '$lib/components/TypingIndicator.svelte';
	import { createChatStore } from '$lib/store';
	import { createWsClient } from '$lib/ws';

	const wsUrl = `${(typeof window !== 'undefined' && window.location.protocol === 'https:' ? 'wss://' : 'ws://') + (typeof window !== 'undefined' ? window.location.host : 'localhost:5173')}/ws`;
	const ws = createWsClient(wsUrl);
	const store = createChatStore(ws);

	const snap = $derived(fromStore(store));

	let tags = $state<string[]>([]);
	let tagInputEl: HTMLInputElement | undefined = $state();

	// Esc-to-stop is a two-tap action: first press shows a confirmation
	// (3 s timeout), second press actually disconnects. A *third* press, by
	// then in the idle state, starts a fresh search.
	let confirmingStop = $state(false);
	let confirmTimer: ReturnType<typeof setTimeout> | null = null;

	// `editingTags` controls whether the TagInput is visible. The chips are
	// hidden by default once the user has been in a conversation (per the
	// user's request: "they should not be able to see their tags unless they
	// hit a button"). Fresh page load shows the editor automatically since
	// they need it to pick interests.
	let editingTags = $state(false);

	onMount(() => {
		if (typeof localStorage !== 'undefined') {
			try {
				const raw = localStorage.getItem('omegle.lastTags');
				if (raw !== null) {
					const parsed: unknown = JSON.parse(raw);
					if (Array.isArray(parsed)) {
						tags = (parsed as unknown[])
							.filter((x): x is string => typeof x === 'string')
							.slice(0, 10);
					}
				}
			} catch {
				// ignore parse errors
			}
		}
		ws.connect();
	});

	onDestroy(() => {
		ws.close();
		if (confirmTimer !== null) clearTimeout(confirmTimer);
	});

	function start(): void {
		if (typeof localStorage !== 'undefined') {
			localStorage.setItem('omegle.lastTags', JSON.stringify(tags));
		}
		editingTags = false;
		store.search(tags);
	}

	function armStopConfirmation(): void {
		confirmingStop = true;
		if (confirmTimer !== null) clearTimeout(confirmTimer);
		confirmTimer = setTimeout(() => {
			confirmingStop = false;
			confirmTimer = null;
		}, 3000);
	}

	function clearStopConfirmation(): void {
		confirmingStop = false;
		if (confirmTimer !== null) {
			clearTimeout(confirmTimer);
			confirmTimer = null;
		}
	}

	function onKey(e: KeyboardEvent): void {
		if (e.key !== 'Escape') return;

		if (snap.current.state === 'chatting') {
			e.preventDefault();
			if (confirmingStop) {
				// Second Esc → actually disconnect.
				clearStopConfirmation();
				store.stop();
			} else {
				// First Esc → ask for confirmation.
				armStopConfirmation();
			}
		} else if (snap.current.state === 'searching') {
			e.preventDefault();
			store.cancel();
		} else if (snap.current.state === 'peer-left' || snap.current.state === 'idle') {
			// Third Esc (after disconnect) or fresh-page Esc — find a new person.
			e.preventDefault();
			start();
		}
	}

	// Whenever we leave the chat or start searching, drop any stale "confirm
	// disconnect" indicator and collapse the tag editor.
	$effect(() => {
		const s = snap.current.state;
		if (s !== 'chatting') {
			clearStopConfirmation();
		}
		if (s === 'searching' || s === 'chatting') {
			editingTags = false;
		}
	});

	// Auto-focus the tag editor whenever it actually becomes visible.
	$effect(() => {
		const s = snap.current.state;
		const showEditor =
			(s === 'idle' && snap.current.messages.length === 0) ||
			((s === 'idle' || s === 'peer-left') && editingTags);
		if (showEditor && tagInputEl !== undefined) {
			const el = tagInputEl;
			void tick().then(() => {
				el.focus();
			});
		}
	});
</script>

<svelte:window onkeydown={onKey} />

<div class="page">
	<div class="app">
		<header class="topbar">
			<a class="brand" href="/">omegle.lol</a>
			<span class="heart" aria-hidden="true">♡</span>
			<span class="tagline">Talk to strangers.</span>
			{#if snap.current.state === 'chatting'}
				<button
					class="report"
					onclick={() => {
						store.report();
					}}
					title="End the chat and flag this stranger for abuse"
					type="button">Report</button
				>
			{/if}
		</header>

		{#if snap.current.state === 'idle' && snap.current.messages.length === 0}
			<Intro onlineCount={snap.current.onlineCount} />
		{/if}

		<StatusBar {store} />

		{#if snap.current.state === 'chatting' && confirmingStop}
			<div class="confirm-banner" role="alert">
				Press <kbd>Esc</kbd> again to disconnect.
			</div>
		{/if}

		<MessageList messages={snap.current.messages} />

		{#if snap.current.peerAway}
			<div class="typing-row away" role="status">
				Stranger is away<span class="dots"></span>
			</div>
		{:else if snap.current.peerTyping}
			<div class="typing-row"><TypingIndicator /></div>
		{/if}

		<section class="action-row" class:chatting={snap.current.state === 'chatting'}>
			{#if snap.current.state === 'chatting'}
				<div class="chat-row">
					<button
						class="outlined stop"
						class:armed={confirmingStop}
						onclick={() => {
							clearStopConfirmation();
							store.stop();
						}}
						type="button">{confirmingStop ? 'Press Esc' : 'Stop'}</button
					>
					<div class="msg-input"><MessageInput {store} /></div>
				</div>
			{:else if snap.current.state === 'searching'}
				<div class="searching-row">
					<span class="muted">Looking for someone<span class="dots"></span></span>
					<button
						class="outlined"
						onclick={() => {
							store.cancel();
						}}
						type="button">Cancel</button
					>
				</div>
			{:else if editingTags || snap.current.messages.length === 0}
				<div class="lobby-row">
					<TagInput placeholder="tech, music, books" bind:tags bind:input={tagInputEl} />
					<button class="start" onclick={start} type="button">Find someone</button>
				</div>
			{:else}
				<div class="lobby-row">
					<button
						class="outlined change-tags"
						onclick={() => {
							editingTags = true;
						}}
						type="button">Change interests</button
					>
					<button class="start" onclick={start} type="button">Find someone</button>
				</div>
			{/if}
		</section>

		<Footer />
	</div>
</div>

<style>
	/* Lock the chat route to a single viewport-sized pane so iOS doesn't
	   leak a body-level scrollbar when keyboard / safe-area / dynamic
	   viewport change the math. Only MessageList scrolls internally.
	   :global keeps these scoped to the chat page — /privacy still scrolls. */
	/* stylelint-disable-next-line selector-pseudo-class-no-unknown -- Svelte :global */
	:global(html),
	/* stylelint-disable-next-line selector-pseudo-class-no-unknown -- Svelte :global */
	:global(body) {
		height: 100dvh;
		overflow: hidden;
	}

	.page {
		background: var(--bg);
		display: flex;
		flex-direction: column;
		height: 100dvh;
	}

	.app {
		background: var(--bg);
		display: flex;
		flex: 1;
		flex-direction: column;
		margin: 0 auto;
		min-height: 0;
		width: calc(100% - 4px);
	}

	.action-row {
		border-top: var(--border);
		padding: 12px 24px;
		width: 100%;
	}

	.action-row.chatting {
		padding: 0;
	}

	.brand {
		color: var(--accent);
		font-size: 15px;
		font-weight: 600;
		letter-spacing: -0.01em;
		text-decoration: none;
	}

	.brand:hover {
		color: var(--accent-soft);
	}

	.chat-row {
		align-items: stretch;
		border-top: var(--border);
		display: flex;
		gap: 0;
	}

	.confirm-banner {
		background: var(--accent-tint);
		border-top: 1px solid var(--accent);
		color: var(--accent);
		font-size: 13px;
		padding: 8px 24px;
		text-align: center;
	}

	.confirm-banner kbd {
		background: var(--bg-card);
		border: 1px solid var(--accent);
		font-family: var(--font-mono);
		font-size: 11px;
		padding: 0 6px;
	}

	.msg-input {
		flex: 1;
		min-width: 0;
	}

	.chat-row .stop {
		border-radius: 0;
		border-right: var(--border);
		font-size: 13px;
		padding: 8px 18px;
	}

	.chat-row .stop.armed {
		background: var(--accent);
		border-color: var(--accent);
		color: #fff;
	}

	.heart {
		color: var(--accent);
		font-size: 14px;
		opacity: 0.7;
	}

	.lobby-row {
		align-items: stretch;
		display: flex;
		gap: 12px;
	}

	.muted {
		color: var(--muted);
	}

	.change-tags {
		font-size: 13px;
		padding: 8px 14px;
	}

	.start {
		flex-shrink: 0;
		padding: 8px 28px;
	}

	.tagline {
		color: var(--muted);
		font-size: 13px;
	}

	.searching-row {
		align-items: center;
		display: flex;
		gap: 16px;
		justify-content: space-between;
		padding: 6px 0;
	}

	.searching-row button {
		font-size: 13px;
		padding: 6px 14px;
	}

	.report {
		background: transparent;
		border: var(--border);
		color: var(--muted);
		font-size: 12px;
		font-weight: 500;
		margin-left: auto;
		padding: 4px 12px;
	}

	.report:hover {
		background: #dc2626;
		border-color: #dc2626;
		color: #fff;
	}

	.topbar {
		align-items: baseline;
		border-bottom: var(--border);
		display: flex;
		gap: 10px;
		padding: 10px 24px;
	}

	.typing-row {
		padding: 0 24px 4px;
	}

	.typing-row.away {
		color: var(--muted);
		font-size: 13px;
	}

	@media (width <= 640px) {
		.topbar {
			padding: 8px 16px;
		}

		/* The home-indicator clearance used to live on .app, which left a
		   ~34px cream stripe below the input on notched iPhones. Instead,
		   absorb the safe-area into the action-row so the input itself
		   reaches the bottom edge. */
		.action-row {
			padding: 10px 16px calc(10px + env(safe-area-inset-bottom));
		}

		.action-row.chatting {
			/* Match the textarea so the safe-area zone reads as input bar,
			   not a strange empty strip. */
			background: var(--bg-card);
			padding: 0 0 env(safe-area-inset-bottom);
		}

		.confirm-banner {
			padding: 8px 16px;
		}

		.typing-row {
			padding: 0 16px 4px;
		}
	}
</style>
