// Shared 1Hz tick used by every countdown. One setInterval for the whole app
// keeps countdowns in sync and avoids hundreds of independent intervals when
// many <CountdownTimer> instances mount.

class TickStore {
  now = $state(Date.now());
  #handle: ReturnType<typeof setInterval> | null = null;
  #refs = 0;

  attach() {
    this.#refs += 1;
    if (this.#handle) return;
    this.#handle = setInterval(() => { this.now = Date.now(); }, 1000);
  }

  detach() {
    this.#refs -= 1;
    if (this.#refs <= 0 && this.#handle) {
      clearInterval(this.#handle);
      this.#handle = null;
      this.#refs = 0;
    }
  }
}

export const tickStore = new TickStore();
