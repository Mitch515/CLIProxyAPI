export interface Toast {
  id: number;
  kind: 'success' | 'error' | 'info';
  title: string;
  body?: string;
}

class ToastStore {
  items = $state<Toast[]>([]);
  #seq = 0;

  push(t: Omit<Toast, 'id'>) {
    const id = ++this.#seq;
    this.items = [...this.items, { id, ...t }];
    setTimeout(() => this.dismiss(id), 4500);
  }

  dismiss(id: number) {
    this.items = this.items.filter((x) => x.id !== id);
  }
}

export const toastStore = new ToastStore();
