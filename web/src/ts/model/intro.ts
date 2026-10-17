// The opening: the building is drawn as an architect would draw it, floor by
// floor in perspective, and then rises out of its own drawing in concrete and
// glass while the camera settles.

import gsap from "gsap";
import type { Stage } from "./stage";
import type { Tower } from "./tower";

export interface IntroOptions {
  /** Where the camera ends, after easing in from a little closer and lower. */
  azimuth: number;
  elevation: number;
  distance: number;
  /** The outline left on the finished building, from 0 (none) to 1. */
  restingLines: number;
}

export function showFinished(stage: Stage, tower: Tower, opt: IntroOptions): void {
  tower.lines.geometry.setDrawRange(0, Infinity);
  tower.lineMaterial.opacity = opt.restingLines;
  stage.clip.constant = 10_000;
  stage.orbit.azimuth = opt.azimuth;
  stage.orbit.elevation = opt.elevation;
  stage.orbit.distance = opt.distance;
  stage.applyCamera();
}

/** The opening as a paused timeline, to play or to step through frame by frame. */
export function buildIntro(stage: Stage, tower: Tower, opt: IntroOptions): gsap.core.Timeline {
  const geometry = tower.lines.geometry;
  const total = tower.levelEnds[tower.levelEnds.length - 1] ?? geometry.getAttribute("position").count;
  const state = {
    drawn: 0,
    rise: -0.5,
    lines: 1,
    azimuth: opt.azimuth - 24,
    elevation: opt.elevation + 7,
    distance: opt.distance * 0.82,
  };
  const apply = () => {
    const count = Math.floor(state.drawn);
    geometry.setDrawRange(0, count - (count % 2));
    stage.clip.constant = state.rise;
    tower.lineMaterial.opacity = state.lines;
    stage.orbit.azimuth = state.azimuth;
    stage.orbit.elevation = state.elevation;
    stage.orbit.distance = state.distance;
    stage.applyCamera();
  };
  apply();
  return gsap
    .timeline({ paused: true, onUpdate: apply })
    .to(state, { drawn: total, duration: 3.2, ease: "power1.inOut" }, 0)
    .to(state, { rise: tower.height + 0.5, duration: 3.0, ease: "power2.inOut" }, 2.0)
    .to(state, { lines: opt.restingLines, duration: 1.6, ease: "power1.out" }, 4.4)
    .to(state, { azimuth: opt.azimuth, elevation: opt.elevation, distance: opt.distance, duration: 6.2, ease: "power3.out" }, 0);
}

/** Plays the opening, keeping the stage drawing every frame while it runs. */
export function playIntro(stage: Stage, timeline: gsap.core.Timeline): Promise<void> {
  return new Promise((resolve) => {
    let running = true;
    stage.addTicker(() => running);
    timeline.eventCallback("onComplete", () => {
      running = false;
      resolve();
    });
    timeline.play(0);
  });
}
