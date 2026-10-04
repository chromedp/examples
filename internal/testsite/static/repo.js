// The Code menu of the repository page: the button shows and hides the dropdown.
(function () {
  "use strict";
  var button = document.querySelector(".code-button");
  var menu = document.getElementById("code-dropdown");
  if (!button || !menu) {
    return;
  }
  function set(open) {
    menu.hidden = !open;
    button.setAttribute("aria-expanded", open ? "true" : "false");
  }
  button.addEventListener("click", function (ev) {
    ev.stopPropagation();
    set(menu.hidden);
  });
  document.addEventListener("click", function (ev) {
    if (!menu.hidden && !menu.contains(ev.target)) {
      set(false);
    }
  });
  document.addEventListener("keydown", function (ev) {
    if (ev.key === "Escape") {
      set(false);
    }
  });
})();
