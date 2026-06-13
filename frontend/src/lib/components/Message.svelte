<script lang="ts">
	import { linkify } from '$lib/format';
	import type { ChatMessage } from '$lib/store';

	const { msg, hideLabel = false }: { msg: ChatMessage; hideLabel?: boolean } = $props();
	const segments = $derived(linkify(msg.text));
</script>

<div class="msg msg-{msg.from}">
	{#if msg.from === 'system'}
		<em class="system">{msg.text}</em>
	{:else}
		{#if !hideLabel}
			<strong class="who" class:stranger={msg.from === 'stranger'} class:you={msg.from === 'you'}
				>{msg.from === 'you' ? 'You:' : 'Stranger:'}</strong
			>
		{/if}
		<span class="body">
			{#each segments as seg, i (i)}
				{#if seg.kind === 'link'}
					<a href={seg.value} rel="noopener noreferrer" target="_blank">{seg.value}</a>
				{:else}
					{seg.value}
				{/if}
			{/each}
		</span>
	{/if}
</div>

<style>
	.msg {
		margin-bottom: 2px;
		overflow-wrap: anywhere;
		user-select: text;
		word-break: normal;
	}

	.who {
		font-weight: 500;
		margin-right: 6px;
	}

	.who.you {
		color: var(--accent);
	}

	.who.stranger {
		color: var(--fg);
	}

	.body a {
		word-break: break-all;
	}

	.system {
		color: var(--muted);
		font-size: 13px;
		font-style: italic;
	}
</style>
