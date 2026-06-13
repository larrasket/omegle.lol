// Privacy is static — opt into SSR + prerender so the policy text is baked
// into the HTML at build time and survives without JS (crawlers, curl,
// readers, etc.). The rest of the site stays SPA-only via the +layout.ts
// `ssr = false`; this page overrides per-route.
export const prerender = true;
export const ssr = true;
