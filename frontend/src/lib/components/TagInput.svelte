<script lang="ts">
	// `let` is required so `tags` (which IS reassigned) and `placeholder` can coexist in one $props().
	// $bindable() is a Svelte 5 rune, not a real default value — suppress the false positive for the whole block.
	/* eslint-disable @typescript-eslint/no-useless-default-assignment */
	let {
		tags = $bindable(),
		placeholder = 'Add interests',
		input = $bindable<HTMLInputElement | undefined>(undefined)
	}: { tags: string[]; placeholder?: string; input?: HTMLInputElement | undefined } = $props();
	/* eslint-enable @typescript-eslint/no-useless-default-assignment */
	let draft = $state('');

	function commit() {
		const t = draft.trim().toLowerCase();
		draft = '';
		if (!t) return;
		if (tags.includes(t)) return;
		if (tags.length >= 10) return;
		tags = [...tags, t];
	}

	function remove(t: string) {
		tags = tags.filter((x) => x !== t);
	}

	function onKey(e: KeyboardEvent) {
		if (e.key === 'Enter' || e.key === ',' || e.key === ' ') {
			e.preventDefault();
			commit();
		} else if (e.key === 'Backspace' && draft === '' && tags.length > 0) {
			tags = tags.slice(0, -1);
		}
	}
</script>

<div class="tag-input">
	<div class="chips">
		{#each tags as t (t)}
			<span class="chip">
				{t}
				<button
					class="x"
					aria-label="Remove {t}"
					onclick={() => {
						remove(t);
					}}
					type="button">×</button
				>
			</span>
		{/each}
		<input
			bind:this={input}
			aria-label="Add interest tag"
			onblur={commit}
			onkeydown={onKey}
			{placeholder}
			type="text"
			bind:value={draft}
		/>
	</div>
</div>

<style>
	.tag-input {
		background: var(--bg-card);
		border: var(--border);
		padding: 7px 10px;
		transition: border-color 0.12s ease;
	}

	.tag-input:focus-within {
		border-color: var(--accent);
	}

	.chips {
		align-items: center;
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}

	.chip {
		align-items: center;
		background: var(--accent-tint);
		border: 1px solid var(--accent);
		color: var(--accent);
		display: inline-flex;
		font-size: 13px;
		gap: 4px;
		padding: 2px 4px 2px 10px;
	}

	.chip .x {
		all: unset;
		color: var(--accent);
		cursor: pointer;
		font-size: 16px;
		font-weight: 400;
		line-height: 1;
		padding: 0 4px;
	}

	.chip .x:hover {
		color: var(--accent-soft);
	}

	.chip .x:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 1px;
	}

	input[type='text'] {
		background: transparent;
		border: none;
		flex: 1;
		min-width: 140px;
		padding: 4px 0;
	}

	input[type='text']:focus-visible {
		border: none;
		padding: 4px 0;
	}
</style>
