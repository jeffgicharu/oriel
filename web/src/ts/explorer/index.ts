// The explorer: the floors and their homes, filters, and a home to pick for
// its card. Devices that can draw 3D then load the building in 3D (scene.ts);
// the rest get the same floors and homes as a flat plan, without downloading
// three.js.

import type { BuildingModel } from "../model/data";
import { isSoftwareRenderer, rendererName, supportsWebGL } from "../model/capable";
import { plateSVG, statusName, type PlateHome } from "./plate";
import type { Hooks } from "./scene";

interface ExploreData {
  model: BuildingModel;
  homes: PlateHome[];
  start?: string;
}

interface Filter {
  beds: number;
  max: number;
  all: boolean;
}

export function mountExplorer(root: HTMLElement): void {
  const source = document.getElementById("oriel-explore");
  if (!source?.textContent) return;
  const data = JSON.parse(source.textContent) as ExploreData;
  const model = data.model;
  const homes = new Map(data.homes.map((h) => [h.id, h]));

  const $ = <T extends Element>(sel: string) => root.querySelector<T>(sel);
  const stageEl = $<HTMLElement>("[data-explorer-stage]")!;
  const plateEl = $<HTMLElement>("[data-plate]")!;
  const loading = $<HTMLElement>("[data-loading]");
  const tip = $<HTMLElement>("[data-tip]")!;
  const card = $<HTMLElement>("[data-card]")!;
  const list = $<HTMLElement>("[data-homes]")!;
  const form = $<HTMLFormElement>("[data-filters]")!;
  const levelButtons = Array.from(root.querySelectorAll<HTMLButtonElement>("[data-level]"));
  const sunTime = $<HTMLInputElement>("[data-sun-time]")!;
  const sunOut = $<HTMLOutputElement>("[data-sun-out]")!;
  const sunMonth = $<HTMLSelectElement>("[data-sun-month]")!;
  const reset = $<HTMLButtonElement>("[data-reset]")!;

  let level: number | null = null;
  let picked: string | null = null;
  let filter = readFilter();

  function readFilter(): Filter {
    const f = new FormData(form);
    return {
      beds: Number(f.get("beds") || 0),
      max: Number(f.get("max") || 0),
      all: f.get("show") === "all",
    };
  }

  const matches = (h: PlateHome): boolean => {
    if (!filter.all && h.s !== "available") return false;
    if (filter.beds && h.b !== filter.beds) return false;
    if (filter.max && (!h.pr || h.pr > filter.max)) return false;
    return true;
  };

  // ---------- The panel ----------

  function paintLevels(): void {
    for (const button of levelButtons) {
      const l = Number(button.dataset.level);
      let free = 0;
      button.querySelectorAll<HTMLElement>("[data-cell]").forEach((cell) => {
        const h = homes.get(cell.dataset.cell!);
        if (!h) return;
        cell.className = `cell cell--${h.s}${matches(h) ? "" : " is-dim"}`;
        if (matches(h)) free++;
      });
      const count = button.querySelector("[data-level-count]");
      if (count) count.textContent = free > 0 ? `${free} ${filter.all ? "shown" : "free"}` : "—";
      button.setAttribute("aria-pressed", String(level === l));
      button.classList.toggle("is-empty", free === 0);
    }
  }

  function paintList(): void {
    if (level === null) {
      list.hidden = true;
      return;
    }
    const onLevel = data.homes.filter((h) => h.l === level).sort((a, b) => a.p - b.p);
    list.hidden = false;
    list.innerHTML =
      `<p class="label">Level ${String(level).padStart(2, "0")} · ${onLevel.length} homes</p><ul role="list">` +
      onLevel
        .map(
          (h) =>
            `<li><button type="button" class="lhome${matches(h) ? "" : " is-dim"}${picked === h.id ? " is-picked" : ""}" data-pick="${h.id}">` +
            `<span class="mono lhome__id">${h.id}</span><span>${h.b === 4 ? "Penthouse" : h.b + " bed"} · ${h.f}</span>` +
            `<span class="lhome__right">${h.pr ? `<span class="tnum">KES ${(h.pr / 1e6).toFixed(2)}M</span>` : ""}<span class="status status--${h.s}"><span class="status__swatch" aria-hidden="true"></span>${statusName(h.s)}</span></span></button></li>`,
        )
        .join("") +
      `</ul>`;
  }

  async function openCard(id: string): Promise<void> {
    picked = id;
    paintList();
    card.hidden = false;
    card.classList.add("is-loading");
    try {
      const res = await fetch(`/explore/card/${encodeURIComponent(id)}`, { headers: { "HX-Request": "true" } });
      if (!res.ok) throw new Error(String(res.status));
      card.innerHTML = await res.text();
    } catch {
      card.innerHTML = `<p class="notice notice--alert">That home could not be loaded. Please try again.</p>`;
    }
    card.classList.remove("is-loading");
    card.querySelector<HTMLButtonElement>("[data-card-close]")?.addEventListener("click", closeCard);
    const url = new URL(location.href);
    url.searchParams.set("home", id);
    history.replaceState(null, "", url);
  }

  function closeCard(): void {
    picked = null;
    card.hidden = true;
    card.innerHTML = "";
    paintList();
    const url = new URL(location.href);
    url.searchParams.delete("home");
    history.replaceState(null, "", url);
    hooks.onPick?.(null);
  }

  const hooks: Hooks = {};

  function chooseLevel(l: number | null): void {
    level = l;
    paintLevels();
    paintList();
    hooks.onLevel?.(l);
  }

  function pick(id: string): void {
    const h = homes.get(id);
    if (!h) return;
    if (level !== h.l) chooseLevel(h.l);
    void openCard(id);
    hooks.onPick?.(id);
  }

  levelButtons.forEach((b) =>
    b.addEventListener("click", () => {
      const l = Number(b.dataset.level);
      if (picked) closeCard();
      chooseLevel(level === l ? null : l);
    }),
  );
  list.addEventListener("click", (e) => {
    const button = (e.target as HTMLElement).closest<HTMLButtonElement>("[data-pick]");
    if (button) pick(button.dataset.pick!);
  });
  form.addEventListener("submit", (e) => e.preventDefault());
  form.addEventListener("change", () => {
    filter = readFilter();
    paintLevels();
    paintList();
    hooks.onFilter?.();
    const url = new URL(location.href);
    for (const [k, v] of [
      ["beds", filter.beds ? String(filter.beds) : ""],
      ["max", filter.max ? String(filter.max) : ""],
      ["show", filter.all ? "all" : ""],
    ] as const) {
      if (v) url.searchParams.set(k, v);
      else url.searchParams.delete(k);
    }
    history.replaceState(null, "", url);
  });
  reset.addEventListener("click", () => {
    if (picked) closeCard();
    chooseLevel(null);
  });

  // The month starts at this month in Nairobi.
  sunMonth.value = String(new Date(Date.now() + 3 * 3600_000).getUTCMonth() + 1);

  paintLevels();

  // ---------- Flat view ----------

  const options = new URLSearchParams(location.hash.slice(1));
  const capture = options.has("capture");
  const nav = navigator as Navigator & { connection?: { saveData?: boolean } };
  const can3D = supportsWebGL() && (capture || !isSoftwareRenderer(rendererName())) && !nav.connection?.saveData;

  const showFlat = () => {
    root.classList.add("is-flat");
    loading?.remove();
    plateEl.hidden = false;
    const draw = () => {
      const l = level ?? (data.start ? homes.get(data.start)!.l : 7);
      plateEl.innerHTML = `<p class="label">Level ${String(l).padStart(2, "0")} from above</p>` + plateSVG(model, data.homes, l, matches, picked);
    };
    hooks.onLevel = (l) => {
      if (l === null) level = null;
      draw();
    };
    hooks.onFilter = draw;
    hooks.onPick = draw;
    plateEl.addEventListener("click", (e) => {
      const g = (e.target as Element).closest<SVGGElement>("[data-home]");
      if (g) pick(g.dataset.home!);
    });
    plateEl.addEventListener("keydown", (e) => {
      const g = (e.target as Element).closest<SVGGElement>("[data-home]");
      if (g && (e.key === "Enter" || e.key === " ")) {
        e.preventDefault();
        pick(g.dataset.home!);
      }
    });
    if (data.start) pick(data.start);
    else chooseLevel(7);
    draw();
  };

  if (!can3D) {
    showFlat();
    return;
  }

  // The 3D view, or the flat plan if it cannot be loaded.
  import("./scene")
    .then((m) =>
      m.mountScene({
        root,
        stageEl,
        tip,
        loading,
        sunTime,
        sunOut,
        sunMonth,
        model,
        homes,
        start: data.start,
        capture,
        picked: () => picked,
        matches,
        chooseLevel,
        pick,
        closeCard,
        hooks,
      }),
    )
    .catch(showFlat);
}
