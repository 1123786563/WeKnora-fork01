/* craft-web.js v1.0.0 — provisioned offline runtime of the Craft web
 * toolchain. It performs local DOM filtering only: no fetch, no XHR, no
 * WebSocket, no EventSource, no dynamic script/style loading, no eval. */
(function () {
  "use strict";

  function normalize(text) {
    return String(text == null ? "" : text).trim().toLowerCase();
  }

  function filterTable(input, table) {
    var needle = normalize(input.value);
    var rows = table.querySelectorAll("tbody tr");
    for (var i = 0; i < rows.length; i++) {
      var row = rows[i];
      var haystack = normalize(row.textContent);
      var hit = needle === "" || haystack.indexOf(needle) !== -1;
      if (hit) {
        row.removeAttribute("data-craft-filter-hide");
      } else {
        row.setAttribute("data-craft-filter-hide", "1");
      }
    }
  }

  function boot() {
    var input = document.querySelector(".craft-filter");
    var table = document.querySelector("table.craft-table");
    if (!input || !table) { return; }
    input.addEventListener("input", function () { filterTable(input, table); });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", boot);
  } else {
    boot();
  }
})();
