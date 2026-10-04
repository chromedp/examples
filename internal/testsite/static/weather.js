// The tabs, the unit switch and the day buttons of the forecast. Each control
// sets an attribute of the section #wob_wc, and the style sheet does the rest.
(function () {
  "use strict";
  var root = document.getElementById("wob_wc");
  if (!root) {
    return;
  }
  var tabs = Array.prototype.slice.call(root.querySelectorAll("[role='tab']"));
  var dayButtons = Array.prototype.slice.call(root.querySelectorAll(".wob-day"));
  var unitSwitches = Array.prototype.slice.call(root.querySelectorAll(".wob-unit"));

  function setType(type) {
    root.dataset.type = type;
    tabs.forEach(function (tab) {
      var on = tab.dataset.type === type;
      tab.setAttribute("aria-selected", on ? "true" : "false");
      tab.tabIndex = on ? 0 : -1;
    });
  }

  function setUnit(unit) {
    root.dataset.unit = unit;
    unitSwitches.forEach(function (el) {
      var on = el.classList.contains(unit === "c" ? "unit-c" : "unit-f");
      el.setAttribute("aria-pressed", on ? "true" : "false");
    });
  }

  function setDay(day) {
    root.dataset.day = String(day);
    dayButtons.forEach(function (btn) {
      btn.setAttribute("aria-pressed", btn.dataset.wobDi === String(day) ? "true" : "false");
    });
  }

  tabs.forEach(function (tab, i) {
    tab.addEventListener("click", function () { setType(tab.dataset.type); });
    tab.addEventListener("keydown", function (ev) {
      var next = ev.key === "ArrowRight" ? i + 1 : ev.key === "ArrowLeft" ? i - 1 : -1;
      if (next >= 0 && next < tabs.length) {
        setType(tabs[next].dataset.type);
        tabs[next].focus();
      }
    });
  });

  unitSwitches.forEach(function (el) {
    function choose() { setUnit(el.classList.contains("unit-c") ? "c" : "f"); }
    el.addEventListener("click", choose);
    el.addEventListener("keydown", function (ev) {
      if (ev.key === "Enter" || ev.key === " ") {
        ev.preventDefault();
        choose();
      }
    });
  });

  dayButtons.forEach(function (btn) {
    btn.addEventListener("click", function () { setDay(btn.dataset.wobDi); });
  });
  root.querySelectorAll("tr[data-row]").forEach(function (row) {
    row.addEventListener("click", function () { setDay(row.dataset.row); });
  });

  setType(root.dataset.type);
  setUnit(root.dataset.unit);
  setDay(root.dataset.day);
})();
