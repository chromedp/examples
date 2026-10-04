// Shows what the browser reports about itself.
(function () {
  "use strict";
  function set(id, value) {
    var el = document.getElementById(id);
    if (el) {
      el.textContent = String(value);
    }
  }
  function update() {
    set("ua", navigator.userAgent);
    set("platform", navigator.platform || "unknown");
    set("language", navigator.language);
    set("viewport", window.innerWidth + " x " + window.innerHeight + " px");
    set("screen", screen.width + " x " + screen.height + " px");
    set("dpr", window.devicePixelRatio);
    set("touch", navigator.maxTouchPoints + (navigator.maxTouchPoints > 0 ? " (touch device)" : " (no touch)"));
    set("orientation", window.innerWidth >= window.innerHeight ? "landscape" : "portrait");
    set("scheme", window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
    set("mobile", window.matchMedia("(max-width: 720px)").matches ? "yes, narrow layout" : "no, wide layout");
    document.body.dataset.ready = "true";
  }
  window.addEventListener("resize", update);
  update();
})();
