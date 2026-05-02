import { api } from './client';
import type {
  Account,
  AccountsResponse,
  HistoryResponse,
  PatchAccountBody,
  SSETicketResponse,
  WindowKey
} from './types';

export const accountsApi = {
  list: () => api.get<AccountsResponse>('/v0/management/accounts'),
  get:  (id: string) => api.get<Account>(`/v0/management/accounts/${encodeURIComponent(id)}`),
  patch: (id: string, body: PatchAccountBody) =>
    api.patch<Account>(`/v0/management/accounts/${encodeURIComponent(id)}`, body),
  history: (id: string, window: WindowKey, since?: string, limit = 500) => {
    const params = new URLSearchParams({ window, limit: String(limit) });
    if (since) params.set('since', since);
    return api.get<HistoryResponse>(`/v0/management/accounts/${encodeURIComponent(id)}/history?${params}`);
  },
  warmup: (id: string) =>
    api.post<{ fired: boolean; last: any }>(`/v0/management/accounts/${encodeURIComponent(id)}/warmup`),
  warmupAll: () =>
    api.post<{ results: Array<{ id: string; ok: boolean; last?: any; error?: string }> }>(
      '/v0/management/accounts/warmup-all'
    ),
  autoDetect: (id: string) =>
    api.post<{ winner: string; token_expired: boolean; attempts: Array<{ model: string; ok: boolean; error?: string }> }>(
      `/v0/management/accounts/${encodeURIComponent(id)}/auto-detect`
    ),
  delete: (id: string) =>
    api.del<{ deleted: boolean }>(`/v0/management/accounts/${encodeURIComponent(id)}`),
  sseTicket: () => api.post<SSETicketResponse>('/v0/management/sse-ticket')
};
