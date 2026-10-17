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
  const list = readShortlist();
  const n = list.length;
  document.querySelectorAll<HTMLElement>("[data-shortlist-count]").forEach((el) => {
    el.textContent = String(n);
    const link = el.closest<HTMLAnchorElement>("[data-shortlist-link]");
    if (!link) return;
    link.toggleAttribute("hidden", n === 0);
    link.href = n ? "/shortlist?homes=" + list.join(",") : "/shortlist";
  });
  // "Compare" on a home's page brings in up to two more homes from the shortlist.
  document.querySelectorAll<HTMLAnchorElement>("[data-compare-with]").forEach((a) => {
    const id = a.dataset.compareWith!;
    const others = list.filter((x) => x !== id).slice(0, 2);
    a.href = "/compare?homes=" + [id, ...others].join(",");
  });
}

/** The shortlist page: fill it from this browser's list, remove homes, and
 * keep the comparison to at most three. */
function setUpShortlistPage(): void {
  const list = readShortlist();
  const params = new URLSearchParams(location.search);
  if (location.pathname === "/shortlist" && !params.get("homes") && list.length) {
    location.replace("/shortlist?homes=" + list.join(","));
    return;
  }
  document.querySelectorAll<HTMLButtonElement>("[data-shortlist-remove]").forEach((button) => {
    button.addEventListener("click", () => {
      const id = button.dataset.shortlistRemove!;
      const next = readShortlist().filter((x) => x !== id);
      writeShortlist(next);
      button.closest("li")?.remove();
      history.replaceState(null, "", next.length ? "/shortlist?homes=" + next.join(",") : "/shortlist");
      if (!next.length) location.replace("/shortlist");
    });
  });
  const form = document.querySelector<HTMLFormElement>("[data-compare-form]");
  if (!form) return;
  const note = form.querySelector<HTMLElement>("[data-compare-note]");
  const picks = () => Array.from(form.querySelectorAll<HTMLInputElement>("[data-compare-pick]"));
  const update = () => {
    const chosen = picks().filter((p) => p.checked).length;
    picks().forEach((p) => (p.disabled = !p.checked && chosen >= 3));
    if (note) note.textContent = chosen < 2 ? "Choose two or three." : `${chosen} chosen.`;
  };
  form.addEventListener("change", update);
  form.addEventListener("submit", (e) => {
    if (picks().filter((p) => p.checked).length < 1) e.preventDefault();
  });
  update();
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
  if (root === document) setUpShortlistPage();
  if (!listening) {
    document.addEventListener("shortlist:change", paintCount);
    listening = true;
  }
}

let listening = false;
