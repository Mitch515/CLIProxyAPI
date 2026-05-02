import { tokenStore } from '$lib/stores/token.svelte';

export class ApiError extends Error {
  constructor(public status: number, message: string) { super(message); }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers ?? {});
  const tok = tokenStore.value;
  if (tok) headers.set('Authorization', `Bearer ${tok}`);
  if (!headers.has('Content-Type') && init.body) {
    headers.set('Content-Type', 'application/json');
  }

  const res = await fetch(path, { ...init, headers });
  if (res.status === 401) {
    tokenStore.clear();
    throw new ApiError(401, 'unauthorized');
  }
  if (!res.ok) {
    const body = await res.text().catch(() => '');
    throw new ApiError(res.status, body || res.statusText);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export const api = {
  get:    <T>(path: string)             => request<T>(path),
  post:   <T>(path: string, body?: any) => request<T>(path, { method: 'POST', body: body ? JSON.stringify(body) : undefined }),
  patch:  <T>(path: string, body: any)  => request<T>(path, { method: 'PATCH', body: JSON.stringify(body) }),
  del:    <T>(path: string)             => request<T>(path, { method: 'DELETE' })
};
