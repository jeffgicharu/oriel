// The building's geometry, as the server describes it. Rectangles are
// [x0, z0, x1, z1] in metres; x runs east, z runs south, y runs up.

export type Rect = [number, number, number, number];

export interface ModelLevel {
  level: number;
  y: number;
  h: number;
  kind: "ground" | "amenity" | "homes" | "penthouse";
}

export interface ModelUnit {
  id: string;
  l: number;
  p: number;
  t: string;
  f: Rect;
  b: Rect;
  c?: boolean;
  k: Rect;
  o: string;
  r?: boolean;
}

export interface BuildingModel {
  plate: Rect;
  core: Rect;
  levels: ModelLevel[];
  roof: number;
  setback: number;
  units: ModelUnit[];
  seat: number;
  head: number;
  lat: number;
  lon: number;
}

export function readModel(): BuildingModel | null {
  const el = document.getElementById("oriel-model");
  if (!el?.textContent) return null;
  try {
    return JSON.parse(el.textContent) as BuildingModel;
  } catch {
    return null;
  }
}
