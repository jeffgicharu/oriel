// The explorer's 3D view: the whole building to turn and zoom, a floor to
// open up with every home on it shown by status, and the sun to move through
// the day and the year. It loads only on devices that can draw it, so the
// rest never download three.js.

import gsap from "gsap";
import * as THREE from "three";
import { OrbitControls } from "three/addons/controls/OrbitControls.js";
import { buildContext } from "../model/context";
import type { BuildingModel } from "../model/data";
import { preferLite, Stage } from "../model/stage";
import { buildTower } from "../model/tower";
import { statusName, type PlateHome } from "./plate";

/** What the explorer's panel does when the view changes, set by the view. */
export interface Hooks {
  onPick?: (id: string | null) => void;
  onLevel?: (l: number | null) => void;
  onFilter?: () => void;
}

/** The explorer's panel, as the 3D view sees it. */
export interface Explorer {
  root: HTMLElement;
  stageEl: HTMLElement;
  tip: HTMLElement;
  loading: HTMLElement | null;
  sunTime: HTMLInputElement;
  sunOut: HTMLOutputElement;
  sunMonth: HTMLSelectElement;
  model: BuildingModel;
  homes: Map<string, PlateHome>;
  start?: string;
  capture: boolean;
  picked: () => string | null;
  matches: (h: PlateHome) => boolean;
  chooseLevel: (l: number | null) => void;
  pick: (id: string) => void;
  closeCard: () => void;
  hooks: Hooks;
}

const LIFT = 24;

