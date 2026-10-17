// The building on the home page. It draws itself, rises, and then turns
// slowly while it is on screen. Nothing runs while it is scrolled away or the
// tab is hidden, and it lets go of the graphics memory when the page goes.

import { buildContext } from "./context";
import { readModel } from "./data";
import { playIntro, showFinished, type IntroOptions } from "./intro";
import { preferLite, Stage, supportsWebGL } from "./stage";
import { nairobiTime } from "./sun";
import { buildTower } from "./tower";

export function mountHero(stageEl: HTMLElement): void {
  const hero = stageEl.closest<HTMLElement>(".hero");
  const model = readModel();
  const nav = navigator as Navigator & { connection?: { saveData?: boolean } };
  if (!hero || !model || !supportsWebGL() || nav.connection?.saveData) {
    hero?.classList.add("hero--flat");
    return;
  }

  const lite = preferLite();
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
    hero.classList.add("hero--flat");
    return;
  }
  const tower = buildTower(model);
  const context = buildContext(model, lite);
  for (const material of Object.values(tower.materials)) material.clippingPlanes = [stage.clip];
  stage.scene.add(context.group, tower.group);
  stage.setSun(nairobiTime(8, 40), model.lat, model.lon);

  const wide = window.matchMedia("(min-width: 900px)");
  const view = (): IntroOptions => {
    stage.shift = wide.matches ? 0.21 : 0;
    stage.orbit.target.set(0, tower.height * 0.47, 0);
    const distance = stage.fitDistance(tower.height + 4, 52, wide.matches ? 0.8 : 0.86);
    return { azimuth: 152, elevation: 9, distance, restingLines: 0.07 };
  };
  let resting = view();
  let introDone = false;
  stage.onResize = () => {
    resting = view();
    if (introDone) {
      stage.orbit.distance = resting.distance;
    }
  };

  hero.classList.add("hero--3d");
  let disposed = false;

  // Once settled, the building turns slowly; the pointer leans it a little.
  let lean = 0;
  let leanTarget = 0;
  const turn = () => {
    let last = performance.now();
    stage.addTicker((_, now) => {
      const dt = Math.min(0.1, (now - last) / 1000);
      last = now;
      resting.azimuth += dt * 3.2;
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

  if (reduced) {
    showFinished(stage, tower, resting);
    introDone = true;
  } else {
    playIntro(stage, tower, resting).then(() => {
      introDone = true;
      turn();
    });
  }

  // Only draw while the building can be seen.
  const observer = new IntersectionObserver(([entry]) => stage.setVisible(!!entry?.isIntersecting && !document.hidden), { threshold: 0.01 });
  observer.observe(stageEl);
  const onVisibility = () => stage.setVisible(!document.hidden && isOnScreen(stageEl));
  document.addEventListener("visibilitychange", onVisibility);

  window.addEventListener(
    "pagehide",
    (event) => {
      if (event.persisted) return;
      disposed = true;
      observer.disconnect();
      document.removeEventListener("visibilitychange", onVisibility);
      tower.dispose();
      context.dispose();
      stage.dispose();
    },
    { once: false },
  );
}

function isOnScreen(el: HTMLElement): boolean {
  const r = el.getBoundingClientRect();
  return r.bottom > 0 && r.top < window.innerHeight;
}
