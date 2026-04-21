/*! pingcast incident-banner widget — vanilla JS, zero deps */
(function () {
  "use strict";

  // Extract slug + config from the <script> tag that loaded us.
  var scripts = document.getElementsByTagName("script");
  var self = null;
  for (var i = 0; i < scripts.length; i++) {
    var src = scripts[i].src || "";
    if (src.indexOf("/widget.js") > -1) {
      self = scripts[i];
      break;
    }
  }
  if (!self) return;

  var slug = self.getAttribute("data-slug");
  if (!slug) return;

  var position = self.getAttribute("data-position") || "top"; // top | bottom
  var theme = self.getAttribute("data-theme") || "auto";      // auto | light | dark
  var apiBase = self.getAttribute("data-api") || inferApiBase(self.src);

  function inferApiBase(scriptSrc) {
    // widget.js lives at https://pingcast.io/widget.js → API at same origin.
    try {
      var u = new URL(scriptSrc);
      return u.origin;
    } catch (e) {
      return "";
    }
  }

  // Public status endpoint — same shape the status page uses.
  var url = apiBase + "/api/status/" + encodeURIComponent(slug);

  fetch(url, { credentials: "omit" })
    .then(function (r) {
      if (!r.ok) throw new Error("status " + r.status);
      return r.json();
    })
    .then(function (data) {
      if (!data || data.all_up === true) return;

      var openIncidents = (data.incidents || []).filter(function (inc) {
        return !inc.resolved_at;
      });
      if (openIncidents.length === 0) return;

      injectBanner(openIncidents[0], data, openIncidents.length);
    })
    .catch(function () {
      // Silent: never break the host page because our status API is down.
    });

  function injectBanner(incident, data, count) {
    if (document.getElementById("pingcast-incident-banner")) return;

    var banner = document.createElement("div");
    banner.id = "pingcast-incident-banner";
    banner.setAttribute("role", "status");
    banner.setAttribute("aria-live", "polite");

    var isDark =
      theme === "dark" ||
      (theme === "auto" &&
        window.matchMedia &&
        window.matchMedia("(prefers-color-scheme: dark)").matches);

    var bg = isDark ? "#1c1917" : "#fef3c7";
    var fg = isDark ? "#fef3c7" : "#78350f";
    var accent = isDark ? "#fbbf24" : "#d97706";

    banner.style.cssText = [
      "position:fixed",
      position === "bottom" ? "bottom:0" : "top:0",
      "left:0",
      "right:0",
      "z-index:2147483646",
      "background:" + bg,
      "color:" + fg,
      "padding:10px 16px",
      "font:500 14px/1.4 system-ui,-apple-system,BlinkMacSystemFont,Segoe UI,Roboto,sans-serif",
      "display:flex",
      "align-items:center",
      "justify-content:center",
      "gap:12px",
      "box-shadow:0 2px 8px rgba(0,0,0,0.08)",
      "border-" + (position === "bottom" ? "top" : "bottom") + ":1px solid " + accent,
    ].join(";");

    var dot = document.createElement("span");
    dot.style.cssText =
      "display:inline-block;width:8px;height:8px;border-radius:999px;background:" +
      accent +
      ";flex-shrink:0";
    banner.appendChild(dot);

    var label = document.createElement("span");
    var title = incident.title || incident.cause || "Incident in progress";
    var extra =
      count > 1 ? " (+" + (count - 1) + " more)" : "";
    label.textContent = title + extra;
    banner.appendChild(label);

    var link = document.createElement("a");
    link.href = apiBase.replace(/\/api.*$/, "") + "/status/" + encodeURIComponent(slug);
    link.textContent = "View status →";
    link.target = "_blank";
    link.rel = "noopener";
    link.style.cssText =
      "color:" + accent + ";text-decoration:underline;margin-left:auto;white-space:nowrap";
    banner.appendChild(link);

    var close = document.createElement("button");
    close.textContent = "×";
    close.setAttribute("aria-label", "Dismiss");
    close.style.cssText =
      "background:transparent;border:0;color:" +
      fg +
      ";font-size:20px;line-height:1;padding:0 4px;cursor:pointer;flex-shrink:0";
    close.onclick = function () {
      banner.parentNode && banner.parentNode.removeChild(banner);
    };
    banner.appendChild(close);

    document.body.appendChild(banner);
  }
})();
