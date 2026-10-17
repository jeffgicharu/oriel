// The sales desk's script.

import htmx from "htmx.org";

declare global {
  interface Window {
    htmx: typeof htmx;
  }
}

window.htmx = htmx;
htmx.config.defaultSwapStyle = "outerHTML";
htmx.config.includeIndicatorStyles = false;
htmx.config.allowEval = false;
htmx.config.allowScriptTags = false;
