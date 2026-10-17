// The shortlist lives in this browser only: it is a convenience, not a record.

const KEY = "oriel.shortlist";
const MAX = 12;

export function readShortlist(): string[] {
  try {
    const raw = localStorage.getItem(KEY);
    const list = raw ? (JSON.parse(raw) as unknown) : [];
    return Array.isArray(list) ? list.filter((x): x is string => typeof x === "string" && /^\d{4}$/.test(x)).slice(0, MAX) : [];
  } catch {
    return [];
  }
}

function writeShortlist(list: string[]): void {
  try {
    localStorage.setItem(KEY, JSON.stringify(list.slice(0, MAX)));
  } catch {
    // Storage may be unavailable (private browsing); the button still reflects the click.
  }
  document.dispatchEvent(new CustomEvent("shortlist:change", { detail: list }));
}

function paint(button: HTMLButtonElement, on: boolean): void {
  button.setAttribute("aria-pressed", String(on));
  const label = button.querySelector("[data-shortlist-label]");
  if (label) label.textContent = on ? "Shortlisted" : "Shortlist";
}

function paintCount(): void {
  const n = readShortlist().length;
  document.querySelectorAll<HTMLElement>("[data-shortlist-count]").forEach((el) => {
    el.textContent = String(n);
    el.closest<HTMLElement>("[data-shortlist-link]")?.toggleAttribute("hidden", n === 0);
  });
}

export function setUpShortlist(root: ParentNode = document): void {
  const list = readShortlist();
  root.querySelectorAll<HTMLButtonElement>("[data-shortlist]").forEach((button) => {
    const id = button.dataset.shortlist!;
    paint(button, list.includes(id));
    button.addEventListener("click", () => {
      const current = readShortlist();
      const on = !current.includes(id);
      const next = on ? [id, ...current.filter((x) => x !== id)] : current.filter((x) => x !== id);
      writeShortlist(next);
      paint(button, on);
    });
  });
  paintCount();
  if (!listening) {
    document.addEventListener("shortlist:change", paintCount);
    listening = true;
  }
}

let listening = false;
