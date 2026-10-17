// The renderer, camera and light, and a render loop that only runs while
// something is moving and the scene can be seen.

import * as THREE from "three";
import { sunDirection, sunPosition, type SunPosition } from "./sun";

export interface StageOptions {
  /** Phones and modest machines: lower resolution, smaller shadows, 30 frames a second. */
  lite: boolean;
}

export interface Orbit {
  /** Compass bearing from the building to the camera, in degrees. */
  azimuth: number;
  /** Degrees above the horizon. */
  elevation: number;
  distance: number;
  target: THREE.Vector3;
}

const PLASTER = 0xecedea;

export class Stage {
  readonly renderer: THREE.WebGLRenderer;
  readonly scene = new THREE.Scene();
  readonly camera: THREE.PerspectiveCamera;
  readonly sun: THREE.DirectionalLight;
  readonly hemisphere: THREE.HemisphereLight;
  /** Everything above this plane is hidden while the building rises. */
  readonly clip = new THREE.Plane(new THREE.Vector3(0, -1, 0), 10_000);
  readonly orbit: Orbit = { azimuth: 150, elevation: 10, distance: 160, target: new THREE.Vector3(0, 28, 0) };
  /** Moves the building sideways in the frame, as a fraction of the width: 0.2 puts it right of centre. */
  shift = 0;
  sunNow: SunPosition = { azimuth: 100, altitude: 30 };

  private readonly canvas: HTMLCanvasElement;
  private readonly lite: boolean;
  private readonly envTexture: THREE.Texture;
  private readonly resizeObserver: ResizeObserver;
  private width = 1;
  private height = 1;
  private frame = 0;
  private dirty = true;
  private visible = true;
  private lastFrame = 0;
  private tickers = new Set<(dt: number, now: number) => boolean>();

