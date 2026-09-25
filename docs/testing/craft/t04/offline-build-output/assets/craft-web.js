/* craft-web.js v1.1.0 — provisioned offline runtime of the Craft web
 * toolchain. It performs local DOM filtering only: no fetch, no XHR, no
 * WebSocket, no EventSource, no dynamic script/style loading, no eval. */
(function () {
  "use strict";

  function normalize(text) {
    return String(text === null || text === undefined ? "" : text).trim().toLowerCase();
  }

  function filterTable(input, table) {
    const needle = normalize(input.value);
    const rows = table.querySelectorAll("tbody tr");
    for (const row of rows) {
      const haystack = normalize(row.textContent);
      const hit = needle === "" || haystack.indexOf(needle) !== -1;
      if (hit) {
        row.removeAttribute("data-craft-filter-hide");
      } else {
        row.setAttribute("data-craft-filter-hide", "1");
      }
    }
  }

  function boot() {
    // build.py renders one filter input immediately before each table, so
    // every table pairs with its own controlling input (all matches, not
    // just the first — later tables used to be dead controls).
    const tables = document.querySelectorAll("table.craft-table");
    for (const table of tables) {
      const input = table.previousElementSibling;
      if (input !== null && input.classList !== undefined && input.classList.contains("craft-filter")) {
        input.addEventListener("input", function () { filterTable(input, table); });
      }
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", boot);
  } else {
    boot();
  }
})();
