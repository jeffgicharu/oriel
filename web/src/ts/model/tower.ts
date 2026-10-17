// The tower, built from the server's description of it: the glazed ground
// floor on its columns, the amenity floor, fourteen floors of homes with their
// slab edges, glass, fins, bays and balconies, the penthouse floor set back
// behind its planted terrace, and the pool and pergola on the roof.

import * as THREE from "three";
import type { BuildingModel, ModelLevel, ModelUnit } from "./data";
import { Kit } from "./kit";

export type Part = "concrete" | "glass" | "bayGlass" | "frame" | "accent" | "balustrade" | "water" | "planting" | "timber";

export interface Tower {
  group: THREE.Group;
  materials: Record<Part, THREE.MeshStandardMaterial>;
  lines: THREE.LineSegments;
  lineMaterial: THREE.LineBasicMaterial;
  /** How many line vertices are drawn once each level is complete, bottom up. */
  levelEnds: number[];
  /** Each level as its own group, when built split for the explorer. */
  levels: Map<number, THREE.Group>;
  height: number;
  dispose(): void;
}

export interface TowerOptions {
  /** Build each level as its own group, so floors can be lifted apart. */
  split?: boolean;
}

const SLAB = 0.32;
const OVERHANG = 0.45;
const GLASS_INSET = 0.2;

export function makeMaterials(): Record<Part, THREE.MeshStandardMaterial> {
  const m = (params: THREE.MeshStandardMaterialParameters) => new THREE.MeshStandardMaterial(params);
  return {
    concrete: m({ color: 0xe9e8e3, roughness: 0.9, metalness: 0 }),
    glass: m({ color: 0x5d7486, roughness: 0.04, metalness: 0.55, envMapIntensity: 1.5 }),
    bayGlass: m({ color: 0x8aa3b6, roughness: 0.03, metalness: 0.5, envMapIntensity: 1.8 }),
    frame: m({ color: 0x2c3135, roughness: 0.55, metalness: 0.3 }),
    accent: m({ color: 0x1f3bd1, roughness: 0.42, metalness: 0.1 }),
    balustrade: m({ color: 0xd4e0e8, roughness: 0.05, metalness: 0.1, transparent: true, opacity: 0.38, depthWrite: false }),
    water: m({ color: 0x4f97bd, roughness: 0.08, metalness: 0.15, envMapIntensity: 1.3 }),
    planting: m({ color: 0x8c9f84, roughness: 1, metalness: 0 }),
    timber: m({ color: 0xc8b192, roughness: 0.85, metalness: 0 }),
  };
}

export function buildTower(model: BuildingModel, options: TowerOptions = {}): Tower {
  if (options.split) return buildSplitTower(model);
  const kit = new Kit<Part>();
  const levelEnds: number[] = [];
  const unitsOn = new Map<number, ModelUnit[]>();
  for (const u of model.units) {
    const list = unitsOn.get(u.l) ?? [];
    list.push(u);
    unitsOn.set(u.l, list);
  }

  for (const level of model.levels) {
    switch (level.kind) {
      case "ground":
        groundFloor(kit, model, level);
        break;
      case "amenity":
        amenityFloor(kit, model, level);
        break;
      case "homes":
        homesFloor(kit, model, level, unitsOn.get(level.level) ?? []);
        break;
      case "penthouse":
        penthouseFloor(kit, model, level, unitsOn.get(level.level) ?? []);
        break;
    }
    levelEnds.push(kit.lineCount);
  }
  roof(kit, model);
  levelEnds.push(kit.lineCount);

  const materials = makeMaterials();
  const group = new THREE.Group();
  group.name = "tower";
  for (const mesh of kit.meshes(materials, SHADOWS)) {
    group.add(mesh);
  }
  const lineMaterial = new THREE.LineBasicMaterial({ vertexColors: true, transparent: true, opacity: 1 });
  const lines = kit.lines(lineMaterial);
  lines.name = "drawing";
  lines.renderOrder = 2;
  group.add(lines);

  // Solids sit a hair behind the lines, so edges stay crisp while both show.
  for (const material of Object.values(materials)) {
    material.polygonOffset = true;
    material.polygonOffsetFactor = 1;
    material.polygonOffsetUnits = 1;
  }

  return {
    group,
    materials,
    lines,
    lineMaterial,
    levelEnds,
    levels: new Map(),
    height: model.roof + 3.6,
    dispose() {
      group.traverse((o) => {
        if (o instanceof THREE.Mesh || o instanceof THREE.LineSegments) o.geometry.dispose();
      });
      Object.values(materials).forEach((m) => m.dispose());
      lineMaterial.dispose();
    },
  };
}