  constructor(canvas: HTMLCanvasElement, options: StageOptions) {
    this.canvas = canvas;
    this.lite = options.lite;
    this.renderer = new THREE.WebGLRenderer({
      canvas,
      antialias: !options.lite,
      alpha: false,
      powerPreference: options.lite ? "low-power" : "high-performance",
      stencil: false,
    });
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, options.lite ? 1.5 : 2));
    this.renderer.outputColorSpace = THREE.SRGBColorSpace;
    this.renderer.toneMapping = THREE.ACESFilmicToneMapping;
    this.renderer.toneMappingExposure = 1.02;
    this.renderer.shadowMap.enabled = true;
    this.renderer.shadowMap.type = THREE.PCFShadowMap;
    this.renderer.localClippingEnabled = true;

    this.scene.background = new THREE.Color(PLASTER);
    this.scene.fog = new THREE.Fog(PLASTER, 280, 720);

    // The glass reflects a soft sky: blue above, a pale horizon, the ground below.
    const pmrem = new THREE.PMREMGenerator(this.renderer);
    const sky = skyScene();
    this.envTexture = pmrem.fromScene(sky.scene, 0.02).texture;
    sky.dispose();
    pmrem.dispose();
    this.scene.environment = this.envTexture;
    this.scene.environmentIntensity = 0.85;

    this.camera = new THREE.PerspectiveCamera(28, 1, 1, 1600);

    this.hemisphere = new THREE.HemisphereLight(0xf1f4f7, 0xd6d1c6, 1.15);
    this.scene.add(this.hemisphere);

    this.sun = new THREE.DirectionalLight(0xfff0dc, 2.7);
    this.sun.castShadow = true;
    const size = options.lite ? 1024 : 2048;
    this.sun.shadow.mapSize.set(size, size);
    const cam = this.sun.shadow.camera;
    cam.left = -75;
    cam.right = 75;
    cam.top = 75;
    cam.bottom = -75;
    cam.near = 10;
    cam.far = 460;
    this.sun.shadow.radius = options.lite ? 1.5 : 3;
    this.sun.shadow.bias = -0.0004;
    this.sun.shadow.normalBias = 0.04;
    this.scene.add(this.sun, this.sun.target);

    this.resizeObserver = new ResizeObserver(() => this.resize());
    this.resizeObserver.observe(canvas.parentElement ?? canvas);
    this.resize();
  }

  /** Puts the sun where it is at a moment, over the site. */
  setSun(date: Date, lat: number, lon: number): void {
    this.sunNow = sunPosition(date, lat, lon);
    this.placeSun();
  }

  placeSun(): void {
    const altitude = Math.max(this.sunNow.altitude, 4);
    const [x, y, z] = sunDirection({ azimuth: this.sunNow.azimuth, altitude });
    this.sun.target.position.set(0, 0, 0);
    this.sun.position.set(x * 220, y * 220, z * 220);
    this.sun.target.updateMatrixWorld();
    // Warmer and dimmer when the sun is low.
    const lowness = 1 - Math.min(1, altitude / 50);
    this.sun.color.setRGB(1, 0.94 - lowness * 0.12, 0.86 - lowness * 0.26);
    this.sun.intensity = 2.2 + Math.min(1, altitude / 40) * 0.8;
    this.requestRender();
  }

  /** Points the camera from its orbit, with the sideways shift applied. */
  applyCamera(): void {
    this.positionFromOrbit();
    this.updateProjection();
  }

  /** Where the camera stands for an orbit. */
  orbitPosition(o: Orbit = this.orbit): THREE.Vector3 {
    const az = (o.azimuth * Math.PI) / 180;
    const el = (o.elevation * Math.PI) / 180;
    return new THREE.Vector3(
      o.target.x + o.distance * Math.cos(el) * Math.sin(az),
      o.target.y + o.distance * Math.sin(el),
      o.target.z - o.distance * Math.cos(el) * Math.cos(az),
    );
  }

  positionFromOrbit(): void {
    this.camera.position.copy(this.orbitPosition());
    this.camera.lookAt(this.orbit.target);
  }

  /**
   * When something else moves the camera (the explorer's orbit controls), a
   * resize only updates the lens and leaves the camera where it is.
   */
  external = false;

  /** The lens: aspect and the sideways shift. */
  updateProjection(): void {
    const w = this.width;
    const h = this.height;
    const a = 0.5 + this.shift;
    if (Math.abs(this.shift) < 1e-3) {
      this.camera.clearViewOffset();
      this.camera.aspect = w / h;
    } else if (a > 0.5) {
      const full = 2 * a * w;
      this.camera.aspect = full / h;
      this.camera.setViewOffset(full, h, 0, 0, w, h);
    } else {
      const full = 2 * (1 - a) * w;
      this.camera.aspect = full / h;
      this.camera.setViewOffset(full, h, full - w, 0, w, h);
    }
    this.camera.updateProjectionMatrix();
    this.requestRender();
  }

  /** The distance at which a building of this size fills the frame. */
  fitDistance(height: number, width: number, fill = 0.8): number {
    const vTan = Math.tan(((this.camera.fov / 2) * Math.PI) / 180);
    const aspect = this.width / this.height;
    const byHeight = height / 2 / vTan / fill;
    const byWidth = width / 2 / (vTan * aspect) / fill;
    return Math.max(byHeight, byWidth);
  }

  /** The stage's size in CSS pixels. */
  get box(): { width: number; height: number } {
    return { width: this.width, height: this.height };
  }

  get aspect(): number {
    return this.width / this.height;
  }

  resize(): void {
    const box = (this.canvas.parentElement ?? this.canvas).getBoundingClientRect();
    const w = Math.max(1, Math.round(box.width));
    const h = Math.max(1, Math.round(box.height));
    if (w === this.width && h === this.height) return;
    this.width = w;
    this.height = h;
    this.renderer.setSize(w, h, false);
    this.canvas.style.width = w + "px";
    this.canvas.style.height = h + "px";
    this.onResize?.();
    if (this.external) this.updateProjection();
    else this.applyCamera();
  }

  onResize?: () => void;

  requestRender(): void {
    this.dirty = true;
    this.schedule();
  }

  /** Runs fn every frame until it returns false. */
  addTicker(fn: (dt: number, now: number) => boolean): void {
    this.tickers.add(fn);
    this.schedule();
  }

  setVisible(visible: boolean): void {
    this.visible = visible;
    if (visible) this.schedule();
  }

  private schedule(): void {
    if (this.frame || !this.visible) return;
    this.frame = requestAnimationFrame((now) => this.loop(now));
  }

  private loop(now: number): void {
    this.frame = 0;
    if (!this.visible) return;
    const minGap = this.lite ? 1000 / 31 : 0;
    if (now - this.lastFrame < minGap) {
      this.schedule();
      return;
    }
    const dt = this.lastFrame ? Math.min(0.1, (now - this.lastFrame) / 1000) : 0.016;
    this.lastFrame = now;
    for (const fn of this.tickers) {
      if (!fn(dt, now)) this.tickers.delete(fn);
      this.dirty = true;
    }
    if (this.dirty) {
      this.dirty = false;
      this.renderer.render(this.scene, this.camera);
    }
    if (this.tickers.size > 0) this.schedule();
    else this.lastFrame = 0;
  }

  /** Draws one frame now, whatever the loop is doing. */
  renderNow(): void {
    this.dirty = false;
    this.renderer.render(this.scene, this.camera);
  }

  dispose(): void {
    if (this.frame) cancelAnimationFrame(this.frame);
    this.tickers.clear();
    this.resizeObserver.disconnect();
    this.envTexture.dispose();
    this.sun.shadow.map?.dispose();
    this.renderer.dispose();
    this.renderer.forceContextLoss();
  }
}

function skyScene(): { scene: THREE.Scene; dispose(): void } {
  const scene = new THREE.Scene();
  const geometry = new THREE.SphereGeometry(10, 48, 24);
  const material = new THREE.ShaderMaterial({
    side: THREE.BackSide,
    depthWrite: false,
    uniforms: {
      zenith: { value: new THREE.Color(0x7fa6c9) },
      horizon: { value: new THREE.Color(0xf3f1ea) },
      ground: { value: new THREE.Color(0xa8a59c) },
    },
    vertexShader: "varying vec3 vDir; void main() { vDir = normalize(position); gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }",
    fragmentShader:
      "uniform vec3 zenith; uniform vec3 horizon; uniform vec3 ground; varying vec3 vDir;" +
      "void main() { float h = vDir.y; vec3 c = h > 0.0 ? mix(horizon, zenith, pow(h, 0.55)) : mix(horizon, ground, pow(-h, 0.35)); gl_FragColor = vec4(c, 1.0); }",
  });
  scene.add(new THREE.Mesh(geometry, material));
  return {
    scene,
    dispose() {
      geometry.dispose();
      material.dispose();
    },
  };
}

export { isSoftwareRenderer, rendererName, supportsWebGL } from "./capable";

/** A lighter scene for phones and modest machines. */
export function preferLite(): boolean {
  const nav = navigator as Navigator & { deviceMemory?: number };
  const memory = nav.deviceMemory ?? 8;
  const cores = nav.hardwareConcurrency ?? 8;
  return memory <= 4 || cores <= 4 || window.matchMedia("(max-width: 700px)").matches;
}
