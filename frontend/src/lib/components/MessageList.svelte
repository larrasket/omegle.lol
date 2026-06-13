<script lang="ts">
	import { tick } from 'svelte';

	import Message from './Message.svelte';

	import type { ChatMessage } from '$lib/store';

	const { messages }: { messages: ChatMessage[] } = $props();

	let container: HTMLElement | undefined = $state();
	let lastLen = 0;

	const NEAR_BOTTOM_PX = 80;

	function isNearBottom(): boolean {
		if (container === undefined) return true;
		return container.scrollHeight - container.scrollTop - container.clientHeight <= NEAR_BOTTOM_PX;
	}

	function scrollToBottom(smooth: boolean): void {
		if (container === undefined) return;
		container.scrollTo({
			top: container.scrollHeight,
			behavior: smooth ? 'smooth' : 'auto'
		});
	}

	// On a new message: auto-scroll only if the user is already near the
	// bottom. If they've scrolled up to read history, we leave them alone
	// — no overlay, no popup. They can scroll down manually whenever.
	$effect(() => {
		const len = messages.length;
		if (len <= lastLen) {
			lastLen = len;
			return;
		}
		lastLen = len;
		void tick().then(() => {
			if (container === undefined) return;
			if (isNearBottom()) scrollToBottom(true);
		});
	});
</script>

<div class="wrap">
	<div bind:this={container} class="list" aria-atomic="false" aria-live="polite" role="log">
		{#each messages as msg, i (i)}
			<Message {msg} />
		{/each}
	</div>
</div>

<style>
	.wrap {
		display: flex;
		flex: 1;
		flex-direction: column;
		min-height: 0;
		position: relative;
	}

	/* Input sits below the list on both desktop and mobile, so messages
	   anchor to the bottom — newest line always sits just above the input.
	   Inner scrollbar is hidden; the chrome looks noisy against the minimal
	   palette and the typing context already cues "more above". */
	.list {
		display: flex;
		flex: 1;
		flex-direction: column;
		justify-content: flex-end;
		overflow-y: auto;
		overscroll-behavior: contain;
		padding: 12px 24px;
		scrollbar-width: none;
		width: 100%;
	}

	.list::-webkit-scrollbar {
		display: none;
	}

	@media (width <= 640px) {
		.list {
			padding: 12px 16px;
		}
	}
</style>