const SHADOWS = {
  balustrade: { cast: false, receive: false },
  water: { cast: false, receive: true },
  planting: { cast: true, receive: true },
};

// The explorer's tower: one group per level, the roof riding on the top one.
function buildSplitTower(model: BuildingModel): Tower {
  const materials = makeMaterials();
  const group = new THREE.Group();
  group.name = "tower";
  const levels = new Map<number, THREE.Group>();
  const unitsOn = new Map<number, ModelUnit[]>();
  for (const u of model.units) {
    const list = unitsOn.get(u.l) ?? [];
    list.push(u);
    unitsOn.set(u.l, list);
  }
  for (const level of model.levels) {
    const kit = new Kit<Part>();
    switch (level.kind) {
      case "ground":
        groundFloor(kit, model, level);
        break;
      case "amenity":
        amenityFloor(kit, model, level);
        break;
      case "homes":
        homesFloor(kit, model, level, unitsOn.get(level.level) ?? []);
        break;
      case "penthouse":
        penthouseFloor(kit, model, level, unitsOn.get(level.level) ?? []);
        roof(kit, model);
        break;
    }
    const g = new THREE.Group();
    g.name = "level-" + level.level;
    g.userData.level = level.level;
    for (const mesh of kit.meshes(materials, SHADOWS)) g.add(mesh);
    levels.set(level.level, g);
    group.add(g);
  }
  for (const material of Object.values(materials)) {
    material.polygonOffset = true;
    material.polygonOffsetFactor = 1;
    material.polygonOffsetUnits = 1;
  }
  const lineMaterial = new THREE.LineBasicMaterial({ visible: false });
  const lines = new THREE.LineSegments(new THREE.BufferGeometry(), lineMaterial);
  return {
    group,
    materials,
    lines,
    lineMaterial,
    levelEnds: [],
    levels,
    height: model.roof + 3.6,
    dispose() {
      group.traverse((o) => {
        if (o instanceof THREE.Mesh) o.geometry.dispose();
      });
      lines.geometry.dispose();
      Object.values(materials).forEach((m) => m.dispose());
      lineMaterial.dispose();
    },
  };
}

function groundFloor(k: Kit<Part>, m: BuildingModel, lv: ModelLevel): void {
  const [x0, z0, x1, z1] = m.plate;
  const { y, h } = lv;
  k.rectLine(x0, z0, x1, z1, 0.03, "faint");
  // The lobby and sales suite, glazed and set back behind the columns.
  k.box("glass", x0 + 2.4, y, z0 + 2.4, x1 - 2.4, y + h - 0.05, z1 - 2.4, "ink");
  for (let x = x0 + 2.4; x <= x1 - 2.4 + 0.01; x += (x1 - x0 - 4.8) / 8) {
    k.box("frame", x - 0.04, y, z1 - 2.45, x + 0.04, y + h - 0.05, z1 - 2.35);
    k.box("frame", x - 0.04, y, z0 + 2.35, x + 0.04, y + h - 0.05, z0 + 2.45);
  }
  const span = (x1 - x0 - 1.8) / 6;
  for (let i = 0; i <= 6; i++) {
    const x = x0 + 0.9 + i * span;
    k.cylinder("concrete", x, z0 + 0.9, 0.34, y, y + h, "ink");
    k.cylinder("concrete", x, z1 - 0.9, 0.34, y, y + h, "ink");
  }
  for (const z of [z0 + (z1 - z0) / 3, z0 + (2 * (z1 - z0)) / 3]) {
    k.cylinder("concrete", x0 + 0.9, z, 0.34, y, y + h, "ink");
    k.cylinder("concrete", x1 - 0.9, z, 0.34, y, y + h, "ink");
  }
  // The entrance canopy on the south side.
  k.box("concrete", -7, y + h - 0.6, z1, 7, y + h - 0.32, z1 + 4.6, "ink");
  k.box("accent", -7, y + h - 0.68, z1 + 4.5, 7, y + h - 0.6, z1 + 4.6);
  k.cylinder("frame", -6.4, z1 + 4.1, 0.09, y, y + h - 0.6, "ink", 8);
  k.cylinder("frame", 6.4, z1 + 4.1, 0.09, y, y + h - 0.6, "ink", 8);
}

