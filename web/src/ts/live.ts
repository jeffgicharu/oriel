// Small live behaviours: forms that ask before they act, and a reservation
// page that refreshes the moment the sales team confirms it.

export function setUpConfirms(root: ParentNode = document): void {
  root.querySelectorAll<HTMLFormElement>("form[data-confirm]").forEach((form) => {
    form.addEventListener("submit", (event) => {
      if (!window.confirm(form.dataset.confirm || "Are you sure?")) event.preventDefault();
    });
  });
}

export function setUpLiveReservation(): void {
  const page = document.querySelector<HTMLElement>("[data-live-reservation]");
  if (!page || !("EventSource" in window)) return;
  const ref = page.dataset.liveReservation;
  const source = new EventSource("/events");
  source.addEventListener("change", (event) => {
    try {
      const data = JSON.parse((event as MessageEvent).data) as { type: string; id?: string };
      if (data.type === "reservation" && data.id === ref) {
        source.close();
        location.reload();
      }
    } catch {
      // Ignore anything unexpected on the stream.
    }
  });
  window.addEventListener("pagehide", () => source.close());
}
