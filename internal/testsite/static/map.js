// A small tile map. It lays out 256 pixel tiles with absolute positions, and it
// supports dragging, the zoom buttons, the wheel, the keyboard and the address.
// The center of the map goes into the address in the form
// ?lat=...&lon=...&zoom=...#@lat,lon,zoomz and into location.hash.
(function () {
  "use strict";
  var TILE = 256;
  var MIN_ZOOM = 1;
  var MAX_ZOOM = 18;
  var mapEl = document.getElementById("map");
  var layer = document.getElementById("tile-layer");
  var coords = document.getElementById("coords");
  var form = document.getElementById("map-form");
  if (!mapEl || !layer) {
    return;
  }
  var state = {
    lat: parseFloat(mapEl.dataset.lat),
    lon: parseFloat(mapEl.dataset.lon),
    zoom: parseFloat(mapEl.dataset.zoom)
  };
  var tiles = new Map();
  var pending = 0;
  var urlTimer = null;

  function clamp(v, lo, hi) {
    return Math.max(lo, Math.min(hi, v));
  }

  function project(lat, lon, z) {
    var n = Math.pow(2, z) * TILE;
    var s = Math.sin(clamp(lat, -85, 85) * Math.PI / 180);
    return {
      x: (lon + 180) / 360 * n,
      y: (0.5 - Math.log((1 + s) / (1 - s)) / (4 * Math.PI)) * n
    };
  }

  function unproject(x, y, z) {
    var n = Math.pow(2, z) * TILE;
    var t = Math.PI * (1 - 2 * y / n);
    return {
      lat: 180 / Math.PI * Math.atan(Math.sinh(t)),
      lon: x / n * 360 - 180
    };
  }

  function normalize() {
    state.zoom = clamp(state.zoom, MIN_ZOOM, MAX_ZOOM);
    state.lat = clamp(state.lat, -84, 84);
    while (state.lon > 180) { state.lon -= 360; }
    while (state.lon < -180) { state.lon += 360; }
  }

  function setPending(delta) {
    pending += delta;
    mapEl.dataset.loading = String(pending);
  }

  function render() {
    normalize();
    var w = mapEl.clientWidth;
    var h = mapEl.clientHeight;
    var z0 = Math.floor(state.zoom);
    var scale = Math.pow(2, state.zoom - z0);
    var c = project(state.lat, state.lon, z0);
    var left = c.x - w / (2 * scale);
    var top = c.y - h / (2 * scale);
    var n = Math.pow(2, z0);
    var wanted = new Set();
    var x1 = Math.floor((left + w / scale) / TILE);
    var y1 = Math.floor((top + h / scale) / TILE);
    for (var tx = Math.floor(left / TILE); tx <= x1; tx++) {
      for (var ty = Math.floor(top / TILE); ty <= y1; ty++) {
        if (ty < 0 || ty >= n) {
          continue;
        }
        var wrapped = ((tx % n) + n) % n;
        var key = z0 + "/" + tx + "/" + ty;
        wanted.add(key);
        var img = tiles.get(key);
        if (!img) {
          img = document.createElement("img");
          img.alt = "";
          img.draggable = false;
          img.dataset.tile = z0 + "/" + wrapped + "/" + ty;
          setPending(1);
          img.addEventListener("load", function () { setPending(-1); });
          img.addEventListener("error", function () { setPending(-1); });
          img.src = "/tiles/" + z0 + "/" + wrapped + "/" + ty + ".png";
          layer.appendChild(img);
          tiles.set(key, img);
        }
        img.style.width = TILE * scale + 0.5 + "px";
        img.style.height = TILE * scale + 0.5 + "px";
        img.style.left = (tx * TILE - left) * scale + "px";
        img.style.top = (ty * TILE - top) * scale + "px";
      }
    }
    tiles.forEach(function (img, key) {
      if (!wanted.has(key)) {
        img.remove();
        tiles.delete(key);
      }
    });
    coords.textContent = "lat " + state.lat.toFixed(5) + ", lon " + state.lon.toFixed(5) + ", zoom " + state.zoom.toFixed(2);
    form.elements.lat.value = state.lat.toFixed(4);
    form.elements.lon.value = state.lon.toFixed(4);
    form.elements.zoom.value = state.zoom.toFixed(1);
    scheduleUrl();
  }

  function fragment() {
    return "@" + state.lat.toFixed(5) + "," + state.lon.toFixed(5) + "," + state.zoom.toFixed(2) + "z";
  }

  function writeUrl() {
    var query = "?lat=" + state.lat.toFixed(5) + "&lon=" + state.lon.toFixed(5) + "&zoom=" + state.zoom.toFixed(2);
    history.replaceState(null, "", location.pathname + query + "#" + fragment());
  }

  function scheduleUrl() {
    clearTimeout(urlTimer);
    urlTimer = setTimeout(writeUrl, 120);
  }

  function moveByPixels(dx, dy) {
    var z0 = Math.floor(state.zoom);
    var scale = Math.pow(2, state.zoom - z0);
    var c = project(state.lat, state.lon, z0);
    var p = unproject(c.x - dx / scale, c.y - dy / scale, z0);
    state.lat = p.lat;
    state.lon = p.lon;
    render();
  }

  function zoomBy(delta) {
    state.zoom = clamp(state.zoom + delta, MIN_ZOOM, MAX_ZOOM);
    render();
  }

  var drag = null;
  mapEl.addEventListener("pointerdown", function (ev) {
    if (ev.target.closest(".map-controls")) {
      return;
    }
    drag = { x: ev.clientX, y: ev.clientY, id: ev.pointerId };
    mapEl.setPointerCapture(ev.pointerId);
    mapEl.classList.add("dragging");
  });
  mapEl.addEventListener("pointermove", function (ev) {
    if (!drag || drag.id !== ev.pointerId) {
      return;
    }
    moveByPixels(ev.clientX - drag.x, ev.clientY - drag.y);
    drag.x = ev.clientX;
    drag.y = ev.clientY;
  });
  function endDrag(ev) {
    if (drag && drag.id === ev.pointerId) {
      drag = null;
      mapEl.classList.remove("dragging");
    }
  }
  mapEl.addEventListener("pointerup", endDrag);
  mapEl.addEventListener("pointercancel", endDrag);

  mapEl.addEventListener("wheel", function (ev) {
    ev.preventDefault();
    zoomBy(ev.deltaY < 0 ? 0.5 : -0.5);
  }, { passive: false });
  mapEl.addEventListener("dblclick", function (ev) {
    if (!ev.target.closest(".map-controls")) {
      zoomBy(1);
    }
  });
  mapEl.addEventListener("keydown", function (ev) {
    var step = 80;
    switch (ev.key) {
      case "ArrowLeft": moveByPixels(step, 0); break;
      case "ArrowRight": moveByPixels(-step, 0); break;
      case "ArrowUp": moveByPixels(0, step); break;
      case "ArrowDown": moveByPixels(0, -step); break;
      case "+": case "=": zoomBy(1); break;
      case "-": case "_": zoomBy(-1); break;
      default: return;
    }
    ev.preventDefault();
  });
  document.getElementById("zoom-in").addEventListener("click", function () { zoomBy(1); });
  document.getElementById("zoom-out").addEventListener("click", function () { zoomBy(-1); });
  window.addEventListener("resize", render);

  // A change of the fragment from outside moves the map.
  window.addEventListener("hashchange", function () {
    var m = /^#@(-?\d+(?:\.\d+)?),(-?\d+(?:\.\d+)?),(\d+(?:\.\d+)?)z$/.exec(location.hash);
    if (m) {
      state.lat = parseFloat(m[1]);
      state.lon = parseFloat(m[2]);
      state.zoom = parseFloat(m[3]);
      render();
    }
  });

  // The form moves the map without a reload.
  form.addEventListener("submit", function (ev) {
    ev.preventDefault();
    var lat = parseFloat(form.elements.lat.value);
    var lon = parseFloat(form.elements.lon.value);
    var zoom = parseFloat(form.elements.zoom.value);
    if (isFinite(lat) && isFinite(lon) && isFinite(zoom)) {
      state.lat = lat;
      state.lon = lon;
      state.zoom = zoom;
      render();
    }
  });

  render();
  writeUrl();
})();