function slab(k: Kit<Part>, m: BuildingModel, y: number, inset = -OVERHANG, thickness = SLAB): void {
  const [x0, z0, x1, z1] = m.plate;
  k.box("concrete", x0 + inset, y, z0 + inset, x1 - inset, y + thickness, z1 - inset, "ink");
}

function amenityFloor(k: Kit<Part>, m: BuildingModel, lv: ModelLevel): void {
  const [x0, z0, x1, z1] = m.plate;
  const { y, h } = lv;
  slab(k, m, y, -OVERHANG, 0.38);
  const g = 0.3;
  k.box("glass", x0 + g, y + 0.38, z0 + g, x1 - g, y + h, z1 - g, "ink");
  for (let x = x0 + 3; x < x1 - 1; x += 3) {
    k.box("frame", x - 0.05, y + 0.38, z0 + g - 0.12, x + 0.05, y + h, z0 + g);
    k.box("frame", x - 0.05, y + 0.38, z1 - g, x + 0.05, y + h, z1 - g + 0.12);
  }
  for (let z = z0 + 3; z < z1 - 1; z += 3) {
    k.box("frame", x0 + g - 0.12, y + 0.38, z - 0.05, x0 + g, y + h, z + 0.05);
    k.box("frame", x1 - g, y + 0.38, z - 0.05, x1 - g + 0.12, y + h, z + 0.05);
  }
}

/** Whether x lies within a bay on a north or south face. */
function inBay(units: ModelUnit[], x: number, face: "N" | "S"): boolean {
  return units.some((u) => {
    const [bx0, bz0, bx1, bz1] = u.b;
    const onFace = face === "N" ? bz0 < u.f[1] + 0.01 : bz1 > u.f[3] - 0.01;
    return onFace && x > bx0 - 0.25 && x < bx1 + 0.25;
  });
}

function homesFloor(k: Kit<Part>, m: BuildingModel, lv: ModelLevel, units: ModelUnit[]): void {
  const [x0, z0, x1, z1] = m.plate;
  const { y, h } = lv;
  const yg = y + SLAB;
  slab(k, m, y);
  k.box("glass", x0 + GLASS_INSET, yg, z0 + GLASS_INSET, x1 - GLASS_INSET, y + h, z1 - GLASS_INSET, "ink");

  // Mullions on the long faces, every 1.5 m.
  for (let x = x0 + 1.5; x < x1 - 1.2; x += 1.5) {
    if (!inBay(units, x, "N")) {
      k.box("frame", x - 0.035, yg, z0 + GLASS_INSET - 0.1, x + 0.035, y + h, z0 + GLASS_INSET);
      k.line(x, yg, z0 + GLASS_INSET - 0.1, x, y + h, z0 + GLASS_INSET - 0.1, "faint");
    }
    if (!inBay(units, x, "S")) {
      k.box("frame", x - 0.035, yg, z1 - GLASS_INSET, x + 0.035, y + h, z1 - GLASS_INSET + 0.1);
      k.line(x, yg, z1 - GLASS_INSET + 0.1, x, y + h, z1 - GLASS_INSET + 0.1, "faint");
    }
  }
  // Fins on the short faces, against the low morning and evening sun.
  for (let z = z0 + 1.8; z < z1 - 1.6; z += 1.4) {
    k.box("concrete", x1 - GLASS_INSET, yg, z - 0.08, x1 + 0.6, y + h, z + 0.08);
    k.box("concrete", x0 - 0.6, yg, z - 0.08, x0 + GLASS_INSET, y + h, z + 0.08);
    k.line(x1 + 0.6, yg, z, x1 + 0.6, y + h, z, "ink");
    k.line(x0 - 0.6, yg, z, x0 - 0.6, y + h, z, "ink");
  }
  for (const u of units) {
    balcony(k, u, y, m);
    bay(k, m, u, y, h);
  }
}

