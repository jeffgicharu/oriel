// Figures that change when a visitor picks another payment plan roll from
// the old amount to the new one, so the change can be read.

const SELECTORS = [".schedule-card__total", ".mortgage__monthly"];
const before = new Map<string, number>();

function amount(el: Element | null): number | null {
  const digits = el?.textContent?.replace(/[^0-9]/g, "");
  return digits ? Number(digits) : null;
}

function format(n: number): string {
  return "KES " + Math.round(n).toLocaleString("en-KE");
}

function roll(el: HTMLElement, from: number, to: number): void {
  if (from === to) return;
  const start = performance.now();
  const length = 620;
  const step = (now: number) => {
    const t = Math.min(1, (now - start) / length);
    const eased = 1 - Math.pow(1 - t, 3);
    el.textContent = format(from + (to - from) * eased);
    if (t < 1) requestAnimationFrame(step);
  };
  requestAnimationFrame(step);
}

export function setUpRolls(): void {
  if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
  document.body.addEventListener("htmx:beforeSwap", () => {
    before.clear();
    for (const sel of SELECTORS) {
      const n = amount(document.querySelector(sel));
      if (n !== null) before.set(sel, n);
    }
  });
  document.body.addEventListener("htmx:afterSwap", () => {
    for (const sel of SELECTORS) {
      const el = document.querySelector<HTMLElement>(sel);
      const to = amount(el);
      const from = before.get(sel);
      if (el && to !== null && from !== undefined) roll(el, from, to);
    }
  });
}
