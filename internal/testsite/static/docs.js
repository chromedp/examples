// Opens the example that the address names, so a link to an example works.
(function () {
  "use strict";
  function openFromHash() {
    var id = decodeURIComponent(location.hash.slice(1));
    if (!id) {
      return;
    }
    var el = document.getElementById(id);
    if (el && el.tagName === "DETAILS") {
      el.open = true;
    }
  }
  window.addEventListener("hashchange", openFromHash);
  openFromHash();
})();
