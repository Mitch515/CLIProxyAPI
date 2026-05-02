// Single-instance reactive token store. Hydrates from localStorage on first
// access in the browser. Used by the API client and the +layout guard.

const STORAGE_KEY = 'cliproxy.mgmt-token';

class TokenStore {
  #value = $state<string>('');
  #hydrated = false;

  get value(): string {
    if (!this.#hydrated && typeof localStorage !== 'undefined') {
      this.#value = localStorage.getItem(STORAGE_KEY) ?? '';
      this.#hydrated = true;
    }
    return this.#value;
  }

  set(v: string) {
    this.#value = v;
    if (typeof localStorage !== 'undefined') {
      if (v) localStorage.setItem(STORAGE_KEY, v);
      else localStorage.removeItem(STORAGE_KEY);
    }
  }

  clear() { this.set(''); }
  get hasToken(): boolean { return Boolean(this.value); }
}

export const tokenStore = new TokenStore();
