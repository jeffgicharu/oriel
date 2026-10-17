// The public site's script. Pages work without it; it adds the phone menu,
// the header's change on scroll, htmx for partial updates, and loads the
// heavier modules (the 3D building, the calculator) only on pages that use them.

import htmx from "htmx.org";
import { setUpPlans } from "./plans";
import { setUpShortlist } from "./shortlist";
import { setUpConfirms, setUpLiveReservation, setUpPans, showFormErrors } from "./live";

declare global {
  interface Window {
    htmx: typeof htmx;
  }
}

window.htmx = htmx;
htmx.config.defaultSwapStyle = "outerHTML";
htmx.config.scrollIntoViewOnBoost = false;
htmx.config.includeIndicatorStyles = false;
htmx.config.allowEval = false;
htmx.config.allowScriptTags = false;

function setUpMenu(): void {
  const button = document.querySelector<HTMLButtonElement>("[data-menu-button]");
  const sheet = document.getElementById("menu-sheet");
  if (!button || !sheet) return;

  const setOpen = (open: boolean) => {
    document.body.classList.toggle("menu-open", open);
    button.setAttribute("aria-expanded", String(open));
    button.querySelector("[data-menu-label]")!.textContent = open ? "Close" : "Menu";
    sheet.toggleAttribute("inert", !open);
    if (open) sheet.querySelector<HTMLElement>("a")?.focus({ preventScroll: true });
  };

  sheet.setAttribute("inert", "");
  button.addEventListener("click", () => setOpen(button.getAttribute("aria-expanded") !== "true"));
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && button.getAttribute("aria-expanded") === "true") {
      setOpen(false);
      button.focus();
    }
  });
  window.matchMedia("(min-width: 1024px)").addEventListener("change", (query) => {
    if (query.matches) setOpen(false);
  });
}

function setUpHeader(): void {
  const header = document.querySelector<HTMLElement>(".site-header--over");
  if (!header) return;
  const update = () => header.classList.toggle("is-scrolled", window.scrollY > 24);
  update();
  window.addEventListener("scroll", update, { passive: true });
}

// Each line of a drawing is given its own length, so it can trace itself.
function measureDrawings(): void {
  document.querySelectorAll<SVGGeometryElement>("[data-draw]").forEach((path, index) => {
    const length = Math.ceil(path.getTotalLength?.() ?? 1200);
    path.style.setProperty("--len", String(length));
    if (!path.style.getPropertyValue("--i")) path.style.setProperty("--i", String(index));
  });
}

// The 3D building loads after the page has painted, and only where it is shown.
function loadModel(): void {
  const explorer = document.querySelector<HTMLElement>("[data-explorer]");
  if (explorer) {
    void import("./explorer/index").then((m) => m.mountExplorer(explorer));
    return;
  }
  const hero = document.querySelector<HTMLElement>("[data-model-stage='hero']");
  if (!hero) return;
  const start = () => import("./model/hero").then((m) => m.mountHero(hero));
  if ("requestIdleCallback" in window) {
    window.requestIdleCallback(start, { timeout: 600 });
  } else {
    setTimeout(start, 120);
  }
}

setUpMenu();
setUpHeader();
measureDrawings();
setUpPlans();
setUpShortlist();
setUpConfirms();
setUpLiveReservation();
showFormErrors();
setUpPans();
loadModel();

// Parts of a page swapped in by htmx get the same treatment.
document.body.addEventListener("htmx:afterSwap", (event) => {
  const target = (event as CustomEvent<{ target: HTMLElement }>).detail.target;
  setUpPlans(target);
  setUpShortlist(target);
});
