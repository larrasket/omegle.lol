<script lang="ts">
	import { tick } from 'svelte';

	import type { ChatStore } from '$lib/store';

	const { store }: { store: ChatStore } = $props();
	let text = $state('');
	let typingTimeout: ReturnType<typeof setTimeout> | null = null;
	let lastTypingState = false;
	let textareaEl: HTMLTextAreaElement | undefined = $state();

	const MAX_HEIGHT_PX = 132; // 6 lines × 22 px

	function autoGrow(): void {
		if (textareaEl === undefined) return;
		textareaEl.style.height = 'auto';
		textareaEl.style.height = `${String(Math.min(textareaEl.scrollHeight, MAX_HEIGHT_PX))}px`;
	}

	function vibrate(ms: number): void {
		if (typeof navigator === 'undefined') return;
		if (!('vibrate' in navigator)) return;
		(navigator.vibrate as (ms: number) => boolean)(ms);
	}

	$effect(() => {
		void tick().then(() => {
			textareaEl?.focus();
			// iOS Safari's `autocorrect` attribute is non-standard and unknown to TS;
			// set it imperatively so the on-screen keyboard substitutes typos.
			textareaEl?.setAttribute('autocorrect', 'on');
			autoGrow();
		});
	});

	function setTyping(active: boolean): void {
		if (active === lastTypingState) return;
		lastTypingState = active;
		store.setTyping(active);
	}

	function onInput(): void {
		autoGrow();
		setTyping(true);
		if (typingTimeout !== null) clearTimeout(typingTimeout);
		typingTimeout = setTimeout(() => {
			setTyping(false);
		}, 1500);
	}

	function send(): void {
		if (!text.trim()) return;
		store.sendMessage(text);
		text = '';
		vibrate(8);
		setTyping(false);
		if (typingTimeout !== null) clearTimeout(typingTimeout);
		void tick().then(() => {
			autoGrow();
		});
	}

	function onKey(e: KeyboardEvent): void {
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			send();
		}
	}
</script>

<form
	class="input"
	onsubmit={(e) => {
		e.preventDefault();
		send();
	}}
>
	<textarea
		bind:this={textareaEl}
		aria-label="Message"
		autocapitalize="sentences"
		autocomplete="off"
		enterkeyhint="send"
		oninput={onInput}
		onkeydown={onKey}
		placeholder="Type a message…"
		rows="1"
		spellcheck="true"
		bind:value={text}
	></textarea>
	<button class="send" disabled={!text.trim()} type="submit">Send</button>
</form>

<style>
	.input {
		display: flex;
		width: 100%;
	}

	.send {
		border: none;
		border-left: var(--border);
	}

	@media (hover: hover) and (pointer: fine) {
		.send {
			display: none;
		}
	}

	textarea {
		border: none;
		flex: 1;

		/* 16px (not inherit) so iOS doesn't auto-zoom on focus. Svelte's
		   scoped-CSS specificity beats the app.css mobile @media override,
		   so we have to set the right value directly here. */
		font-size: 16px;
		line-height: 22px;
		max-height: 132px;
		min-height: 44px;
		overflow-y: auto;
		padding: 11px 12px;
		resize: none;
	}

	textarea:focus-visible {
		border: none;
		outline: 2px solid var(--accent);
		outline-offset: -2px;
	}
</style>