function bay(k: Kit<Part>, m: BuildingModel, u: ModelUnit, y: number, h: number): void {
  const [bx0, bz0, bx1, bz1] = u.b;
  const yb = y + SLAB;
  const seat = yb + m.seat;
  const head = yb + m.head;
  const top = y + h - 0.02;
  // A solid seat below, the glass, and a cobalt band at the head.
  k.box("concrete", bx0, yb, bz0, bx1, seat, bz1);
  k.box("bayGlass", bx0 + 0.03, seat, bz0 + 0.03, bx1 - 0.03, head, bz1 - 0.03);
  k.box("accent", bx0, head, bz0, bx1, top, bz1);
  for (const [px, pz] of [
    [bx0, bz0],
    [bx1, bz0],
    [bx1, bz1],
    [bx0, bz1],
  ] as const) {
    k.box("frame", px - 0.045, seat, pz - 0.045, px + 0.045, head, pz + 0.045);
  }
  // One mullion down the middle of each outward face.
  const mx = (bx0 + bx1) / 2;
  const mz = (bz0 + bz1) / 2;
  k.box("frame", mx - 0.03, seat, bz0 - 0.03, mx + 0.03, head, bz0 + 0.03);
  k.box("frame", mx - 0.03, seat, bz1 - 0.03, mx + 0.03, head, bz1 + 0.03);
  if (u.c) {
    k.box("frame", bx0 - 0.03, seat, mz - 0.03, bx0 + 0.03, head, mz + 0.03);
    k.box("frame", bx1 - 0.03, seat, mz - 0.03, bx1 + 0.03, head, mz + 0.03);
  }
  k.outline(bx0, yb, bz0, bx1, top, bz1, "cobalt");
  k.rectLine(bx0, bz0, bx1, bz1, seat, "cobalt");
  k.rectLine(bx0, bz0, bx1, bz1, head, "cobalt");
}

function balcony(k: Kit<Part>, u: ModelUnit, y: number, m: BuildingModel): void {
  const [kx0, kz0, kx1, kz1] = u.k;
  if (kx1 - kx0 < 0.5 || kz1 - kz0 < 0.5) return;
  const north = kz1 <= m.plate[1] + 0.01;
  const outer = north ? kz0 : kz1;
  const top = y + 0.26;
  const rail = top + 1.06;
  k.box("concrete", kx0, y, kz0, kx1, top, kz1, "ink");
  const t = 0.03;
  k.box("balustrade", kx0, top, outer - t, kx1, rail, outer + t);
  k.box("balustrade", kx0 - t, top, kz0, kx0 + t, rail, kz1);
  k.box("balustrade", kx1 - t, top, kz0, kx1 + t, rail, kz1);
  k.box("frame", kx0, rail, outer - 0.035, kx1, rail + 0.05, outer + 0.035);
  k.box("frame", kx0 - 0.035, rail, kz0, kx0 + 0.035, rail + 0.05, kz1);
  k.box("frame", kx1 - 0.035, rail, kz0, kx1 + 0.035, rail + 0.05, kz1);
  k.line(kx0, rail, outer, kx1, rail, outer, "faint");
  k.line(kx0, rail, kz0, kx0, rail, kz1, "faint");
  k.line(kx1, rail, kz0, kx1, rail, kz1, "faint");
}

function penthouseFloor(k: Kit<Part>, m: BuildingModel, lv: ModelLevel, units: ModelUnit[]): void {
  const [x0, z0, x1, z1] = m.plate;
  const { y, h } = lv;
  const s = m.setback;
  const yg = y + SLAB;
  slab(k, m, y);
  k.box("glass", x0 + s, yg, z0 + s, x1 - s, y + h, z1 - s, "ink");
  for (let x = x0 + s + 1.5; x < x1 - s - 1.2; x += 1.5) {
    k.box("frame", x - 0.035, yg, z0 + s - 0.1, x + 0.035, y + h, z0 + s);
    k.box("frame", x - 0.035, yg, z1 - s, x + 0.035, y + h, z1 - s + 0.1);
  }
  for (let z = z0 + s + 1.5; z < z1 - s - 1.2; z += 1.5) {
    k.box("frame", x0 + s - 0.1, yg, z - 0.035, x0 + s, y + h, z + 0.035);
    k.box("frame", x1 - s, yg, z - 0.035, x1 - s + 0.1, y + h, z + 0.035);
  }
  // The wall between the two penthouses' terraces.
  k.box("concrete", -0.2, yg, z0, 0.2, y + h, z0 + s, "ink");
  k.box("concrete", -0.2, yg, z1 - s, 0.2, y + h, z1, "ink");
  // Planters along the terrace edge.
  const p = 0.7;
  const ph = yg + 0.75;
  for (const [a0, b0, a1, b1] of [
    [x0, z0, x1, z0 + p],
    [x0, z1 - p, x1, z1],
    [x0, z0 + p, x0 + p, z1 - p],
    [x1 - p, z0 + p, x1, z1 - p],
  ] as const) {
    k.box("concrete", a0, yg, b0, a1, ph, b1, "ink");
    k.box("planting", a0 + 0.08, ph, b0 + 0.08, a1 - 0.08, ph + 0.32, b1 - 0.08);
  }
  // Terrace decking.
  k.box("timber", x0 + p, yg, z0 + p, x1 - p, yg + 0.05, z1 - p);
  for (const u of units) bay(k, m, u, y, h);
}

