// A floor seen from above, drawn as SVG: the flat view of the explorer for
// devices without 3D, and the view a keyboard or screen reader uses.

import type { BuildingModel } from "../model/data";

export interface PlateHome {
  id: string;
  l: number;
  p: number;
  t: string;
  b: number;
  m: number;
  s: string;
  pr?: number;
  f: string;
  h: number;
  r?: boolean;
}

const statusNames: Record<string, string> = {
  available: "Available",
  held: "On hold",
  reserved: "Reserved",
  sold: "Sold",
  withheld: "Not yet released",
};

export function statusName(s: string): string {
  return statusNames[s] ?? s;
}

export function plateSVG(model: BuildingModel, homes: PlateHome[], level: number, matches: (h: PlateHome) => boolean, selected: string | null): string {
  const s = 18;
  const [x0, z0, x1, z1] = model.plate;
  const pad = 3;
  const w = (x1 - x0 + pad * 2) * s;
  const h = (z1 - z0 + pad * 2 + 6) * s;
  const X = (v: number) => (v - x0 + pad) * s;
  const Z = (v: number) => (v - z0 + pad + 6) * s;
  const units = model.units.filter((u) => u.l === level);
  let out = `<svg class="plate" viewBox="0 0 ${w} ${h}" role="group" aria-label="Level ${level} from above">`;
  out += `<defs><pattern id="plate-hatch" width="10" height="10" patternUnits="userSpaceOnUse" patternTransform="rotate(45)"><path d="M0 0V10" stroke="currentColor" stroke-width="2"/></pattern>`;
  out += `<pattern id="plate-cross" width="10" height="10" patternUnits="userSpaceOnUse"><path d="M0 0L10 10M10 0L0 10" stroke="currentColor" stroke-width="1.4"/></pattern></defs>`;
  out += `<path class="plate__river" d="M0 ${Z(z0 - 4.5)}C${w * 0.3} ${Z(z0 - 3)} ${w * 0.6} ${Z(z0 - 6)} ${w} ${Z(z0 - 4)}"/>`;
  out += `<text class="plate__note" x="${X(x0)}" y="${Z(z0 - 5.6)}">River</text>`;
  if (level !== 16) {
    const [c0, cz0, c1, cz1] = model.core;
    out += `<rect class="plate__core" x="${X(c0)}" y="${Z(cz0)}" width="${(c1 - c0) * s}" height="${(cz1 - cz0) * s}"/>`;
    out += `<text class="plate__note" x="${X((c0 + c1) / 2)}" y="${Z((cz0 + cz1) / 2)}" text-anchor="middle">Lifts and stairs</text>`;
  }
  for (const u of units) {
    const home = homes.find((hm) => hm.id === u.id);
    if (!home) continue;
    const [fx0, fz0, fx1, fz1] = u.f;
    const [bx0, bz0, bx1, bz1] = u.b;
    const on = matches(home);
    const cls = `plate__home plate__home--${home.s}${on ? "" : " is-dim"}${selected === home.id ? " is-selected" : ""}`;
    // The name starts with the words the plan shows, then adds what it doesn't.
    const label = `${home.id} ${home.t} · ${statusName(home.s)}, ${home.b === 4 ? "penthouse" : home.b + " bedroom" + (home.b > 1 ? "s" : "")}${home.pr ? ", KES " + home.pr.toLocaleString("en-KE") : ""}`;
    out += `<g class="${cls}" data-home="${home.id}" tabindex="0" role="button" aria-label="${label}">`;
    out += `<rect class="plate__fill" x="${X(fx0)}" y="${Z(fz0)}" width="${(fx1 - fx0) * s}" height="${(fz1 - fz0) * s}"/>`;
    out += `<rect class="plate__bay" x="${X(bx0)}" y="${Z(bz0)}" width="${(bx1 - bx0) * s}" height="${(bz1 - bz0) * s}"/>`;
    out += `<text class="plate__id" x="${X((fx0 + fx1) / 2)}" y="${Z((fz0 + fz1) / 2) - 4}" text-anchor="middle">${home.id}</text>`;
    out += `<text class="plate__sub" x="${X((fx0 + fx1) / 2)}" y="${Z((fz0 + fz1) / 2) + 16}" text-anchor="middle">${home.t} · ${statusName(home.s)}</text>`;
    out += `</g>`;
  }
  out += `<g class="plate__north" transform="translate(${w - 24} 30)"><path d="M0 -14L6 7L0 2L-6 7Z"/><text y="-20" text-anchor="middle">N</text></g>`;
  out += `</svg>`;
  return out;
}
