// The site around the tower, in the manner of an architect's model: paving,
// lawns, the river to the north with trees along its banks, Riverside Drive
// to the south, and the neighbouring buildings as plain white blocks.

import * as THREE from "three";
import type { BuildingModel } from "./data";
import { Kit } from "./kit";

type Part = "ground" | "paving" | "lawn" | "water" | "road" | "marking" | "block";

export interface Context {
  group: THREE.Group;
  dispose(): void;
}

// A small, repeatable random sequence, so the trees stand in the same places
// on every visit.
function random(seed: number): () => number {
  let s = seed >>> 0;
  return () => {
    s = (s * 1664525 + 1013904223) >>> 0;
    return s / 4294967296;
  };
}

export function buildContext(model: BuildingModel, lite: boolean): Context {
  const [x0, z0, x1, z1] = model.plate;
  const kit = new Kit<Part>();
  const rnd = random(86);

  const materials: Record<Part, THREE.MeshStandardMaterial> = {
    ground: new THREE.MeshStandardMaterial({ color: 0xe4e5e1, roughness: 1 }),
    paving: new THREE.MeshStandardMaterial({ color: 0xd9dbd6, roughness: 0.95 }),
    lawn: new THREE.MeshStandardMaterial({ color: 0xc9d2c3, roughness: 1 }),
    water: new THREE.MeshStandardMaterial({ color: 0x9db4c1, roughness: 0.15, metalness: 0.1, envMapIntensity: 0.9 }),
    road: new THREE.MeshStandardMaterial({ color: 0xc3c6c1, roughness: 1 }),
    marking: new THREE.MeshStandardMaterial({ color: 0xf2f3f0, roughness: 1 }),
    block: new THREE.MeshStandardMaterial({ color: 0xf0f0ed, roughness: 0.95 }),
  };

  // The ground itself stretches to the fog.
  kit.box("ground", -700, -0.3, -700, 700, 0, 700);
  // The plot: paving round the tower, lawn to the river.
  kit.box("paving", x0 - 14, 0, z0 - 12, x1 + 14, 0.04, z1 + 22);
  kit.box("lawn", x0 - 26, 0, z0 - 34, x1 + 26, 0.03, z0 - 12);
  kit.box("lawn", x1 + 14, 0, z0 - 12, x1 + 26, 0.03, z1 + 22);
  kit.box("lawn", x0 - 26, 0, z0 - 12, x0 - 14, 0.03, z1 + 22);

  // The river, winding west to east along the north of the site.
  const river = new THREE.Shape();
  const riverZ = (x: number) => z0 - 52 + Math.sin(x / 55) * 9 + Math.sin(x / 23) * 2.5;
  const width = (x: number) => 11 + Math.sin(x / 40) * 2;
  river.moveTo(-700, riverZ(-700) - width(-700) / 2);
  for (let x = -700; x <= 700; x += 10) river.lineTo(x, riverZ(x) - width(x) / 2);
  for (let x = 700; x >= -700; x -= 10) river.lineTo(x, riverZ(x) + width(x) / 2);
  river.closePath();
  kit.shape("water", river, 0.01, 0.03);

  // Riverside Drive, with its centre line.
  const roadZ = z1 + 34;
  kit.box("road", -700, 0, roadZ - 6, 700, 0.035, roadZ + 6);
  for (let x = -700; x < 700; x += 9) kit.box("marking", x, 0.035, roadZ - 0.1, x + 4, 0.045, roadZ + 0.1);
  // The drive into the site.
  kit.box("paving", -5, 0, z1 + 22, 5, 0.04, roadZ - 6);

  // Neighbours: plain blocks, lower than Oriel, as on a model.
  const blocks: [number, number, number, number, number][] = [
    [-92, 4, 34, 26, 21],
    [-96, 62, 30, 22, 15],
    [86, 8, 28, 22, 27],
    [92, 66, 34, 24, 18],
    [-150, -10, 26, 36, 12],
    [150, 0, 30, 26, 33],
    [-40, 86, 38, 22, 15],
    [44, 92, 30, 28, 24],
    [-200, 70, 40, 30, 18],
    [210, 60, 36, 30, 21],
    [0, 140, 50, 26, 12],
    [-120, -140, 36, 28, 15],
    [130, -150, 40, 30, 21],
    [10, -170, 34, 26, 12],
  ];
  for (const [cx, cz, w, d, h] of blocks) {
    kit.box("block", cx - w / 2, 0, cz - d / 2, cx + w / 2, h, cz + d / 2);
  }

  const group = new THREE.Group();
  group.name = "context";
  for (const mesh of kit.meshes(materials, {
    ground: { cast: false, receive: true },
    paving: { cast: false, receive: true },
    lawn: { cast: false, receive: true },
    water: { cast: false, receive: true },
    road: { cast: false, receive: true },
    marking: { cast: false, receive: false },
    block: { cast: true, receive: true },
  })) {
    group.add(mesh);
  }

  // Trees: one shape for every crown and one for every trunk, drawn as two
  // instanced batches.
  const spots: [number, number, number][] = [];
  const along = (zAt: (x: number) => number, from: number, to: number, step: number, jitter: number) => {
    for (let x = from; x <= to; x += step + rnd() * step * 0.6) {
      spots.push([x + (rnd() - 0.5) * jitter, zAt(x) + (rnd() - 0.5) * jitter, 2.3 + rnd() * 1.6]);
    }
  };
  const stretch = lite ? 200 : 320;
  along((x) => riverZ(x) + width(x) / 2 + 4, -stretch, stretch, 9, 3);
  along((x) => riverZ(x) - width(x) / 2 - 4, -stretch, stretch, 11, 3);
  along(() => roadZ - 9, -stretch, stretch, 12, 1.5);
  along(() => roadZ + 9, -stretch, stretch, 22, 1.5);
  for (let i = 0; i < (lite ? 10 : 18); i++) {
    const side = i % 2 ? 1 : -1;
    spots.push([side * (x1 + 18 + rnd() * 6), z0 - 6 + rnd() * (z1 - z0 + 24), 2.0 + rnd() * 1.2]);
  }
  for (let i = 0; i < (lite ? 6 : 12); i++) {
    spots.push([x0 - 10 + rnd() * (x1 - x0 + 20), z0 - 18 - rnd() * 12, 2.2 + rnd() * 1.3]);
  }

  const crownGeometry = new THREE.IcosahedronGeometry(1, 1);
  const trunkGeometry = new THREE.CylinderGeometry(0.16, 0.22, 1, 6);
  const crownMaterial = new THREE.MeshStandardMaterial({ color: 0xa7b59d, roughness: 1, flatShading: true });
  const trunkMaterial = new THREE.MeshStandardMaterial({ color: 0x8d877c, roughness: 1 });
  const crowns = new THREE.InstancedMesh(crownGeometry, crownMaterial, spots.length);
  const trunks = new THREE.InstancedMesh(trunkGeometry, trunkMaterial, spots.length);
  const m = new THREE.Matrix4();
  const q = new THREE.Quaternion();
  spots.forEach(([x, z, r], i) => {
    const trunkH = r * 1.15;
    m.compose(new THREE.Vector3(x, trunkH + r * 0.75, z), q.setFromEuler(new THREE.Euler(0, rnd() * Math.PI, 0)), new THREE.Vector3(r, r * 0.9, r));
    crowns.setMatrixAt(i, m);
    m.compose(new THREE.Vector3(x, trunkH / 2, z), q.identity(), new THREE.Vector3(1, trunkH, 1));
    trunks.setMatrixAt(i, m);
  });
  crowns.castShadow = true;
  crowns.receiveShadow = true;
  trunks.castShadow = true;
  crowns.computeBoundingSphere();
  trunks.computeBoundingSphere();
  group.add(crowns, trunks);

  return {
    group,
    dispose() {
      group.traverse((o) => {
        if (o instanceof THREE.Mesh) o.geometry.dispose();
      });
      Object.values(materials).forEach((mat) => mat.dispose());
      crownMaterial.dispose();
      trunkMaterial.dispose();
    },
  };
}
