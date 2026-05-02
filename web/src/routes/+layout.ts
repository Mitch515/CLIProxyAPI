// Universal layout load. Runs on both server (during build) and client.
// Disables SSR so the SPA can boot inside the embedded static bundle.

export const ssr = false;
export const prerender = false;
export const trailingSlash = 'never';
