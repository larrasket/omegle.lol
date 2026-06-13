<script lang="ts">
	import { onDestroy, onMount } from 'svelte';

	interface TagCount {
		tag: string;
		count: number;
	}
	interface Stats {
		active_connections: number;
		active_rooms: number;
		paired_users: number;
		searching: number;
		top_tags: TagCount[];
		uptime_seconds: number;
		started_at: string;
		server_time: string;
	}

	const POLL_MS = 2000;
	const STATS_URL = '/705812da16d3edd0/stats';
	const CREDS_KEY = '_obs';

	let username = $state('admin');
	let password = $state('');
	let authed = $state(false);
	let stats = $state<Stats | null>(null);
	let error = $state<string | null>(null);
	let lastUpdated = $state<Date | null>(null);
	let timer: ReturnType<typeof setTimeout> | null = null;

	function authHeader(): string {
		return `Basic ${btoa(`${username}:${password}`)}`;
	}

	async function fetchStats(): Promise<void> {
		try {
			const res = await fetch(STATS_URL, {
				cache: 'no-store',
				headers: { authorization: authHeader() }
			});
			if (res.status === 401) {
				error = 'Wrong username or password.';
				authed = false;
				sessionStorage.removeItem(CREDS_KEY);
				stop();
				return;
			}
			if (!res.ok) {
				error = `Server returned ${String(res.status)}.`;
				return;
			}
			stats = (await res.json()) as Stats;
			lastUpdated = new Date();
			error = null;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Network error.';
		}
	}

	function schedule(): void {
		if (!authed) return;
		timer = setTimeout(() => {
			void fetchStats().then(schedule);
		}, POLL_MS);
	}

	function stop(): void {
		if (timer !== null) {
			clearTimeout(timer);
			timer = null;
		}
	}

	async function signIn(e: SubmitEvent): Promise<void> {
		e.preventDefault();
		if (!password) return;
		await fetchStats();
		if (stats) {
			authed = true;
			sessionStorage.setItem(CREDS_KEY, JSON.stringify({ username, password }));
			schedule();
		}
	}

	function signOut(): void {
		stop();
		authed = false;
		stats = null;
		password = '';
		sessionStorage.removeItem(CREDS_KEY);
	}

	function formatUptime(s: number): string {
		if (s < 60) return `${String(s)}s`;
		const m = Math.floor(s / 60);
		if (m < 60) return `${String(m)}m ${String(s % 60)}s`;
		const h = Math.floor(m / 60);
		if (h < 24) return `${String(h)}h ${String(m % 60)}m`;
		const d = Math.floor(h / 24);
		return `${String(d)}d ${String(h % 24)}h`;
	}

	function formatTime(d: Date | null): string {
		if (!d) return '—';
		return d.toLocaleTimeString();
	}

	onMount(async () => {
		const raw = sessionStorage.getItem(CREDS_KEY);
		if (raw !== null && raw !== '') {
			try {
				const parsed = JSON.parse(raw) as { username: string; password: string };
				username = parsed.username;
				password = parsed.password;
				await fetchStats();
				if (stats) {
					authed = true;
					schedule();
				}
			} catch {
				sessionStorage.removeItem(CREDS_KEY);
			}
		}
	});

	onDestroy(stop);
</script>

<svelte:head>
	<title>omegle.lol</title>
	<meta name="robots" content="noindex, nofollow" />
</svelte:head>

