export type WindowKey = '5h' | '7d';

export interface WindowState {
  pct_used: number;
  tokens_used?: number;
  tokens_limit?: number;
  reset_at?: string; // ISO8601
  exhausted: boolean;
  observed_at?: string;
}

export interface WarmupRecord {
  fired_at: string;
  trigger: string;
  model: string;
  ok: boolean;
  error?: string;
}

export interface SubscriptionInfo {
  plan_type?: string;
  active_start?: string;
  active_until?: string;
  last_checked?: string;
  expired: boolean;
}

export interface Account {
  id: string;
  provider: string;
  email?: string;
  label?: string;
  status: 'active' | 'disabled' | 'expired' | string;
  warmup_enabled: boolean;
  warmup_model?: string;
  windows: Record<WindowKey, WindowState>;
  created_at?: string;
  updated_at?: string;
  last_warmup?: WarmupRecord | null;
  subscription?: SubscriptionInfo;
}

export interface AccountsResponse { accounts: Account[]; }

export interface HistoryPoint {
  t: string;
  pct: number;
  tokens?: number;
  reset_at?: string;
  source: 'header' | 'error_body' | string;
}

export interface HistoryResponse {
  window: WindowKey;
  points: HistoryPoint[];
}

export interface PatchAccountBody {
  label?: string;
  warmup_enabled?: boolean;
  warmup_model?: string;
}

export interface SSETicketResponse {
  ticket: string;
  expires_at: string;
}
