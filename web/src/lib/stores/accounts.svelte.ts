import { accountsApi } from '$lib/api/accounts';
import type { Account, PatchAccountBody } from '$lib/api/types';

class AccountsStore {
  byId = $state<Map<string, Account>>(new Map());
  loading = $state(true);
  error = $state<string | null>(null);

  list = $derived.by(() => {
    const arr = Array.from(this.byId.values());
    arr.sort((a, b) => {
      const p = (a.provider ?? '').localeCompare(b.provider ?? '');
      if (p !== 0) return p;
      return (a.label ?? a.email ?? a.id).localeCompare(b.label ?? b.email ?? b.id);
    });
    return arr;
  });

  async refresh() {
    this.loading = true;
    this.error = null;
    try {
      const res = await accountsApi.list();
      const next = new Map<string, Account>();
      for (const a of res.accounts) next.set(a.id, a);
      this.byId = next;
    } catch (e: any) {
      this.error = e?.message ?? 'failed to load accounts';
    } finally {
      this.loading = false;
    }
  }

  async refreshOne(id: string) {
    try {
      const a = await accountsApi.get(id);
      this.upsert(a);
    } catch {
      /* ignore — likely just deleted */
    }
  }

  upsert(a: Account) {
    const next = new Map(this.byId);
    next.set(a.id, a);
    this.byId = next;
  }

  async patch(id: string, body: PatchAccountBody) {
    const previous = this.byId.get(id);
    if (previous) {
      this.upsert({ ...previous, ...body, warmup_model: body.warmup_model ?? previous.warmup_model });
    }
    try {
      const updated = await accountsApi.patch(id, body);
      this.upsert(updated);
    } catch (e) {
      if (previous) this.upsert(previous);
      throw e;
    }
  }

  async fireWarmup(id: string) {
    await accountsApi.warmup(id);
    await this.refreshOne(id);
  }

  async fireWarmupAll() {
    return await accountsApi.warmupAll();
  }

  async autoDetect(id: string) {
    const result = await accountsApi.autoDetect(id);
    await this.refreshOne(id);
    return result;
  }

  async remove(id: string) {
    await accountsApi.delete(id);
    const next = new Map(this.byId);
    next.delete(id);
    this.byId = next;
  }
}

export const accountsStore = new AccountsStore();
