// A small toolkit for building the model out of boxes and cylinders. Parts
// that share a material are merged into one geometry, so the whole building
// draws in about a dozen calls. Every part can also leave its outline in a
// single set of lines, ordered as it is built, so the building can be drawn
// line by line before it fills in.

import * as THREE from "three";
import { mergeGeometries } from "three/addons/utils/BufferGeometryUtils.js";

export type Ink = "ink" | "cobalt" | "faint" | null;

const inkColours: Record<Exclude<Ink, null>, THREE.Color> = {
  ink: new THREE.Color(0x1a1d1f),
  cobalt: new THREE.Color(0x1f3bd1),
  faint: new THREE.Color(0x7d8389),
};

export class Kit<Part extends string> {
  // Indexed and non-indexed geometries cannot be merged together, so each
  // part keeps two lists.
  private geometries = new Map<string, { part: Part; list: THREE.BufferGeometry[] }>();
  private positions: number[] = [];
  private colours: number[] = [];

  /** How many line vertices have been added so far. */
  get lineCount(): number {
    return this.positions.length / 3;
  }

  private add(part: Part, geometry: THREE.BufferGeometry): void {
    geometry.deleteAttribute("uv");
    const key = geometry.index ? part : part + "|flat";
    let entry = this.geometries.get(key);
    if (!entry) {
      entry = { part, list: [] };
      this.geometries.set(key, entry);
    }
    entry.list.push(geometry);
  }

  /** A box between two corners. */
  box(part: Part, x0: number, y0: number, z0: number, x1: number, y1: number, z1: number, ink: Ink = null): void {
    const w = Math.abs(x1 - x0);
    const h = Math.abs(y1 - y0);
    const d = Math.abs(z1 - z0);
    if (w < 1e-4 || h < 1e-4 || d < 1e-4) return;
    const g = new THREE.BoxGeometry(w, h, d);
    g.translate((x0 + x1) / 2, (y0 + y1) / 2, (z0 + z1) / 2);
    this.add(part, g);
    if (ink) this.outline(x0, y0, z0, x1, y1, z1, ink);
  }

  /** An upright cylinder. */
  cylinder(part: Part, x: number, z: number, r: number, y0: number, y1: number, ink: Ink = null, segments = 12): void {
    const g = new THREE.CylinderGeometry(r, r, y1 - y0, segments, 1);
    g.translate(x, (y0 + y1) / 2, z);
    this.add(part, g);
    if (ink) {
      this.line(x - r, y0, z, x - r, y1, z, ink);
      this.line(x + r, y0, z, x + r, y1, z, ink);
    }
  }

  /** A flat shape lying on the ground at height y. */
  shape(part: Part, shape: THREE.Shape, y: number, thickness = 0.02): void {
    const g = new THREE.ExtrudeGeometry(shape, { depth: thickness, bevelEnabled: false, curveSegments: 24 });
    g.rotateX(Math.PI / 2);
    g.translate(0, y + thickness, 0);
    this.add(part, g);
  }

  /** One line. */
  line(x0: number, y0: number, z0: number, x1: number, y1: number, z1: number, ink: Exclude<Ink, null>): void {
    const c = inkColours[ink];
    this.positions.push(x0, y0, z0, x1, y1, z1);
    this.colours.push(c.r, c.g, c.b, c.r, c.g, c.b);
  }

  /** A rectangle drawn flat at height y. */
  rectLine(x0: number, z0: number, x1: number, z1: number, y: number, ink: Exclude<Ink, null>): void {
    this.line(x0, y, z0, x1, y, z0, ink);
    this.line(x1, y, z0, x1, y, z1, ink);
    this.line(x1, y, z1, x0, y, z1, ink);
    this.line(x0, y, z1, x0, y, z0, ink);
  }

  /** The twelve edges of a box. */
  outline(x0: number, y0: number, z0: number, x1: number, y1: number, z1: number, ink: Exclude<Ink, null>): void {
    this.rectLine(x0, z0, x1, z1, y0, ink);
    this.rectLine(x0, z0, x1, z1, y1, ink);
    this.line(x0, y0, z0, x0, y1, z0, ink);
    this.line(x1, y0, z0, x1, y1, z0, ink);
    this.line(x1, y0, z1, x1, y1, z1, ink);
    this.line(x0, y0, z1, x0, y1, z1, ink);
  }

  /** One mesh per material, with shadows as the material wants them. */
  meshes(materials: Record<Part, THREE.Material>, shadows: Partial<Record<Part, { cast: boolean; receive: boolean }>>): THREE.Mesh[] {
    const out: THREE.Mesh[] = [];
    for (const { part, list } of this.geometries.values()) {
      const merged = mergeGeometries(list, false);
      list.forEach((g) => g.dispose());
      if (!merged) continue;
      merged.computeBoundingSphere();
      const mesh = new THREE.Mesh(merged, materials[part]);
      const s = shadows[part] ?? { cast: true, receive: true };
      mesh.castShadow = s.cast;
      mesh.receiveShadow = s.receive;
      mesh.name = part;
      out.push(mesh);
    }
    this.geometries.clear();
    return out;
  }

  /** Every line added, as one object. */
  lines(material: THREE.LineBasicMaterial): THREE.LineSegments {
    const g = new THREE.BufferGeometry();
    g.setAttribute("position", new THREE.Float32BufferAttribute(this.positions, 3));
    g.setAttribute("color", new THREE.Float32BufferAttribute(this.colours, 3));
    g.computeBoundingSphere();
    this.positions = [];
    this.colours = [];
    return new THREE.LineSegments(g, material);
  }
}
