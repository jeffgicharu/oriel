// Floor plans draw themselves the first time they come into view, and the
// furniture can be switched off to see the bare rooms.

export function setUpPlans(root: ParentNode = document): void {
  const figures = root.querySelectorAll<HTMLElement>("[data-plan]");
  if (figures.length === 0) return;

  const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  if (!reduced && "IntersectionObserver" in window) {
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (!entry.isIntersecting) continue;
          entry.target.classList.add("is-drawing");
          observer.unobserve(entry.target);
        }
      },
      { threshold: 0.25 },
    );
    figures.forEach((f) => {
      // Plans already on screen when the page opens draw straight away.
      observer.observe(f);
    });
  }

  root.querySelectorAll<HTMLInputElement>("[data-plan-furniture]").forEach((toggle) => {
    const figure = toggle.closest(".unit-plan__drawing")?.querySelector<HTMLElement>("[data-plan]");
    if (!figure) return;
    toggle.addEventListener("change", () => figure.classList.toggle("hide-furniture", !toggle.checked));
  });
}
