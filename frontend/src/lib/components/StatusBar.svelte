<script lang="ts">
	import { fromStore } from 'svelte/store';

	import type { ChatStore } from '$lib/store';

	const { store }: { store: ChatStore } = $props();
	const snap = $derived(fromStore(store));
</script>

{#if snap.current.state === 'connecting'}
	<header class="status">
		<span class="muted">Connecting<span class="dots"></span></span>
	</header>
{:else if snap.current.state === 'peer-left'}
	<header class="status">
		<span class="muted">They left.</span>
	</header>
{/if}

<style>
	.muted {
		color: var(--muted);
	}

	.status {
		align-items: center;
		border-bottom: var(--border);
		color: var(--muted);
		display: flex;
		font-size: 13px;
		gap: 16px;
		padding: 10px 24px;
		width: 100%;
	}

	@media (width <= 640px) {
		.status {
			padding: 8px 16px;
		}
	}
</style>
