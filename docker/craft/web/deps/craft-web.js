/* craft-web.js v1.1.1 — provisioned offline runtime of the Craft web
 * toolchain. It performs local DOM filtering only: no fetch, no XHR, no
 * WebSocket, no EventSource, no dynamic script/style loading, no eval. */
(function () {
  "use strict";

  function normalize(text) {
    return String(text === null || text === undefined ? "" : text).trim().toLowerCase();
  }

  function bindTable(table) {
    // Explicit pairing: build.py emits aria-controls on the filter input so
    // the binding survives template insertions between the two nodes.
    const inputId = table.getAttribute("aria-labelledby");
    let input = null;
    if (inputId !== null && inputId !== "") {
      input = document.getElementById(inputId);
    }
    if (input === null) {
      const previous = table.previousElementSibling;
      if (previous !== null && previous.classList.contains("craft-filter")) {
        input = previous;
      }
    }
    if (input === null) {
      if (typeof console !== "undefined" && console.warn !== undefined) {
        console.warn("craft-web: table without a paired filter control");
      }
      return;
    }
    // Static rendered rows: precompute each haystack once so a keystroke
    // costs only indexOf plus attribute toggles.
    const rows = [];
    for (const row of table.querySelectorAll("tbody tr")) {
      rows.push({ row: row, haystack: normalize(row.textContent) });
    }
    input.addEventListener("input", function () {
      const needle = normalize(input.value);
      for (const entry of rows) {
        const hit = needle === "" || entry.haystack.indexOf(needle) !== -1;
        if (hit) {
          entry.row.removeAttribute("data-craft-filter-hide");
        } else {
          entry.row.setAttribute("data-craft-filter-hide", "1");
        }
      }
    });
  }

  function boot() {
    for (const table of document.querySelectorAll("table.craft-table")) {
      bindTable(table);
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", boot);
  } else {
    boot();
  }
})();
