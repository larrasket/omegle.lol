<svelte:head>
	<title>omegle.lol · privacy</title>
	<meta
		name="description"
		content="omegle.lol privacy policy — what we don't collect and what we briefly process."
	/>
</svelte:head>

<main class="wrap">
	<header class="bar">
		<a class="brand" href="/">omegle.lol</a>
		<span class="sep">·</span>
		<span class="title">privacy</span>
	</header>

	<article class="prose">
		<p class="lede">
			omegle.lol is anonymous text chat. We try to collect as little as possible, and what we do
			touch we don't keep. This page explains, plainly, what happens when you use the site.
		</p>

		<h2>What we don't store</h2>
		<p>
			We don't have user accounts. We don't store your chat messages anywhere — every message lives
			only in memory on the server, for as long as you and your partner are both connected. When
			either of you disconnects, the conversation is gone forever. There is no backup, no log, no
			archive.
		</p>

		<h2>What we briefly process</h2>
		<ul>
			<li>
				<strong>Your IP address</strong>, briefly, to rate-limit connection attempts and prevent
				abuse. It stays in memory, not on disk, and is forgotten when you leave.
			</li>
			<li>
				<strong>The tags you enter</strong>, briefly, to match you with someone who shares an
				interest. Tags are discarded the moment you leave the queue.
			</li>
			<li>
				<strong>An anonymous session ID</strong> — a random string that exists only as long as your WebSocket
				connection is open. It is never tied to anything that could identify you.
			</li>
		</ul>

		<h2>Cookies</h2>
		<p>
			We don't set any cookies. Our hosting provider, Google Cloud Run, sets a load-balancer cookie
			called <code>GAESA</code> to keep your connection on a single instance. It is opaque, doesn't identify
			you, and we never read it.
		</p>

		<h2>Third parties</h2>
		<p>
			The service runs on Google Cloud Run in the United States. When you connect, Google's
			infrastructure handles TLS termination and routing on our behalf. We don't share your data
			with anyone else, and we don't run analytics, ads, or trackers.
		</p>

		<h2>Children</h2>
		<p>
			omegle.lol is intended for adults. We don't knowingly collect information from anyone under
			18, and we don't recommend the service for minors.
		</p>

		<h2>Changes</h2>
		<p>
			If this policy ever changes in a way that materially affects how we handle data, we'll update
			the date below and replace this page. There is no mailing list, so please check back if you
			care.
		</p>

		<h2>Contact</h2>
		<p>
			Questions or concerns: <a href="mailto:cochlea.setting9o@icloud.com"
				>cochlea.setting9o@icloud.com</a
			>.
		</p>

		<p class="updated">Last updated: 2026-05-25.</p>
	</article>
</main>

<style>
	.wrap {
		display: flex;
		flex: 1;
		flex-direction: column;
		gap: 18px;
		margin: 0 auto;
		max-width: 720px;
		padding: 16px 18px 48px;
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

	.prose {
		color: var(--fg);
		display: flex;
		flex-direction: column;
		gap: 14px;
	}

	.prose h2 {
		font-size: 16px;
		font-weight: 600;
		margin-top: 8px;
	}

	.prose p,
	.prose ul {
		color: var(--fg);
		line-height: 1.6;
	}

	.lede {
		color: var(--muted);
	}

	.prose ul {
		list-style: none;
		padding-left: 0;
	}

	.prose li {
		border-left: 2px solid var(--accent);
		margin: 6px 0;
		padding: 2px 0 2px 12px;
	}

	.prose li + li {
		margin-top: 8px;
	}

	.prose code {
		background: var(--accent-tint);
		font-family: var(--font-mono);
		font-size: 13px;
		padding: 1px 4px;
	}

	.updated {
		color: var(--muted);
		font-size: 13px;
		margin-top: 12px;
	}
</style>
