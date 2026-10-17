// The pipeline board: drag a lead between stages, or within one, and the
// desk saves where it landed. Dropping on Lost asks for a reason first.

import Sortable from "sortablejs";

function idOf(el: Element | null | undefined): string {
  return (el as HTMLElement | null)?.dataset?.id || "";
}

async function save(id: string, stage: string, after: string, before: string, reason = ""): Promise<string> {
  const body = new URLSearchParams({ stage, after, before, reason });
  const res = await fetch(`/desk/leads/${encodeURIComponent(id)}/move`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body,
    credentials: "same-origin",
  });
  if (res.ok) return "";
  return (await res.text()) || "The lead could not be moved.";
}

function askReason(dialog: HTMLDialogElement): Promise<string | null> {
  const input = dialog.querySelector<HTMLInputElement>("input[name=reason]");
  if (input) input.value = "";
  dialog.showModal();
  return new Promise((resolve) => {
    dialog.addEventListener(
      "close",
      () => {
        const value = input?.value.trim() || "";
        resolve(dialog.returnValue === "ok" && value ? value : null);
      },
      { once: true },
    );
  });
}

export function setUpBoard(board: HTMLElement): void {
  const dialog = document.querySelector<HTMLDialogElement>("[data-lost-dialog]");
  board.querySelectorAll<HTMLElement>("[data-stage]").forEach((list) => {
    Sortable.create(list, {
      group: "leads",
      animation: 160,
      // Pointer-driven dragging behaves the same with a mouse, a pen or a finger.
      forceFallback: true,
      fallbackOnBody: true,
      fallbackTolerance: 4,
      delay: 120,
      delayOnTouchOnly: true,
      ghostClass: "is-ghost",
      chosenClass: "is-chosen",
      filter: "a",
      preventOnFilter: false,
      onEnd: async (event) => {
        const card = event.item as HTMLElement;
        const to = event.to as HTMLElement;
        const stage = to.dataset.stage || "";
        if (event.from === event.to && event.oldIndex === event.newIndex) return;
        let reason = "";
        if (stage === "lost" && (event.from as HTMLElement).dataset.stage !== "lost") {
          const answer = dialog ? await askReason(dialog) : window.prompt("Why was this lead lost?");
          if (!answer) {
            location.reload();
            return;
          }
          reason = answer;
        }
        card.classList.add("is-saving");
        const problem = await save(idOf(card), stage, idOf(card.previousElementSibling), idOf(card.nextElementSibling), reason);
        card.classList.remove("is-saving");
        if (problem) {
          window.alert(problem);
          location.reload();
          return;
        }
        // Keep the column counts right.
        board.querySelectorAll<HTMLElement>(".board__col").forEach((col) => {
          const count = col.querySelector(".board__count");
          const cards = col.querySelectorAll(".lead-card").length;
          const more = Number(col.querySelector<HTMLElement>(".board__more")?.textContent?.match(/\d+/)?.[0] || 0);
          if (count) count.textContent = String(cards + more);
        });
      },
    });
  });
}