<main class="wrap">
	<header class="bar">
		<a class="brand" href="/">omegle.lol</a>
		<span class="sep">·</span>
		<span class="title">admin</span>
		{#if authed}
			<span class="live">
				live <span class="dots"></span>
			</span>
			<button class="outlined small" onclick={signOut} type="button">sign out</button>
		{/if}
	</header>

	{#if !authed}
		<form class="signin" onsubmit={signIn}>
			<label>
				<span>username</span>
				<input autocomplete="username" type="text" bind:value={username} />
			</label>
			<label>
				<span>password</span>
				<input autocomplete="current-password" required type="password" bind:value={password} />
			</label>
			<button disabled={!password} type="submit">sign in</button>
			{#if error}
				<p class="error">{error}</p>
			{/if}
		</form>
	{:else if stats}
		<section class="grid">
			<article class="card">
				<h2>connections</h2>
				<p class="big">{stats.active_connections}</p>
				<p class="sub">WebSockets currently open</p>
			</article>
			<article class="card">
				<h2>paired</h2>
				<p class="big">{stats.paired_users}</p>
				<p class="sub">{stats.active_rooms} active {stats.active_rooms === 1 ? 'chat' : 'chats'}</p>
			</article>
			<article class="card">
				<h2>searching</h2>
				<p class="big">{stats.searching}</p>
				<p class="sub">in the matcher queue</p>
			</article>
			<article class="card">
				<h2>uptime</h2>
				<p class="big mono">{formatUptime(stats.uptime_seconds)}</p>
				<p class="sub">since {new Date(stats.started_at).toLocaleString()}</p>
			</article>
		</section>

		<section class="tags">
			<h2>top tags</h2>
			{#if stats.top_tags.length === 0}
				<p class="empty">Nobody is searching right now.</p>
			{:else}
				<ol>
					{#each stats.top_tags as t (t.tag)}
						<li>
							<span class="tag">{t.tag}</span>
							<span class="count">{t.count}</span>
						</li>
					{/each}
				</ol>
			{/if}
		</section>

		<footer class="foot">
			Last refresh {formatTime(lastUpdated)} · refreshes every {String(POLL_MS / 1000)}s
			{#if error}
				<span class="error inline">· {error}</span>
			{/if}
		</footer>
	{:else}
		<p class="loading">loading<span class="dots"></span></p>
	{/if}
</main>

<style>
	.wrap {
		display: flex;
		flex: 1;
		flex-direction: column;
		gap: 18px;
		margin: 0 auto;
		max-width: 920px;
		padding: 16px 18px 32px;
		width: 100%;
	}

	.bar {
		align-items: center;
		border-bottom: var(--border-soft);
		display: flex;
		gap: 8px;
		padding-bottom: 12px;
	}

	.brand {
		color: var(--fg);
		font-weight: 600;
		text-decoration: none;
	}

	.brand:hover {
		color: var(--accent);
	}

	.sep {
		color: var(--muted);
	}

	.title {
		color: var(--muted);
	}

	.live {
		color: var(--accent);
		font-size: 13px;
		margin-left: auto;
	}

	.small {
		font-size: 13px;
		padding: 5px 12px;
	}

	.signin {
		display: flex;
		flex-direction: column;
		gap: 12px;
		max-width: 320px;
	}

	.signin label {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.signin label span {
		color: var(--muted);
		font-size: 13px;
	}

	.error {
		color: #b91c1c;
		font-size: 13px;
	}

	.error.inline {
		color: #b91c1c;
	}

	.grid {
		display: grid;
		gap: 12px;
		grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
	}

	.card {
		background: var(--bg-card);
		border: var(--border);
		display: flex;
		flex-direction: column;
		gap: 4px;
		padding: 14px 16px 16px;
	}

	.card h2 {
		color: var(--muted);
		font-size: 12px;
		font-weight: 500;
		letter-spacing: 0.04em;
		text-transform: uppercase;
	}

	.big {
		color: var(--fg);
		font-size: 32px;
		font-weight: 600;
		letter-spacing: -0.02em;
		line-height: 1.1;
	}

	.sub {
		color: var(--muted);
		font-size: 13px;
	}

	.tags {
		background: var(--bg-card);
		border: var(--border);
		padding: 14px 16px;
	}

	.tags h2 {
		color: var(--muted);
		font-size: 12px;
		font-weight: 500;
		letter-spacing: 0.04em;
		margin-bottom: 10px;
		text-transform: uppercase;
	}

	.tags ol {
		display: flex;
		flex-direction: column;
		gap: 4px;
		list-style: none;
	}

	.tags li {
		align-items: center;
		display: flex;
		justify-content: space-between;
		padding: 4px 0;
	}

	.tag {
		color: var(--fg);
	}

	.count {
		color: var(--accent);
		font-variant-numeric: tabular-nums;
		font-weight: 500;
	}

	.empty {
		color: var(--muted);
		font-size: 13px;
	}

	.foot {
		color: var(--muted);
		font-size: 12px;
		padding-top: 8px;
	}

	.loading {
		color: var(--muted);
	}
</style>
