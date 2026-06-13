<footer>
	<span
		>made with love <span class="heart" aria-hidden="true">♡</span> ·
		<a href="/privacy">privacy</a></span
	>
	<span class="shortcuts">Enter to send · Esc to stop · Esc again to find someone</span>
</footer>

<style>
	footer {
		align-items: center;
		border-top: var(--border);
		color: var(--muted);
		display: flex;
		flex-wrap: wrap;
		font-size: 12px;
		gap: 16px;
		justify-content: space-between;
		padding: 12px 24px;
	}

	.heart {
		color: var(--accent);
	}

	.shortcuts {
		color: var(--muted);
	}

	@media (width <= 640px) {
		footer {
			display: none;
		}
	}
</style>
