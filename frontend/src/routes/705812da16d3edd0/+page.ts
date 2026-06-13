// Admin dashboard is fully client-rendered: it polls a private endpoint
// with credentials held in sessionStorage. Pre-rendering it would leak
// nothing (no auth state at build time) but is also pointless, so just
// skip SSR/prerender for clarity.
export const prerender = false;
export const ssr = false;
