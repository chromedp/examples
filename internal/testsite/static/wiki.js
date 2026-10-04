// Marks the section of the article that is on the screen in the contents list.
(function () {
  "use strict";
  var toc = document.getElementById("toc");
  if (!toc || !("IntersectionObserver" in window)) {
    return;
  }
  var links = {};
  toc.querySelectorAll("a[href^='#']").forEach(function (a) {
    links[a.getAttribute("href").slice(1)] = a;
  });
  var observer = new IntersectionObserver(function (entries) {
    entries.forEach(function (entry) {
      var link = links[entry.target.id];
      if (link && entry.isIntersecting) {
        Object.keys(links).forEach(function (k) { links[k].removeAttribute("aria-current"); });
        link.setAttribute("aria-current", "location");
      }
    });
  }, { rootMargin: "-70px 0px -70% 0px" });
  Object.keys(links).forEach(function (id) {
    var el = document.getElementById(id);
    if (el) {
      observer.observe(el);
    }
  });
})();
