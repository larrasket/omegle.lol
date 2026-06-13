<script lang="ts">
	// Threshold below which we don't show the count. An empty-looking
	// landing page ("1 online") is worse than no signal at all — better
	// to stay quiet until there's real density to show.
	const SHOW_AT_LEAST = 5;

	const { onlineCount = 0 }: { onlineCount?: number } = $props();
</script>

<section class="intro">
	{#if onlineCount >= SHOW_AT_LEAST}
		<p class="online" role="status">
			<span class="dot" aria-hidden="true"></span>
			{onlineCount} people online right now
		</p>
	{/if}
	<p class="lede">
		Non-bs Omegle, no ads, no creepy paywalls, no “premium chat” bait, no broken moderation, no
		auto-ban, no nonesense.
	</p>
	<p class="body">
		Omegle was the smallest possible meeting between people, where you would meet someone you'll
		never see again. It was simple, unreliable in human ways, and somehow that was good enough. Then
		it disappeared, and a swarm of vibe-coded clones replaced it, stuffed with ads, scams, dark
		patterns, and “premium” nonsense trying to extract money from loneliness. They copied the
		surface of it without understanding why people were there in the first place. This is a rebuilt
		by hand.
	</p>
</section>

<style>
	.body {
		color: var(--muted);
		font-size: 14px;
		line-height: 1.55;
		max-width: 70ch;
	}

	.intro {
		border-bottom: var(--border);
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 20px 24px;
	}

	.lede {
		font-size: 15px;
		font-weight: 500;
	}

	.online {
		align-items: center;
		color: var(--muted);
		display: inline-flex;
		font-size: 13px;
		font-variant-numeric: tabular-nums;
		gap: 6px;
	}

	.dot {
		background: #16a34a;
		display: inline-block;
		height: 8px;
		width: 8px;
	}

	@media (width <= 640px) {
		.intro {
			display: none;
		}
	}
</style>
