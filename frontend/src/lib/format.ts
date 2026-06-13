import type { ChatMessage } from './store';

/** URL pattern: http(s) only, conservative (no protocol-relative, no bare domains). */
const URL_RE = /\bhttps?:\/\/[^\s<>"]+/gi;

export type Segment = { kind: 'text'; value: string } | { kind: 'link'; value: string };

/** Split a message string into text and link segments. */
export function linkify(text: string): Segment[] {
	const out: Segment[] = [];
	let lastEnd = 0;
	for (const match of text.matchAll(URL_RE)) {
		const start = match.index;
		if (start > lastEnd) {
			out.push({ kind: 'text', value: text.slice(lastEnd, start) });
		}
		out.push({ kind: 'link', value: match[0] });
		lastEnd = start + match[0].length;
	}
	if (lastEnd < text.length) {
		out.push({ kind: 'text', value: text.slice(lastEnd) });
	}
	return out;
}

export type Group =
	| { kind: 'group'; from: ChatMessage['from']; lines: ChatMessage[]; key: string }
	| { kind: 'separator'; label: string; key: string };

const GROUP_GAP_MS = 5 * 60 * 1000;

function formatSeparator(ts: number): string {
	const d = new Date(ts);
	const now = new Date();
	const sameDay =
		d.getFullYear() === now.getFullYear() &&
		d.getMonth() === now.getMonth() &&
		d.getDate() === now.getDate();
	const time = d.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
	return sameDay
		? time
		: `${d.toLocaleDateString([], { month: 'short', day: 'numeric' })} · ${time}`;
}

/** Group consecutive same-sender messages within GROUP_GAP_MS. Emit a separator
 *  before the first group AND before any group whose start time is > GROUP_GAP_MS
 *  after the previous group's last message. */
export function groupMessages(messages: ChatMessage[]): Group[] {
	const out: Group[] = [];
	let i = 0;
	while (i < messages.length) {
		const first = messages[i];
		if (first === undefined) break;
		const groupStart = i;
		let j = i + 1;
		while (j < messages.length) {
			const next = messages[j];
			const prev = messages[j - 1];
			if (next === undefined || prev === undefined) break;
			if (next.from !== first.from) break;
			if (next.ts - prev.ts > GROUP_GAP_MS) break;
			j++;
		}
		const lines = messages.slice(groupStart, j);

		let needsSeparator = out.length === 0;
		if (!needsSeparator) {
			const lastGroup = out[out.length - 1];
			if (lastGroup?.kind === 'group') {
				const lastLine = lastGroup.lines[lastGroup.lines.length - 1];
				if (lastLine !== undefined && first.ts - lastLine.ts > GROUP_GAP_MS) {
					needsSeparator = true;
				}
			}
		}
		if (needsSeparator) {
			out.push({
				kind: 'separator',
				label: formatSeparator(first.ts),
				key: `sep-${String(first.ts)}`
			});
		}
		out.push({
			kind: 'group',
			from: first.from,
			lines,
			key: `grp-${String(first.ts)}-${first.from}`
		});
		i = j;
	}
	return out;
}
