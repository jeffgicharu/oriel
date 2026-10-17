// Whether this device should draw the building in 3D, decided before any 3D
// code is fetched: a page that will show the drawn elevation instead never
// downloads or compiles three.js.

/** Whether the browser can make a WebGL context at all. */
export function supportsWebGL(): boolean {
  try {
    const canvas = document.createElement("canvas");
    return !!(canvas.getContext("webgl2") || canvas.getContext("webgl"));
  } catch {
    return false;
  }
}

/** The name of the device's graphics renderer, if the browser shares it. */
export function rendererName(): string {
  try {
    const canvas = document.createElement("canvas");
    const gl = (canvas.getContext("webgl2") || canvas.getContext("webgl")) as WebGLRenderingContext | null;
    if (!gl) return "";
    const info = gl.getExtension("WEBGL_debug_renderer_info");
    const name = String(info ? gl.getParameter(info.UNMASKED_RENDERER_WEBGL) : gl.getParameter(gl.RENDERER));
    gl.getExtension("WEBGL_lose_context")?.loseContext();
    return name;
  } catch {
    return "";
  }
}

/** Whether 3D would be drawn by the processor instead of a graphics chip. */
export function isSoftwareRenderer(name: string): boolean {
  return /swiftshader|llvmpipe|softpipe|software|basic render/i.test(name);
}

/**
 * Whether to draw in 3D: there must be WebGL with a real graphics chip, and
 * the visitor must not have asked to save data. #capture forces it, for
 * recording the opening on a machine without a graphics chip.
 */
export function canDraw3D(): boolean {
  if (new URLSearchParams(location.hash.slice(1)).has("capture")) return true;
  const nav = navigator as Navigator & { connection?: { saveData?: boolean } };
  if (nav.connection?.saveData || !supportsWebGL()) return false;
  return !isSoftwareRenderer(rendererName());
}