export function mountScene(x: Explorer): void {
  const { root, stageEl, tip, loading, sunTime, sunOut, sunMonth, model, homes, capture, matches, chooseLevel, pick, closeCard, hooks } = x;

  const lite = !capture && preferLite();
  if (capture) gsap.ticker.lagSmoothing(0);
  const canvas = document.createElement("canvas");
  canvas.className = "explorer__canvas";
  canvas.setAttribute("aria-hidden", "true");
  stageEl.prepend(canvas);
  const stage = new Stage(canvas, { lite });
  stage.external = true;
  // The closed building draws merged, in a few dozen calls; the floors come
  // apart only while one is open.
  const whole = buildTower(model);
  whole.lines.visible = false;
  const tower = buildTower(model, { split: true });
  tower.group.visible = false;
  const context = buildContext(model, lite);
  stage.scene.add(context.group, whole.group, tower.group);

  // Status materials: the hatches of the drawings, as small repeating textures.
  const hatch = (kind: "single" | "cross", colour: string, ground: string) => {
    const c = document.createElement("canvas");
    c.width = c.height = 64;
    const g = c.getContext("2d")!;
    g.fillStyle = ground;
    g.fillRect(0, 0, 64, 64);
    g.strokeStyle = colour;
    g.lineWidth = 6;
    for (let i = -64; i < 128; i += 16) {
      g.beginPath();
      g.moveTo(i, 0);
      g.lineTo(i + 64, 64);
      g.stroke();
      if (kind === "cross") {
        g.beginPath();
        g.moveTo(i + 64, 0);
        g.lineTo(i, 64);
        g.stroke();
      }
    }
    const t = new THREE.CanvasTexture(c);
    t.colorSpace = THREE.SRGBColorSpace;
    t.wrapS = t.wrapT = THREE.RepeatWrapping;
    t.repeat.set(3, 3);
    return t;
  };
  const textures = [hatch("single", "#2a2e31", "#e9eaf2"), hatch("cross", "#2a2e31", "#dcdde3")];
  const mats = {
    available: new THREE.MeshStandardMaterial({ color: 0x1f3bd1, roughness: 0.55, emissive: 0x0b1a6e, emissiveIntensity: 0.35 }),
    held: new THREE.MeshStandardMaterial({ map: textures[0], roughness: 0.8 }),
    reserved: new THREE.MeshStandardMaterial({ map: textures[1], roughness: 0.8 }),
    sold: new THREE.MeshStandardMaterial({ color: 0x2a2e31, roughness: 0.75 }),
    withheld: new THREE.MeshStandardMaterial({ color: 0xc9ccd0, roughness: 0.9 }),
    dim: new THREE.MeshStandardMaterial({ color: 0xe4e5e1, roughness: 1, transparent: true, opacity: 0.55 }),
    hover: new THREE.MeshStandardMaterial({ color: 0x5c74f2, roughness: 0.4, emissive: 0x3552e6, emissiveIntensity: 0.55 }),
    picked: new THREE.MeshStandardMaterial({ color: 0xf2b84b, roughness: 0.45, emissive: 0xa26a10, emissiveIntensity: 0.4 }),
  };

  // One box per home, inside its floor, shown when the floor is open.
  const volumes = new Map<string, THREE.Mesh>();
  const levelY = new Map(model.levels.map((l) => [l.level, l]));
  const boxGeometry = new THREE.BoxGeometry(1, 1, 1);
  for (const u of model.units) {
    const lv = levelY.get(u.l);
    const group = tower.levels.get(u.l);
    if (!lv || !group) continue;
    const [x0, z0, x1, z1] = u.f;
    const inset = 0.18;
    const y0 = lv.y + 0.36;
    const y1 = lv.y + lv.h - 0.08;
    const mesh = new THREE.Mesh(boxGeometry, mats.dim);
    mesh.scale.set(x1 - x0 - inset * 2, y1 - y0, z1 - z0 - inset * 2);
    mesh.position.set((x0 + x1) / 2, (y0 + y1) / 2, (z0 + z1) / 2);
    mesh.userData.id = u.id;
    mesh.visible = false;
    mesh.castShadow = false;
    mesh.receiveShadow = true;
    group.add(mesh);
    volumes.set(u.id, mesh);
    // The bay too, so a home reads as its own shape.
    const [bx0, bz0, bx1, bz1] = u.b;
    const bay = new THREE.Mesh(boxGeometry, mats.dim);
    bay.scale.set(bx1 - bx0 - 0.1, (y1 - y0) * 0.8, bz1 - bz0 - 0.1);
    bay.position.set((bx0 + bx1) / 2, y0 + (y1 - y0) * 0.4, (bz0 + bz1) / 2);
    bay.userData.id = u.id;
    bay.userData.bay = true;
    bay.visible = false;
    group.add(bay);
    volumes.set(u.id + ":bay", bay);
  }

  let hovered: string | null = null;
  const materialFor = (id: string): THREE.Material => {
    const h = homes.get(id)!;
    if (id === x.picked()) return mats.picked;
    if (id === hovered) return mats.hover;
    if (!matches(h)) return mats.dim;
    return (mats as Record<string, THREE.MeshStandardMaterial | undefined>)[h.s] ?? mats.dim;
  };
  const paintVolumes = () => {
    for (const [key, mesh] of volumes) {
      const id = key.split(":")[0]!;
      mesh.material = materialFor(id);
    }
    stage.requestRender();
  };
  paintVolumes();

  // Camera and controls.
  const wide = window.matchMedia("(min-width: 1000px)");
  const overview = () => {
    stage.shift = wide.matches ? 0.14 : 0;
    stage.updateProjection();
    const distance = stage.fitDistance(tower.height + 4, 52, wide.matches ? 0.72 : 0.84);
    return { azimuth: 152, elevation: 12, distance, target: new THREE.Vector3(0, tower.height * 0.44, 0) };
  };
  const start = overview();
  stage.orbit.azimuth = start.azimuth;
  stage.orbit.elevation = start.elevation;
  stage.orbit.distance = start.distance;
  stage.orbit.target.copy(start.target);
  stage.applyCamera();

  const controls = new OrbitControls(stage.camera, canvas);
  controls.target.copy(start.target);
  controls.enableDamping = true;
  controls.dampingFactor = 0.08;
  controls.enablePan = false;
  controls.minDistance = 36;
  controls.maxDistance = 320;
  controls.minPolarAngle = THREE.MathUtils.degToRad(8);
  controls.maxPolarAngle = THREE.MathUtils.degToRad(86);
  controls.rotateSpeed = 0.7;
  controls.zoomSpeed = 0.8;
  controls.update();
  let settling = 0;
  const keepUpdating = () => {
    settling = performance.now() + 900;
    stage.addTicker(() => {
      const moved = controls.update();
      return moved || performance.now() < settling;
    });
  };
  controls.addEventListener("start", keepUpdating);
  controls.addEventListener("change", () => stage.requestRender());
  // An open floor sits in the part of the stage the home card leaves clear,
  // at the size that fits it there, so a picked home is never under the card.
  const cardRoom = 400; // the card, its margin and a gap beside it
  const plateSpan = Math.hypot(model.plate[2] - model.plate[0] + 4, model.plate[3] - model.plate[1] + 4);
  const floorClear = () => Math.max(stage.box.width - cardRoom, stage.box.width * 0.45);
  const floorShift = () => (wide.matches ? (floorClear() - stage.box.width) / 2 / stage.box.width : 0);
  const floorDistance = () => {
    if (!wide.matches) return 132;
    const vTan = Math.tan(THREE.MathUtils.degToRad(stage.camera.fov / 2));
    return (plateSpan * stage.box.height) / (2 * vTan * 0.82 * floorClear());
  };
  stage.onResize = () => {
    stage.shift = openLevel !== null ? floorShift() : wide.matches ? 0.14 : 0;
  };

  // The sun.
  const applySun = () => {
    const minutes = Number(sunTime.value);
    const h = Math.floor(minutes / 60);
    const m = minutes % 60;
    sunOut.value = `${String(h).padStart(2, "0")}:${String(m).padStart(2, "0")}`;
    const now = new Date();
    const date = new Date(Date.UTC(now.getUTCFullYear(), Number(sunMonth.value) - 1, 15, h - 3, m));
    stage.setSun(date, model.lat, model.lon);
    const alt = stage.sunNow.altitude;
    // Dusk: the sky deepens and the glass glows from the lights inside.
    const night = THREE.MathUtils.clamp((4 - alt) / 10, 0, 1);
    const day = new THREE.Color(0xecedea);
    const dusk = new THREE.Color(0x27303b);
    const sky = day.clone().lerp(dusk, night);
    stage.scene.background = sky;
    (stage.scene.fog as THREE.Fog).color.copy(sky);
    stage.hemisphere.intensity = 1.15 - night * 0.85;
    stage.sun.intensity *= 1 - night;
    for (const t of [whole, tower]) {
      for (const key of ["glass", "bayGlass"] as const) {
        const mat = t.materials[key];
        mat.emissive.setHex(0xffc477);
        mat.emissiveIntensity = night * (key === "bayGlass" ? 0.9 : 0.55);
      }
    }
    root.classList.toggle("is-night", night > 0.5);
    stage.requestRender();
  };
  sunTime.addEventListener("input", applySun);
  sunMonth.addEventListener("change", applySun);
  applySun();

  // Opening a floor: the floors above lift away, its glass comes off, and the
  // homes on it appear in their colours.
  const tweenCamera = (target: THREE.Vector3, distance: number, elevation: number) => {
    const offset = stage.camera.position.clone().sub(controls.target);
    const azimuth = Math.atan2(offset.x, -offset.z);
    const el = THREE.MathUtils.degToRad(elevation);
    const to = new THREE.Vector3(
      target.x + distance * Math.cos(el) * Math.sin(azimuth),
      target.y + distance * Math.sin(el),
      target.z - distance * Math.cos(el) * Math.cos(azimuth),
    );
    let running = true;
    stage.addTicker(() => running);
    gsap.to(controls.target, { x: target.x, y: target.y, z: target.z, duration: 1.1, ease: "power3.inOut", onUpdate: () => controls.update() });
    gsap.to(stage.camera.position, {
      x: to.x,
      y: to.y,
      z: to.z,
      duration: 1.1,
      ease: "power3.inOut",
      onUpdate: () => controls.update(),
      onComplete: () => {
        running = false;
      },
    });
  };

  const lens = { shift: stage.shift };
  const tweenShift = (to: number) => {
    lens.shift = stage.shift;
    gsap.to(lens, {
      shift: to,
      duration: 1.1,
      ease: "power3.inOut",
      onUpdate: () => {
        stage.shift = lens.shift;
        stage.updateProjection();
      },
    });
  };

  let openLevel: number | null = null;
  const showLevel = (l: number | null) => {
    const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    const duration = reduced ? 0 : 1.0;
    let running = true;
    stage.addTicker(() => running);
    if (l !== null) {
      whole.group.visible = false;
      tower.group.visible = true;
    }
    for (const [lvl, group] of tower.levels) {
      const lift = l !== null && lvl > l ? LIFT : 0;
      gsap.to(group.position, { y: lift, duration, ease: "power3.inOut" });
      const open = lvl === l;
      group.children.forEach((child) => {
        if (child.name === "glass" || child.name === "bayGlass") child.visible = !open;
        if (child.userData.id) child.visible = open;
      });
    }
    gsap.delayedCall(duration + 0.05, () => {
      running = false;
      if (openLevel === null) {
        whole.group.visible = true;
        tower.group.visible = false;
      }
      stage.requestRender();
    });
    openLevel = l;
    if (l === null) {
      const o = overview();
      tweenShift(wide.matches ? 0.14 : 0);
      tweenCamera(o.target, o.distance, o.elevation);
    } else {
      const lv = levelY.get(l)!;
      tweenShift(floorShift());
      tweenCamera(new THREE.Vector3(0, lv.y + 1.5, 0), floorDistance(), 42);
    }
    paintVolumes();
  };

  hooks.onLevel = (l) => showLevel(l);
  hooks.onFilter = paintVolumes;
  hooks.onPick = () => paintVolumes();

  // Picking homes with the pointer.
  const raycaster = new THREE.Raycaster();
  const pointer = new THREE.Vector2();
  const hit = (event: PointerEvent): string | null => {
    if (openLevel === null) return null;
    const box = canvas.getBoundingClientRect();
    pointer.set(((event.clientX - box.left) / box.width) * 2 - 1, -((event.clientY - box.top) / box.height) * 2 + 1);
    raycaster.setFromCamera(pointer, stage.camera);
    const candidates = Array.from(volumes.values()).filter((m) => m.visible);
    const found = raycaster.intersectObjects(candidates, false)[0];
    return found ? (found.object.userData.id as string) : null;
  };
  canvas.addEventListener("pointermove", (event) => {
    if (event.pointerType !== "mouse") return;
    const id = hit(event);
    if (id !== hovered) {
      hovered = id;
      paintVolumes();
    }
    canvas.style.cursor = id ? "pointer" : "grab";
    if (id) {
      const h = homes.get(id)!;
      tip.hidden = false;
      tip.innerHTML = `<strong class="mono">${h.id}</strong> ${h.b === 4 ? "Penthouse" : h.b + " bed"}${h.pr ? ` · KES ${(h.pr / 1e6).toFixed(2)}M` : ""} · ${statusName(h.s)}`;
      const box = stageEl.getBoundingClientRect();
      tip.style.transform = `translate(${event.clientX - box.left + 16}px, ${event.clientY - box.top + 12}px)`;
    } else {
      tip.hidden = true;
    }
  });
  canvas.addEventListener("pointerleave", () => {
    tip.hidden = true;
    if (hovered) {
      hovered = null;
      paintVolumes();
    }
  });
  let down: { x: number; y: number } | null = null;
  canvas.addEventListener("pointerdown", (e) => (down = { x: e.clientX, y: e.clientY }));
  canvas.addEventListener("pointerup", (e) => {
    if (!down || Math.hypot(e.clientX - down.x, e.clientY - down.y) > 6) return;
    down = null;
    const id = hit(e);
    if (id) pick(id);
    else if (openLevel === null) {
      // A tap on the closed building opens the floor under the pointer.
      const box = canvas.getBoundingClientRect();
      pointer.set(((e.clientX - box.left) / box.width) * 2 - 1, -((e.clientY - box.top) / box.height) * 2 + 1);
      raycaster.setFromCamera(pointer, stage.camera);
      const found = raycaster.intersectObject(whole.group, true)[0];
      if (found) {
        const y = found.point.y;
        const lv = model.levels.find((x) => y >= x.y && y < x.y + x.h);
        if (lv && lv.level >= 2) chooseLevel(lv.level);
      }
    }
  });

  loading?.remove();
  root.classList.add("is-ready");

  // Draw only while the explorer is on screen.
  const observer = new IntersectionObserver(([entry]) => stage.setVisible(!!entry?.isIntersecting && !document.hidden), { threshold: 0.01 });
  observer.observe(stageEl);
  document.addEventListener("visibilitychange", () => stage.setVisible(!document.hidden));
  window.addEventListener("pagehide", (event) => {
    if (event.persisted) return;
    observer.disconnect();
    controls.dispose();
    whole.dispose();
    tower.dispose();
    context.dispose();
    boxGeometry.dispose();
    textures.forEach((t) => t.dispose());
    Object.values(mats).forEach((m) => m.dispose());
    stage.dispose();
  });

  if (capture) {
    const w = window as unknown as Record<string, unknown>;
    w.__orielExplore = { chooseLevel, pick, closeCard, stage };
    document.documentElement.dataset.capture = "ready";
  }

  if (x.start) pick(x.start);
}
