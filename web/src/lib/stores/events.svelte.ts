import { accountsApi } from '$lib/api/accounts';
import { accountsStore } from './accounts.svelte';
import { toastStore } from './toasts.svelte';

type Status = 'connecting' | 'open' | 'closed' | 'error';

class EventsStore {
  status = $state<Status>('closed');
  #es: EventSource | null = null;
  #backoffMs = 1000;
  #stopped = false;

  async start() {
    if (typeof EventSource === 'undefined') return;
    this.#stopped = false;
    await this.#connect();
  }

  stop() {
    this.#stopped = true;
    if (this.#es) {
      this.#es.close();
      this.#es = null;
    }
    this.status = 'closed';
  }

  async #connect() {
    if (this.#stopped) return;
    this.status = 'connecting';
    try {
      const { ticket } = await accountsApi.sseTicket();
      const es = new EventSource(`/v0/management/events?ticket=${encodeURIComponent(ticket)}`);
      this.#es = es;

      es.addEventListener('open', () => {
        this.status = 'open';
        this.#backoffMs = 1000;
      });

      es.addEventListener('account.changed', (ev: MessageEvent) => {
        try {
          const data = JSON.parse(ev.data);
          if (data?.id) accountsStore.refreshOne(data.id);
        } catch { /* ignore parse */ }
      });

      es.addEventListener('warmup.fired', (ev: MessageEvent) => {
        try {
          const data = JSON.parse(ev.data);
          const ok = data?.record?.ok ?? false;
          const model = data?.record?.model ?? '';
          toastStore.push({
            kind: ok ? 'success' : 'error',
            title: ok ? 'Warmup fired' : 'Warmup failed',
            body: ok ? `Pinged ${model}` : `Tried ${model}: ${data?.record?.error ?? 'unknown'}`
          });
          if (data?.id) accountsStore.refreshOne(data.id);
        } catch { /* ignore */ }
      });

      es.addEventListener('error', () => {
        this.status = 'error';
        es.close();
        this.#es = null;
        if (this.#stopped) return;
        const wait = this.#backoffMs;
        this.#backoffMs = Math.min(this.#backoffMs * 2, 30_000);
        setTimeout(() => this.#connect(), wait);
      });
    } catch {
      this.status = 'error';
      const wait = this.#backoffMs;
      this.#backoffMs = Math.min(this.#backoffMs * 2, 30_000);
      setTimeout(() => this.#connect(), wait);
    }
  }
}

export const eventsStore = new EventsStore();
