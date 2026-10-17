// The sales desk's script: the menu on small screens, forms that ask
// before they act, and the live stream that keeps every page current.

import htmx from "htmx.org";
import { setUpConfirms } from "../live";

declare global {
  interface Window {
    htmx: typeof htmx;
  }
}

window.htmx = htmx;
htmx.config.defaultSwapStyle = "outerHTML";
htmx.config.includeIndicatorStyles = false;
htmx.config.allowEval = false;
htmx.config.allowScriptTags = false;

type Change = { type: string; id?: string; status?: string; text?: string };

function setUpMenu(): void {
  const button = document.querySelector<HTMLButtonElement>("[data-desk-menu]");
  const side = document.getElementById("desk-side");
  if (!button || !side) return;
  const close = () => {
    side.classList.remove("is-open");
    button.setAttribute("aria-expanded", "false");
  };
  button.addEventListener("click", () => {
    const open = !side.classList.contains("is-open");
    side.classList.toggle("is-open", open);
    button.setAttribute("aria-expanded", String(open));
  });
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape") close();
  });
  document.addEventListener("click", (e) => {
    if (side.classList.contains("is-open") && !side.contains(e.target as Node) && e.target !== button) close();
  });
}

function toast(text: string, href?: string, label?: string): void {
  const box = document.querySelector<HTMLElement>("[data-toasts]");
  if (!box) return;
  const el = document.createElement("div");
  el.className = "desk-toast";
  const p = document.createElement("span");
  p.textContent = text;
  el.append(p);
  if (href) {
    const a = document.createElement("a");
    a.href = href;
    a.textContent = label || "Open";
    el.append(a);
  }
  box.append(el);
  setTimeout(() => el.remove(), 9000);
  while (box.children.length > 3) box.firstElementChild?.remove();
}

const statuses = ["available", "held", "reserved", "sold", "withheld"];

function applyUnit(change: Change): void {
  if (!change.id || !change.status) return;
  const id = change.id;
  const selector = `[data-unit="${CSS.escape(id)}"]`;
  document.querySelectorAll<HTMLElement>(selector).forEach((cell) => {
    statuses.forEach((s) => cell.classList.remove("is-" + s));
    cell.classList.add("is-" + change.status);
    // Fetch the cell again so its words follow its new status, then let it fill.
    htmx
      .ajax("get", `/desk/homes/${encodeURIComponent(id)}/cell`, { target: cell, swap: "outerHTML" })
      .then(() => {
        document.querySelectorAll<HTMLElement>(selector).forEach((fresh) => {
          fresh.classList.add("is-changed");
          setTimeout(() => fresh.classList.remove("is-changed"), 2400);
        });
      })
      .catch(() => undefined);
  });
}

function setUpLive(): void {
  if (!document.body.hasAttribute("data-desk-live") || !("EventSource" in window)) return;
  const source = new EventSource("/desk/events");
  source.addEventListener("change", (event) => {
    let change: Change;
    try {
      change = JSON.parse((event as MessageEvent).data) as Change;
    } catch {
      return;
    }
    if (change.type === "unit") applyUnit(change);
    if (change.type === "reservation" && change.status === "requested") {
      toast(change.text || "A new reservation request", "/desk/reservations", "Review it");
      const badge = document.querySelector<HTMLElement>("[data-waiting-badge]");
      if (badge) {
        const n = Number(badge.textContent || "0") + 1;
        badge.textContent = String(n);
        badge.classList.remove("is-empty");
      }
    }
    if (change.type === "lead" && change.text) toast(change.text, "/desk/leads", "Leads");
    document.dispatchEvent(new CustomEvent("desk:change", { detail: change }));
  });
  window.addEventListener("pagehide", () => source.close());
}

// The home drawer on the stacking plan closes with its button or Escape.
function setUpDrawer(): void {
  const drawer = document.getElementById("drawer");
  if (!drawer) return;
  const close = () => {
    if (!drawer.firstElementChild) return;
    drawer.replaceChildren();
    if (location.search.includes("home=")) history.pushState(null, "", location.pathname);
  };
  drawer.addEventListener("click", (e) => {
    if ((e.target as HTMLElement).closest("[data-drawer-close]")) close();
  });
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape") close();
  });
  window.addEventListener("popstate", () => {
    if (!location.search.includes("home=")) drawer.replaceChildren();
  });
}

setUpMenu();
setUpDrawer();
setUpConfirms();
setUpLive();
