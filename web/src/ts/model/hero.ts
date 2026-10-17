// The building on the home page. It draws itself, rises, and then turns
// slowly while it is on screen. Nothing runs while it is scrolled away or the
// tab is hidden, and it lets go of the graphics memory when the page goes.
//
// Devices that would have to draw 3D without a graphics processor get the
// flat drawing instead, because the scene would crawl there.

import { buildContext } from "./context";
import { readModel } from "./data";
import { buildIntro, playIntro, showFinished, type IntroOptions } from "./intro";
import { isSoftwareRenderer, preferLite, rendererName, Stage, supportsWebGL } from "./stage";
import { nairobiTime } from "./sun";
import { buildTower } from "./tower";

const TURN_DEGREES_PER_SECOND = 3.2;

export function mountHero(stageEl: HTMLElement): void {
  const hero = stageEl.closest<HTMLElement>(".hero");
  const model = readModel();
  const nav = navigator as Navigator & { connection?: { saveData?: boolean } };
  const options = new URLSearchParams(location.hash.slice(1));
  // #capture steps the opening frame by frame for recording it, and draws
  // even where the device has no graphics processor.
  const capture = options.has("capture");
  const renderer = rendererName();
  const flat = () => hero?.classList.add("hero--flat");
  if (!hero || !model || !supportsWebGL() || nav.connection?.saveData) return flat();
  if (!capture && isSoftwareRenderer(renderer)) return flat();

  const lite = !capture && preferLite();
  const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  const canvas = document.createElement("canvas");
  canvas.className = "hero__canvas";
  canvas.setAttribute("aria-hidden", "true");
  stageEl.appendChild(canvas);

  let stage: Stage;
  try {
    stage = new Stage(canvas, { lite });
  } catch {
    canvas.remove();
    return flat();
  }
  const tower = buildTower(model);
  const context = buildContext(model, lite);
  for (const material of Object.values(tower.materials)) material.clippingPlanes = [stage.clip];
  stage.scene.add(context.group, tower.group);
  // Late afternoon: the sun from the west throws the shadow towards the viewer.
  stage.setSun(nairobiTime(16, 10), model.lat, model.lon);

  const wide = window.matchMedia("(min-width: 900px)");
  const view = (): IntroOptions => {
    stage.shift = wide.matches ? 0.21 : 0;
    stage.orbit.target.set(0, tower.height * 0.47, 0);
    const distance = stage.fitDistance(tower.height + 4, 52, wide.matches ? 0.76 : 0.84);
    return { azimuth: 152, elevation: 9, distance, restingLines: 0.07 };
  };
  let resting = view();
  let introDone = false;
  let disposed = false;
  stage.onResize = () => {
    const next = view();
    resting.distance = next.distance;
    if (introDone) stage.orbit.distance = resting.distance;
  };

  hero.classList.add("hero--3d");
  const intro = buildIntro(stage, tower, resting);
  const introLength = intro.duration();

  // Once settled, the building turns slowly; the pointer leans it a little.
  let lean = 0;
  let leanTarget = 0;
  const turn = () => {
    let last = performance.now();
    stage.addTicker((_, now) => {
      const dt = Math.min(0.1, (now - last) / 1000);
      last = now;
      resting.azimuth += dt * TURN_DEGREES_PER_SECOND;
      lean += (leanTarget - lean) * Math.min(1, dt * 3);
      stage.orbit.azimuth = resting.azimuth + lean;
      stage.applyCamera();
      return !disposed;
    });
  };
  if (window.matchMedia("(pointer: fine)").matches) {
    hero.addEventListener("pointermove", (event) => {
      const box = hero.getBoundingClientRect();
      leanTarget = ((event.clientX - box.left) / box.width - 0.5) * 10;
    });
  }

  const debug = window as unknown as Record<string, unknown>;
  if (capture || options.has("stats")) {
    debug.__orielStats = () => ({
      renderer,
      lite,
      calls: stage.renderer.info.render.calls,
      triangles: stage.renderer.info.render.triangles,
      lines: stage.renderer.info.render.lines,
      geometries: stage.renderer.info.memory.geometries,
      textures: stage.renderer.info.memory.textures,
      programs: stage.renderer.info.programs?.length ?? 0,
      pixelRatio: stage.renderer.getPixelRatio(),
    });
  }

  if (capture) {
    // Shows the scene exactly as it is t seconds into the opening.
    const base = resting.azimuth;
    debug.__orielSeek = (t: number) =>
      new Promise<void>((resolve) => {
        intro.seek(Math.min(t, introLength), false);
        if (t > introLength) {
          stage.orbit.azimuth = base + (t - introLength) * TURN_DEGREES_PER_SECOND;
          stage.applyCamera();
        }
        stage.renderNow();
        requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
      });
    debug.__orielIntroLength = introLength;
    intro.seek(0, false);
    stage.renderNow();
    document.documentElement.dataset.capture = "ready";
    return;
  }

  if (reduced) {
    showFinished(stage, tower, resting);
    introDone = true;
  } else {
    playIntro(stage, intro).then(() => {
      introDone = true;
      turn();
    });
  }

  // Only draw while the building can be seen.
  const observer = new IntersectionObserver(([entry]) => stage.setVisible(!!entry?.isIntersecting && !document.hidden), { threshold: 0.01 });
  observer.observe(stageEl);
  const onVisibility = () => stage.setVisible(!document.hidden && isOnScreen(stageEl));
  document.addEventListener("visibilitychange", onVisibility);

  window.addEventListener("pagehide", (event) => {
    if (event.persisted) return;
    disposed = true;
    intro.kill();
    observer.disconnect();
    document.removeEventListener("visibilitychange", onVisibility);
    tower.dispose();
    context.dispose();
    stage.dispose();
  });
}

function isOnScreen(el: HTMLElement): boolean {
  const r = el.getBoundingClientRect();
  return r.bottom > 0 && r.top < window.innerHeight;
}