function roof(k: Kit<Part>, m: BuildingModel): void {
  const [x0, z0, x1, z1] = m.plate;
  const s = m.setback;
  const r = m.roof;
  const top = r + 0.45;
  k.box("concrete", x0 + s - 0.6, r, z0 + s - 0.6, x1 - s + 0.6, top, z1 - s + 0.6, "ink");
  // Glass balustrade round the roof garden.
  const e0 = x0 + s - 0.5;
  const e1 = x1 - s + 0.5;
  const f0 = z0 + s - 0.5;
  const f1 = z1 - s + 0.5;
  const rail = top + 1.1;
  k.box("balustrade", e0, top, f0, e1, rail, f0 + 0.04);
  k.box("balustrade", e0, top, f1 - 0.04, e1, rail, f1);
  k.box("balustrade", e0, top, f0, e0 + 0.04, rail, f1);
  k.box("balustrade", e1 - 0.04, top, f0, e1, rail, f1);
  k.rectLine(e0, f0, e1, f1, rail, "faint");
  // Decking, then the pool on the river side.
  k.box("timber", e0 + 0.4, top, f0 + 0.4, e1 - 0.4, top + 0.06, f1 - 0.4);
  const px0 = -13;
  const px1 = 3;
  const pz0 = z0 + s + 1.4;
  const pz1 = pz0 + 4.2;
  k.box("concrete", px0 - 0.4, top, pz0 - 0.4, px1 + 0.4, top + 0.5, pz0, "ink");
  k.box("concrete", px0 - 0.4, top, pz1, px1 + 0.4, top + 0.5, pz1 + 0.4, "ink");
  k.box("concrete", px0 - 0.4, top, pz0, px0, top + 0.5, pz1, "ink");
  k.box("concrete", px1, top, pz0, px1 + 0.4, top + 0.5, pz1, "ink");
  k.box("water", px0, top, pz0, px1, top + 0.42, pz1);
  k.rectLine(px0, pz0, px1, pz1, top + 0.42, "cobalt");
  // Loungers by the pool.
  for (let x = px0 + 0.6; x < px1 - 1; x += 2.3) {
    k.box("frame", x, top + 0.06, pz1 + 1.0, x + 0.7, top + 0.42, pz1 + 2.9);
  }
  // The pergola over the south half.
  const g0 = -15;
  const g1 = 13;
  const h0 = 1.6;
  const h1 = z1 - s - 1.2;
  const gh = top + 3.0;
  for (let x = g0; x <= g1 + 0.01; x += 4) {
    k.box("frame", x - 0.09, top, h0 - 0.09, x + 0.09, gh, h0 + 0.09, "ink");
    k.box("frame", x - 0.09, top, h1 - 0.09, x + 0.09, gh, h1 + 0.09, "ink");
  }
  k.box("frame", g0, gh, h0 - 0.1, g1, gh + 0.22, h0 + 0.1, "ink");
  k.box("frame", g0, gh, h1 - 0.1, g1, gh + 0.22, h1 + 0.1, "ink");
  for (let x = g0 + 0.3; x < g1; x += 0.6) {
    k.box("frame", x - 0.04, gh + 0.22, h0 - 0.3, x + 0.04, gh + 0.42, h1 + 0.3);
  }
  // Lift overrun and plant room above the core.
  const [c0, , c1] = m.core;
  k.box("concrete", c0 + 1.2, top, 2.6, c1 - 1.2, top + 3.1, 7.6, "ink");
  // Planting along the garden edge.
  k.box("planting", e0 + 0.5, top, f0 + 0.5, e0 + 1.4, top + 0.7, f1 - 0.5);
  k.box("planting", e1 - 1.4, top, f0 + 0.5, e1 - 0.5, top + 0.7, f1 - 0.5);
}
