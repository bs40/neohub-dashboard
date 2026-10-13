/* neoHub dashboard front-end. Plain JS, no build step. */
"use strict";

const $ = (s, el = document) => el.querySelector(s);
const $$ = (s, el = document) => Array.from(el.querySelectorAll(s));
const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
const LEVELS = ["wake", "leave", "return", "sleep"];
const DAYS7 = ["monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"];
const PHOTOS = [
  "/img/Autumn%20Fireside%20Retreat%20with%20Sleeping%20Dog-3.png",
  "/img/Autumn%20Glow%20by%20the%20Fireside.png",
  "/img/Autumn%20Hygge%20by%20the%20Fireside-2.png",
  "/img/Cosy%20Autumn%20Candlelit%20Living%20Room.png",
  "/img/Cosy%20Autumn%20Living%20Room%20Retreat-1.png",
  "/img/Cosy%20Rustic%20Autumn%20Kitchen%20Glow.png",
];
const ICON = {
  leaf: '<svg viewBox="0 0 24 24"><path d="M5 19c0-8 6-13 14-14-1 8-6 14-14 14Zm0 0 7-7"/></svg>',
  flame: '<svg viewBox="0 0 24 24"><path d="M12 3c1 4 5 6 5 11a5 5 0 0 1-10 0c0-2 1-3 1-3s1 2 2 2c0-3-1-5 2-10Z"/></svg>',
  clock: '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/></svg>',
  gear: '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1Z"/></svg>',
  monitor: '<svg viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="12" rx="2"/><path d="M8 20h8M12 16v4"/></svg>',
  info: '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M12 11v5M12 8h.01"/></svg>',
  pencil: '<svg viewBox="0 0 24 24"><path d="M4 20h4l10-10-4-4L4 16v4ZM13 7l4 4"/></svg>',
  power: '<svg viewBox="0 0 24 24"><path d="M12 3v9M6.3 6.3a8 8 0 1 0 11.4 0"/></svg>',
  hold: '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/></svg>',
  lock: '<svg viewBox="0 0 24 24"><rect x="5" y="11" width="14" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 8 0v4"/></svg>',
  cal: '<svg viewBox="0 0 24 24"><rect x="3" y="5" width="18" height="16" rx="2"/><path d="M3 10h18M8 3v4M16 3v4"/></svg>',
  home: '<svg viewBox="0 0 24 24"><path d="M3 11l9-7 9 7v9a1 1 0 0 1-1 1h-5v-6h-6v6H4a1 1 0 0 1-1-1v-9Z"/></svg>',
};

let status = null;
let selectedZone = null;
let refreshTimer = null;

// ---------- API ----------
async function api(path, opts) {
  const r = await fetch(path, opts);
  const j = await r.json().catch(() => ({ error: "bad JSON from server" }));
  if (!r.ok || j.error) throw new Error(j.error || `HTTP ${r.status}`);
  return j;
}
async function action(action, params = {}, okMsg) {
  try {
    const r = await api("/api/action", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ action, ...params }) });
    toast(okMsg || summarise(r.result));
    await loadStatus(true);
    return r;
  } catch (e) {
    toast("Failed: " + e.message, true);
    throw e;
  }
}
function summarise(r) {
  if (Array.isArray(r)) r = r[0];
  if (r && typeof r === "object" && "result" in r) return String(r.result);
  return "Done";
}
function toast(msg, err) {
  const t = $("#toast");
  t.textContent = msg;
  t.className = "toast show" + (err ? " err" : "");
  clearTimeout(t._t);
  t._t = setTimeout(() => (t.className = "toast"), err ? 6000 : 2500);
}
async function confirmThen(msg, fn) { if (confirm(msg)) return fn(); }

// ---------- status ----------
async function loadStatus(force) {
  try {
    status = await api("/api/status" + (force ? "?force=1" : ""));
    $("#statusdot").className = "dot";
    renderHeader();
    renderOverview();
    renderZoneSelects();
    if ($("#tab-zone").classList.contains("active")) renderZoneDetail();
    if ($("#tab-profiles").classList.contains("active")) renderProfiles();
    if ($("#tab-hub").classList.contains("active")) renderHub();
  } catch (e) {
    $("#statusdot").className = "dot bad";
    $("#statusline").textContent = "Hub unreachable";
    toast("Hub unreachable: " + e.message, true);
  }
}
const devices = () => (status?.live?.devices || []).slice().sort((a, b) => a.DEVICE_ID - b.DEVICE_ID);
const devByName = (n) => devices().find((d) => d.ZONE_NAME === n);
const engByName = (n) => status?.engineers?.[n] || {};
const profileById = (id) => Object.values(status?.profiles || {}).find((p) => p.PROFILE_ID === id);

function renderHeader() {
  const l = status.live, s = status.system;
  const all = devices();
  const heating = all.filter((d) => d.HEAT_ON).length;
  const standby = all.filter((d) => d.STANDBY).length;
  const offline = all.filter((d) => d.OFFLINE).length;
  let line, dot = "dot";
  if (offline) { line = `${offline} zone${offline > 1 ? "s" : ""} offline`; dot = "dot bad"; }
  else if (heating) line = `${heating} zone${heating > 1 ? "s" : ""}\nheating`;
  else if (standby === all.length) line = `${all.length} zones\nin standby`;
  else if (standby) line = `${standby} in standby\n${all.length - standby} idle`;
  else line = `${all.length} zones\nall idle`;
  if (l.HUB_AWAY) line += " · away";
  if (l.HUB_HOLIDAY) line += " · holiday";
  if (s.NTP_ON !== "Running" && dot === "dot") dot = "dot warn";
  $("#statusline").textContent = line;
  $("#statusdot").className = dot;

  const h = new Date().getHours();
  $("#greeting").textContent = h < 12 ? "Good morning" : h < 18 ? "Good afternoon" : "Good evening";
  let sub;
  if (offline) sub = `${offline} zone${offline > 1 ? "s are" : " is"} offline`;
  else if (heating) sub = `Warming up: ${all.filter((d) => d.HEAT_ON).map((d) => d.ZONE_NAME).slice(0, 3).join(", ")}${heating > 3 ? " and more" : ""}`;
  else if (standby === all.length) sub = "Your home is cosy and in standby";
  else if (l.HUB_HOLIDAY) sub = "Holiday mode is on";
  else if (l.HUB_AWAY) sub = "Away mode is on";
  else sub = "Your home is cosy, nothing is calling for heat";
  $("#herosub").textContent = sub;
  $("#cancelholds").hidden = !all.some((d) => d.HOLD_ON);
}
const chip = (text, cls = "", icon = "") => `<span class="chip ${cls}">${icon ? ICON[icon] : ""}${esc(text)}</span>`;

function zoneChips(d) {
  const c = [];
  if (d.OFFLINE) c.push(chip("Offline", "bad"));
  if (d.STANDBY) c.push(chip("Standby", "", "leaf"));
  else if (d.HEAT_ON) c.push(chip("Heating", "heat", "flame"));
  else c.push(chip("Idle"));
  if (d.HOLD_ON) c.push(chip(`Hold ${d.HOLD_TEMP}° for ${d.HOLD_TIME}`, "warn", "hold"));
  if (d.PREHEAT_ACTIVE) c.push(chip("Preheat", "heat"));
  if (d.ACTIVE_PROFILE) { const p = profileById(d.ACTIVE_PROFILE); c.push(chip(`Profile: ${p ? p.name : "#" + d.ACTIVE_PROFILE}`)); }
  if (d.LOCK) c.push(chip("Locked", "warn", "lock"));
  if (d.LOW_BATTERY) c.push(chip("Low battery", "bad"));
  if (d.WINDOW_OPEN) c.push(chip("Window open", "warn"));
  if (d.AWAY) c.push(chip("Away", "warn"));
  if (d.HOLIDAY) c.push(chip("Holiday", "warn"));
  return c.join("");
}

// ---------- overview ----------
function renderOverview() {
  $("#zonegrid").innerHTML = devices().map((d, i) => `
    <div class="zone ${d.OFFLINE ? "offline" : ""}" data-zone="${esc(d.ZONE_NAME)}" tabindex="0">
      <div class="inner">
        <div class="name"><span>${esc(d.ZONE_NAME)}</span><span class="id">#${d.DEVICE_ID}</span></div>
        <div class="cur">${esc(d.ACTUAL_TEMP)}°</div>
        <div class="set">Set ${esc(d.SET_TEMP)}°${d.HOLD_ON ? " · hold" : ""}</div>
        <div class="chips">${d.OFFLINE ? chip("Offline", "bad") : d.STANDBY ? chip("Standby", "", "leaf") : d.HEAT_ON ? chip("Heating", "heat", "flame") : chip("Idle")}${d.HOLD_ON ? chip("Hold", "warn", "hold") : ""}</div>
      </div>
      <div class="quick">
        <button class="btn" data-act="dec" title="Target down 0.5°">−</button>
        <button class="btn" data-act="inc" title="Target up 0.5°">+</button>
        <button class="btn" data-act="standby" title="${d.STANDBY ? "Take out of standby" : "Put in standby"}">${d.STANDBY ? ICON.power : ICON.leaf}</button>
      </div>
    </div>`).join("");
}
$("#zonegrid").addEventListener("click", async (e) => {
  const card = e.target.closest(".zone");
  if (!card) return;
  const name = card.dataset.zone;
  const btn = e.target.closest("button[data-act]");
  if (!btn) { selectedZone = name; switchTab("zone"); return; }
  const d = devByName(name);
  const cur = parseFloat(d.SET_TEMP);
  if (btn.dataset.act === "inc") return action("setTemp", { zones: [name], temp: Math.min(35, cur + 0.5) }, `${name}: set to ${(cur + 0.5).toFixed(1)}°`);
  if (btn.dataset.act === "dec") return action("setTemp", { zones: [name], temp: Math.max(5, cur - 0.5) }, `${name}: set to ${(cur - 0.5).toFixed(1)}°`);
  if (btn.dataset.act === "standby") return action("standby", { zones: [name], on: !d.STANDBY }, `${name}: standby ${d.STANDBY ? "off" : "on"}`);
});
$$("button[data-bulk]").forEach((b) => b.addEventListener("click", () => {
  const all = devices().map((d) => d.ZONE_NAME);
  const k = b.dataset.bulk;
  if (k === "standbyOff") return confirmThen("Take ALL zones out of standby so they follow their schedules?", () => action("standby", { zones: all, on: false }, "All zones out of standby"));
  if (k === "standbyOn") return confirmThen("Put ALL zones into standby (frost protection only)?", () => action("standby", { zones: all, on: true }, "All zones in standby"));
  if (k === "cancelHold") return confirmThen("Cancel holds on all zones?", () => action("cancelHold", { zones: all }, "Holds cancelled"));
}));

// ---------- temperature chart (one series, last 96 readings at 15-minute spacing) ----------
// Samples of 0 (or anything non-positive / non-numeric) mean "no reading yet", e.g. after a stat
// re-joins the hub its 24 h buffer is zero-filled. Those are left as gaps rather than drawn.
const sampleOk = (n) => Number.isFinite(n) && n > 0 && n < 100;
function chart(arr, statTime, width) {
  const v = (arr || []).map((s) => { const n = Number(s); return sampleOk(n) ? n : null; });
  const valid = v.filter((n) => n !== null);
  if (valid.length < 2) return '<div class="muted">Not enough readings yet. The thermostat fills its 24-hour buffer one sample every 15 minutes.</div>';
  const W = Math.max(320, Math.round(width || 640)), H = 150, L = 34, R = 10, T = 10, B = 24;
  let min = Math.floor(Math.min(...valid) - 1), max = Math.ceil(Math.max(...valid) + 1);
  if (max - min < 4) { min -= 1; max += 1; }
  const x = (i) => L + (i / (v.length - 1)) * (W - L - R);
  const y = (n) => T + (1 - (n - min) / (max - min)) * (H - T - B);
  // One sub-path per run of valid samples; the area fill follows the same runs.
  let path = "", area = "", pen = false, runStart = 0;
  v.forEach((n, i) => {
    if (n === null) { if (pen) { area += `L${x(i - 1).toFixed(1)} ${(H - B).toFixed(1)}L${x(runStart).toFixed(1)} ${(H - B).toFixed(1)}Z`; } pen = false; return; }
    const p = `${x(i).toFixed(1)} ${y(n).toFixed(1)}`;
    if (!pen) { runStart = i; path += "M" + p; area += `M${x(i).toFixed(1)} ${(H - B).toFixed(1)}L` + p; pen = true; }
    else { path += "L" + p; area += "L" + p; }
  });
  if (pen) area += `L${x(v.length - 1).toFixed(1)} ${(H - B).toFixed(1)}L${x(runStart).toFixed(1)} ${(H - B).toFixed(1)}Z`;
  const ticks = [min, (min + max) / 2, max];
  // x labels: latest sample is "now"; label every 4 hours going back.
  const now = new Date();
  if (statTime && /^\d{1,2}:\d{2}$/.test(statTime)) { const [hh, mm] = statTime.split(":").map(Number); now.setHours(hh, mm, 0, 0); }
  const xl = [];
  for (let i = 0; i < v.length; i += 16) { const t = new Date(now.getTime() - (v.length - 1 - i) * 15 * 60000); xl.push({ i, t: `${String(t.getHours()).padStart(2, "0")}:${String(t.getMinutes()).padStart(2, "0")}` }); }
  return `<svg class="chart" viewBox="0 0 ${W} ${H}" preserveAspectRatio="none" data-vals="${v.map((n) => (n === null ? "" : n)).join(",")}" data-min="${min}" data-max="${max}" data-w="${W}">
    <defs><linearGradient id="areafill" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#f3b562" stop-opacity=".28"/><stop offset="1" stop-color="#f3b562" stop-opacity="0"/></linearGradient></defs>
    ${ticks.map((t) => `<line class="gridl" x1="${L}" x2="${W - R}" y1="${y(t).toFixed(1)}" y2="${y(t).toFixed(1)}"/><text x="${L - 6}" y="${(y(t) + 4).toFixed(1)}" text-anchor="end">${Math.round(t)}°</text>`).join("")}
    ${xl.map((o) => `<text x="${x(o.i).toFixed(1)}" y="${H - 6}" text-anchor="${o.i === 0 ? "start" : "middle"}">${o.t}</text>`).join("")}
    <path class="area" d="${area}"/><path class="line" d="${path}"/>
    <line class="guide" x1="0" x2="0" y1="${T}" y2="${H - B}" style="display:none"/><circle class="dot" r="4" style="display:none"/>
    <rect class="hit" x="${L}" y="0" width="${W - L - R}" height="${H}"/></svg>`;
}
function wireChart(wrap) {
  const svg = $("svg.chart", wrap), tip = $(".tip", wrap);
  if (!svg || !svg.dataset.vals) return;
  const v = svg.dataset.vals.split(",").map((s) => (s === "" ? null : Number(s))), min = +svg.dataset.min, max = +svg.dataset.max;
  const W = +svg.dataset.w || 640, H = 150, L = 34, R = 10, T = 10, B = 24;
  const guide = $(".guide", svg), dot = $(".dot", svg);
  svg.addEventListener("mousemove", (e) => {
    const r = svg.getBoundingClientRect();
    const fx = ((e.clientX - r.left) / r.width) * W;
    const i = Math.round(Math.min(1, Math.max(0, (fx - L) / (W - L - R))) * (v.length - 1));
    const px = L + (i / (v.length - 1)) * (W - L - R);
    guide.setAttribute("x1", px); guide.setAttribute("x2", px); guide.style.display = "";
    const agoMin = (v.length - 1 - i) * 15;
    const when = agoMin ? Math.floor(agoMin / 60) + "h" + (agoMin % 60 ? (agoMin % 60) + "m" : "") + " ago" : "latest";
    if (v[i] === null) { dot.style.display = "none"; tip.textContent = `no reading · ${when}`; tip.style.left = (px / W) * r.width + "px"; tip.style.top = (T / H) * r.height + "px"; tip.style.display = "block"; return; }
    const py = T + (1 - (v[i] - min) / (max - min)) * (H - T - B);
    dot.setAttribute("cx", px); dot.setAttribute("cy", py); dot.style.display = "";
    tip.textContent = `${v[i].toFixed(1)}° · ${when}`;
    tip.style.left = (px / W) * r.width + "px"; tip.style.top = (py / H) * r.height + "px"; tip.style.display = "block";
  });
  svg.addEventListener("mouseleave", () => { guide.style.display = "none"; dot.style.display = "none"; tip.style.display = "none"; });
}

// ---------- zone tab ----------
// Horizontal picker strip (zones on the Zone and Schedules tabs, profile names on Profiles).
// Rebuilt only when the list of names changes, so a status refresh keeps the scroll position.
const CHEV_L = '<svg viewBox="0 0 24 24"><path d="M15 5l-7 7 7 7"/></svg>', CHEV_R = '<svg viewBox="0 0 24 24"><path d="M9 5l7 7-7 7"/></svg>';
function buildStrip(wrap, names, onPick) {
  const key = names.join("|");
  if (wrap.dataset.key === key) return;
  wrap.dataset.key = key;
  wrap.innerHTML = `<button class="zarrow left" aria-label="Scroll left">${CHEV_L}</button><div class="zstrip" role="tablist">${names.map((n) => `<button role="tab" data-z="${esc(n)}">${esc(n)}</button>`).join("")}</div><button class="zarrow right" aria-label="Scroll right">${CHEV_R}</button>`;
  const strip = $(".zstrip", wrap);
  const update = () => { wrap.classList.toggle("can-left", strip.scrollLeft > 2); wrap.classList.toggle("can-right", strip.scrollLeft + strip.clientWidth < strip.scrollWidth - 2); };
  strip.addEventListener("scroll", update, { passive: true });
  new ResizeObserver(update).observe(strip);
  $(".zarrow.left", wrap).onclick = () => strip.scrollBy({ left: -strip.clientWidth * 0.7, behavior: "smooth" });
  $(".zarrow.right", wrap).onclick = () => strip.scrollBy({ left: strip.clientWidth * 0.7, behavior: "smooth" });
  strip.addEventListener("click", (e) => { const b = e.target.closest("button[data-z]"); if (b) onPick(b.dataset.z); });
  update();
}
// Underline `selected` in a strip and scroll it into view.
function syncStrip(wrap, selected) {
  const strip = $(".zstrip", wrap); if (!strip) return;
  let act = null;
  $$("button[data-z]", strip).forEach((b) => { const on = b.dataset.z === selected; b.classList.toggle("active", on); b.setAttribute("aria-selected", on); if (on) act = b; });
  if (act) {
    const left = act.offsetLeft - strip.offsetLeft, right = left + act.offsetWidth;
    if (left < strip.scrollLeft + 48) strip.scrollTo({ left: Math.max(0, left - 48), behavior: "smooth" });
    else if (right > strip.scrollLeft + strip.clientWidth - 48) strip.scrollTo({ left: right - strip.clientWidth + 48, behavior: "smooth" });
  }
}
const ZONE_STRIPS = { "#zonestrip": () => renderZoneDetail(), "#schedstrip": () => loadSchedule() };
function renderZoneSelects() {
  const names = devices().map((d) => d.ZONE_NAME);
  if (!selectedZone || !names.includes(selectedZone)) selectedZone = names[0];
  for (const [id, onPick] of Object.entries(ZONE_STRIPS)) buildStrip($(id), names, (z) => { selectedZone = z; syncStrips(); onPick(); });
  syncStrips();
}
function syncStrips() { for (const id of Object.keys(ZONE_STRIPS)) syncStrip($(id), selectedZone); }
$("#backzones").addEventListener("click", () => switchTab("overview"));

function renderZoneDetail() {
  syncStrips();
  const d = devByName(selectedZone), e = engByName(selectedZone), sn = status.serials?.[selectedZone];
  if (!d) return;
  const n = selectedZone;
  $("#zonedetail").innerHTML = `
  <div class="cols">
    <div class="zone-left">
      <div class="zone-head"><span class="leaf">${d.HEAT_ON ? ICON.flame : ICON.leaf}</span><h2 class="serif">${esc(n)}</h2><span class="id">#${d.DEVICE_ID}</span></div>
      <div class="bigtemp"><span class="cur">${esc(d.ACTUAL_TEMP)}°</span><span class="target">Target ${esc(d.SET_TEMP)}°</span><span class="chips">${zoneChips(d)}</span></div>
      <div class="chartbox"><div class="tip"></div></div>
      <div class="controls">
        <div class="field"><span class="lbl">Set temperature</span>
          <span class="stepper"><button class="btn step" id="z-tdec">−</button><input type="number" id="z-temp" step="0.5" min="5" max="35" value="${esc(d.SET_TEMP)}"><button class="btn step" id="z-tinc">+</button></span><span class="unit">°C</span>
          <button class="btn primary" id="z-settemp">Set</button></div>
        <div class="field"><span class="lbl">Hold at</span>
          <input type="number" id="z-holdtemp" step="0.5" min="5" max="35" value="${d.HOLD_TEMP || 21}"><span class="unit">°C</span>
          <input type="number" id="z-holdh" min="0" max="99" value="1" style="width:64px"><span class="unit">h</span>
          <input type="number" id="z-holdm" min="0" max="59" value="0" style="width:64px"><span class="unit">m</span>
          <button class="btn primary" id="z-hold">${ICON.hold}Hold</button>${d.HOLD_ON ? '<button class="btn" id="z-cancelhold">End hold</button>' : ""}</div>
        <div class="field"><span class="lbl">Standby (frost only)</span><button class="btn" id="z-standby">${d.STANDBY ? ICON.power + "Take out of standby" : ICON.leaf + "Put in standby"}</button></div>
        <div class="field"><span class="lbl">Keypad lock</span>${d.LOCK ? '<button class="btn" id="z-unlock">Unlock</button>' : '<input id="z-pin" placeholder="4-digit PIN" maxlength="4" style="width:120px"><button class="btn" id="z-lock">' + ICON.lock + "Lock</button>"}</div>
        <div class="field"><span class="lbl">Rename zone</span><input id="z-newname" value="${esc(n)}" style="width:220px"><button class="btn" id="z-rename">${ICON.pencil}Rename</button></div>
        <div class="field"><span class="lbl">Stored profile</span>
          <select id="z-prof" style="min-width:220px"><option value="0">None (own comfort levels)</option>${Object.values(status.profiles || {}).map((p) => `<option value="${p.PROFILE_ID}" ${p.PROFILE_ID === d.ACTIVE_PROFILE ? "selected" : ""}>${esc(p.name)}</option>`).join("")}</select>
          <button class="btn primary" id="z-runprof">Apply</button></div>
      </div>
    </div>
    <div>
      <div class="panel">
        <h3><span class="ico">${ICON.gear}</span>Engineer settings</h3>
        <table class="kv"><tbody>
          ${engRow("Frost temperature", "frost", e.FROST_TEMP, 5, 17, 1, "°C")}
          ${engRow("Switching differential", "diff", e["SWITCHING DIFFERENTIAL"], 0, 3, 1, "°C")}
          ${engRow("Output delay", "delay", e.OUTPUT_DELAY, 0, 15, 1, "min")}
          ${engRow("Max preheat", "preheat", e.MAX_PREHEAT, 0, 5, 1, "h")}
          ${engRow("Floor limit", "floor", e.FLOOR_LIMIT, 20, 40, 1, "°C")}
          ${engRow("User limit (0 = off)", "userlimit", e.USER_LIMIT, 0, 35, 1, "°C")}
        </tbody></table>
      </div>
      <div class="panel">
        <h3><span class="ico">${ICON.monitor}</span>Device information</h3>
        <table class="kv"><tbody>
          <tr><td>Device type</td><td>${esc(e.DEVICE_TYPE)} (stat version ${esc(e.STAT_VERSION)})</td></tr>
          <tr><td>Serial</td><td class="mono">${esc(sn ? sn[1] : "?")}</td></tr>
          <tr><td>Sensor</td><td>${esc(e.SENSOR_MODE)} / RF ${esc(e.RF_SENSOR_MODE)}</td></tr>
          <tr><td>Mode</td><td>${esc(d.HC_MODE)} · ${d.THERMOSTAT ? "thermostat" : "timer"} · ${esc(d.FAN_CONTROL)}</td></tr>
          <tr><td>Active level</td><td>${esc(d.ACTIVE_LEVEL)} (${LEVELS[d.ACTIVE_LEVEL - 1] || "?"})</td></tr>
          <tr><td>Stat clock</td><td style="text-transform:capitalize">${esc(d.DATE)} ${esc(d.TIME)}</td></tr>
          <tr><td>Deadband</td><td>${esc(e.DEADBAND)}</td></tr>
          <tr><td>Window switch</td><td>${esc(e.WINDOW_SWITCH_OPEN)}</td></tr>
          <tr><td>Engineer data saved</td><td>${e.TIMESTAMP ? new Date(e.TIMESTAMP * 1000).toLocaleString() : "?"}</td></tr>
          <tr><td>PIN</td><td>${esc(d.PIN_NUMBER)}</td></tr>
        </tbody></table>
      </div>
    </div>
  </div>`;
  // The chart is drawn at the box's real pixel width so text is never stretched.
  const box = $(".chartbox", $("#zonedetail"));
  box.insertAdjacentHTML("afterbegin", chart(d.RECENT_TEMPS, d.TIME, box.clientWidth - 28));
  wireChart(box);
  const z = [n];
  const tempIn = $("#z-temp");
  $("#z-tdec").onclick = () => (tempIn.value = (Math.max(5, parseFloat(tempIn.value) - 0.5)).toFixed(1));
  $("#z-tinc").onclick = () => (tempIn.value = (Math.min(35, parseFloat(tempIn.value) + 0.5)).toFixed(1));
  $("#z-settemp").onclick = () => action("setTemp", { zones: z, temp: tempIn.value }, `${n}: set to ${tempIn.value}°`);
  $("#z-hold").onclick = () => action("hold", { zones: z, temp: $("#z-holdtemp").value, hours: $("#z-holdh").value, minutes: $("#z-holdm").value }, `${n}: holding ${$("#z-holdtemp").value}°`);
  $("#z-cancelhold") && ($("#z-cancelhold").onclick = () => action("cancelHold", { zones: z }, `${n}: hold cancelled`));
  $("#z-standby").onclick = () => action("standby", { zones: z, on: !d.STANDBY }, `${n}: standby ${d.STANDBY ? "off" : "on"}`);
  $("#z-lock") && ($("#z-lock").onclick = () => action("lock", { zones: z, pin: $("#z-pin").value }, `${n}: locked`));
  $("#z-unlock") && ($("#z-unlock").onclick = () => action("unlock", { zones: z }, `${n}: unlocked`));
  $("#z-rename").onclick = () => { const to = $("#z-newname").value.trim(); if (to && to !== n) confirmThen(`Rename "${n}" to "${to}"?`, async () => { await action("renameZone", { from: n, to }, "Renamed"); selectedZone = to; }); };
  $("#z-runprof").onclick = () => { const id = Number($("#z-prof").value); return id ? action("runProfile", { profileId: id, zones: z }, `${n}: profile applied`) : action("clearProfile", { zones: z }, `${n}: now using its own comfort levels`); };
  const engActions = { frost: ["setFrost", "temp"], diff: ["setDiff", "value"], delay: ["setDelay", "value"], preheat: ["setPreheat", "value"], floor: ["setFloor", "value"], userlimit: ["setUserLimit", "value"] };
  for (const [k, [act, param]] of Object.entries(engActions)) {
    const b = $(`#eng-${k}-btn`); if (b) b.onclick = () => action(act, { zones: z, [param]: $(`#eng-${k}`).value }, `${n}: saved`);
  }
}
const engRow = (label, key, val, min, max, step, unit) => `<tr><td class="ctl">${label}</td><td class="ctl"><input type="number" id="eng-${key}" value="${val ?? ""}" min="${min}" max="${max}" step="${step}"> <span class="unit">${unit}</span><button class="btn small" id="eng-${key}-btn">Save</button></td></tr>`;

// ---------- schedules (comfort levels) ----------
$("#schedload").addEventListener("click", loadSchedule);
async function loadSchedule() {
  syncStrips();
  const name = selectedZone;
  $("#scheddetail").innerHTML = '<div class="panel muted">Loading…</div>';
  try {
    const z = await api("/api/zone?name=" + encodeURIComponent(name));
    const levels = z.comfort?.[name];
    if (!levels) throw new Error("No comfort levels returned");
    const d = devByName(name);
    $("#scheddetail").innerHTML = `<div class="panel">
      <h3><span class="ico">${ICON.cal}</span>${esc(name)} comfort levels ${d?.ACTIVE_PROFILE ? chip("Currently following profile: " + (profileById(d.ACTIVE_PROFILE)?.name || d.ACTIVE_PROFILE), "warn") : ""}</h3>
      <p class="muted">Schedule format: <b>${esc(formatName(status.system.FORMAT))}</b>. Times are 24-hour, temperatures in °C. Leave a time blank to disable that level. Saving takes the zone off any stored profile.</p>
      ${scheduleEditor("sched", levels)}
      <div class="row gap wrap" style="margin-top:12px"><button class="btn primary" id="schedsave">Save to ${esc(name)}</button>
      <button class="btn" id="schedcopy">${ICON.hold}Copy to other zones…</button></div>
      <details style="margin-top:12px"><summary class="muted">Timer (hot water / timeclock) programme for this zone</summary><pre class="mono out">${esc(JSON.stringify(z.timer0?.profiles?.[0] || z.timer0, null, 1))}</pre></details>
    </div>`;
    $("#schedsave").onclick = () => action("setComfortLevels", { zone: name, levels: readEditor("sched", levels) }, `${name}: comfort levels saved`);
    $("#schedcopy").onclick = async () => {
      const others = devices().map((x) => x.ZONE_NAME).filter((x) => x !== name);
      const pick = prompt("Copy this schedule to which zones? Comma-separated, or * for all:\n" + others.join(", "));
      if (!pick) return;
      const targets = pick.trim() === "*" ? others : pick.split(",").map((s) => s.trim()).filter((s) => others.includes(s));
      if (!targets.length) return toast("No matching zones", true);
      const lv = readEditor("sched", levels);
      for (const t of targets) await action("setComfortLevels", { zone: t, levels: lv }, `${t}: comfort levels saved`);
    };
  } catch (e) { $("#scheddetail").innerHTML = `<div class="panel"><span class="chip bad">${esc(e.message)}</span></div>`; }
}
function formatName(f) { return { 0: "Non-programmable", 1: "5 day / 2 day", 2: "7 day", 4: "24 hours fixed" }[f] ?? f; }
function daysForEditor(levels) {
  const keys = Object.keys(levels || {});
  if (keys.length === 7) return DAYS7;
  if (keys.length) return keys.sort((a, b) => DAYS7.indexOf(a) - DAYS7.indexOf(b));
  return status.system.FORMAT === 2 ? DAYS7 : ["monday", "sunday"];
}
// Period / Wake / Leave / Return / Sleep table. Editable by default; `readOnly` renders the
// same layout with pills instead of inputs (used by the Profiles tab).
function scheduleEditor(prefix, levels, readOnly = false) {
  const days = daysForEditor(levels);
  const dayLabel = (day) => day === "monday" && days.length === 2 ? "Weekdays (Mon–Fri)" : day === "sunday" && days.length === 2 ? "Weekend (Sat–Sun)" : day;
  const cell = (l, v) => readOnly
    ? `<div class="lv"><span class="ro">${v[0] === "24:00" ? "—" : esc(v[0])}</span><span class="tv"><span class="ro">${esc(v[1])}</span><span class="unit">°C</span></span></div>`
    : `<div class="lv"><input type="time" data-l="${l}" data-k="t" value="${esc(v[0] === "24:00" ? "" : v[0])}"><span class="tv"><input type="number" data-l="${l}" data-k="v" step="0.5" min="5" max="35" value="${esc(v[1])}"><span class="unit">°C</span></span></div>`;
  return `<div class="tablewrap"><table class="sched" id="${prefix}-table"><thead><tr><th>Period</th>${LEVELS.map((l) => `<th style="text-transform:capitalize">${l}</th>`).join("")}</tr></thead><tbody>
  ${days.map((day) => `<tr data-day="${day}"><td>${dayLabel(day)}</td>${LEVELS.map((l) => `<td>${cell(l, levels?.[day]?.[l] || ["24:00", 16])}</td>`).join("")}</tr>`).join("")}</tbody></table></div>`;
}
// Reads the editor back into the hub's shape. Extra array items the hub sent (e.g. cool temp,
// flags) are preserved from `orig` so a round trip does not strip them.
function readEditor(prefix, orig) {
  const out = {};
  $$(`#${prefix}-table tbody tr`).forEach((tr) => {
    const day = tr.dataset.day; out[day] = {};
    for (const l of LEVELS) {
      const t = $(`input[data-l="${l}"][data-k="t"]`, tr).value || "24:00";
      const v = Number($(`input[data-l="${l}"][data-k="v"]`, tr).value);
      const extra = Array.isArray(orig?.[day]?.[l]) ? orig[day][l].slice(2) : [];
      out[day][l] = [t, v, ...extra];
    }
  });
  return out;
}

// ---------- profiles ----------
let selectedProfile = null; // profile name shown on the Profiles tab
function renderProfiles() {
  const profs = Object.values(status.profiles || {}).sort((a, b) => a.PROFILE_ID - b.PROFILE_ID);
  const names = profs.map((p) => p.name);
  if (!selectedProfile || !names.includes(selectedProfile)) selectedProfile = names[0] || null;
  buildStrip($("#profstrip"), names, (n) => { selectedProfile = n; renderProfiles(); });
  syncStrip($("#profstrip"), selectedProfile);
  const p = profs.find((x) => x.name === selectedProfile);
  const users = p ? devices().filter((d) => d.ACTIVE_PROFILE === p.PROFILE_ID).map((d) => d.ZONE_NAME) : [];
  $("#profiles").innerHTML = p ? `<div class="profile" data-pid="${p.PROFILE_ID}">
    <div class="body">
      <h3><span class="ico">${ICON.cal}</span>${esc(p.name)} <span class="id">#${p.PROFILE_ID}</span></h3>
      <div class="row gap wrap" style="margin:4px 0 10px"><span class="muted">Used by</span><span class="chips">${users.length ? users.map((u) => chip(u, "warn", "home")).join("") : chip("No zones")}</span></div>
      ${scheduleEditor("prof-" + p.PROFILE_ID, p.info, true)}
      <div class="row gap wrap" style="margin-top:10px">
        <button class="btn" data-pact="apply">Apply to zones…</button>
        <button class="btn" data-pact="rename">Rename</button>
        <button class="btn danger" data-pact="delete">Delete</button>
      </div>
    </div></div>` : '<div class="panel muted">No stored profiles on the hub.</div>';
  if (!$("#newprofeditor").innerHTML) $("#newprofeditor").innerHTML = scheduleEditor("newprof", null);
}
$("#profiles").addEventListener("click", async (e) => {
  const b = e.target.closest("button[data-pact]"); if (!b) return;
  const card = b.closest("[data-pid]"), id = Number(card.dataset.pid), p = profileById(id);
  if (b.dataset.pact === "apply") {
    const names = devices().map((d) => d.ZONE_NAME);
    const pick = prompt(`Apply "${p.name}" to which zones? Comma-separated, or * for all:\n` + names.join(", "));
    if (!pick) return;
    const targets = pick.trim() === "*" ? names : pick.split(",").map((s) => s.trim()).filter((s) => names.includes(s));
    if (!targets.length) return toast("No matching zones", true);
    return action("runProfile", { profileId: id, zones: targets }, `Profile applied to ${targets.length} zone(s)`);
  }
  if (b.dataset.pact === "rename") { const to = prompt("New name for profile", p.name); if (to && to !== p.name) { selectedProfile = to; return action("renameProfile", { from: p.name, to }, "Profile renamed"); } }
  if (b.dataset.pact === "delete") return confirmThen(`Delete stored profile "${p.name}"? Zones using it will keep their last-transferred schedule.`, () => action("deleteProfile", { name: p.name }, "Profile deleted"));
});
$("#storeprof").addEventListener("click", () => {
  const name = $("#newprofname").value.trim(); if (!name) return toast("Give the profile a name", true);
  return action("storeProfile", { name, info: readEditor("newprof") }, `Profile "${name}" stored`);
});

// ---------- hub tab ----------
function renderHub() {
  const s = status.system, l = status.live, h = status.holiday;
  const hubLocal = new Date((l.HUB_TIME || 0) * 1000);
  const pad = (n) => String(n).padStart(2, "0");
  const hubClock = `${pad(hubLocal.getUTCHours())}:${pad(hubLocal.getUTCMinutes())}:${pad(hubLocal.getUTCSeconds())}`;
  const hubDate = hubLocal.toLocaleDateString(undefined, { day: "numeric", month: "short", year: "numeric", timeZone: "UTC" });
  const fmt = (ts) => (ts ? new Date(ts * 1000).toLocaleString() : "—");
  $("#hubdetail").innerHTML = `<div class="cols3">
    <div class="panel"><h3><span class="ico">${ICON.clock}</span>Time</h3>
      <table class="kv"><tbody>
        <tr><td>Hub local time</td><td><b>${hubClock}</b><br><span class="muted">${hubDate}</span></td></tr>
        <tr><td>This device</td><td>${new Date().toLocaleString()}</td></tr>
        <tr><td>NTP client</td><td>${s.NTP_ON === "Running" ? chip("Running", "good") : chip(esc(s.NTP_ON), "warn")}</td></tr>
        <tr><td>Time zone offset</td><td>${esc(s.TIME_ZONE)} h</td></tr>
        <tr><td>DST</td><td>${s.DST_AUTO ? "Auto" : "Manual"} (currently ${s.DST_ON ? "on" : "off"})</td></tr>
      </tbody></table>
      <div class="field"><span class="lbl">NTP</span><button class="btn" id="h-ntp-on">Start</button><button class="btn" id="h-ntp-off">Stop</button></div>
      <div class="field"><span class="lbl">Set hub clock</span><button class="btn" id="h-sync">${ICON.clock}Sync from this device's clock</button></div>
      <div class="field"><span class="lbl">Time zone (hours from UTC)</span><input type="number" id="h-tz" step="0.5" min="-12" max="14" value="${esc(s.TIME_ZONE)}"><button class="btn" id="h-tz-btn">Save</button></div>
      <div class="field"><span class="lbl">DST</span><select id="h-dstregion"><option>UK</option><option>EU</option><option>NZ</option></select><button class="btn" id="h-dst-on">Auto on</button><button class="btn" id="h-dst-off">Off</button></div>
    </div>
    <div class="panel"><h3><span class="ico">${ICON.leaf}</span>Modes</h3>
      <div class="field"><span class="lbl">Away</span>${chip(l.HUB_AWAY ? "On" : "Off", l.HUB_AWAY ? "warn" : "")}<button class="btn" id="h-away">${l.HUB_AWAY ? "Turn away off" : "Turn away on"}</button></div>
      <div class="field"><span class="lbl">Holiday</span>${h?.start ? chip(`${fmt(h.start)} → ${fmt(h.end)}`, "warn") : chip("None")}${h?.start ? '<button class="btn" id="h-hol-cancel">Cancel holiday</button>' : ""}</div>
      <div class="field top"><span class="lbl">Set holiday</span>
        <div class="holiday"><span class="unit">From</span><input type="datetime-local" id="h-hol-start"><span class="unit">To</span><input type="datetime-local" id="h-hol-end"><button class="btn" id="h-hol-set">Set holiday</button></div></div>
      <div class="field"><span class="lbl">Schedule format</span><select id="h-format">${["NONPROGRAMMABLE", "24HOURSFIXED", "5DAY/2DAY", "7DAY"].map((f) => `<option ${formatCode(f) === s.FORMAT ? "selected" : ""}>${f}</option>`).join("")}</select><button class="btn" id="h-format-btn">Save</button></div>
      <div class="field"><span class="lbl">Temperature unit</span><select id="h-unit"><option ${s.CORF === "C" ? "selected" : ""}>°C</option><option ${s.CORF === "F" ? "selected" : ""}>°F</option></select><button class="btn" id="h-unit-btn">Save</button></div>
      <div class="field"><span class="lbl">Find the hub</span><button class="btn" id="h-identify">${ICON.flame}Flash hub LED</button></div>
    </div>
    <div class="panel"><h3><span class="ico">${ICON.monitor}</span>Hub information</h3>
      <table class="kv"><tbody>
        <tr><td>Hub address</td><td id="h-hubaddr"><span class="muted">…</span></td></tr>
        <tr><td>Firmware</td><td>${esc(status.firmware["firmware version"])} (hub type ${esc(s.HUB_TYPE)})</td></tr>
        <tr><td>System type</td><td>${esc(s.GLOBAL_SYSTEM_TYPE)} · ${esc(s.GLOBAL_HC_MODE)}</td></tr>
        <tr><td>Heating levels</td><td>${esc(s.HEATING_LEVELS)}</td></tr>
        <tr><td class="ctl">ZigBee channel</td><td class="ctl"><select id="h-chan" style="padding:4px 8px">${Array.from({ length: 16 }, (_, i) => i + 11).map((c) => `<option ${String(c) === String(s.ZIGBEE_CHANNEL) ? "selected" : ""}>${c}</option>`).join("")}</select><button class="btn small" id="h-chan-btn">Change</button></td></tr>
        <tr><td>Legacy local port</td><td>${esc(s.LEGACY_LOCAL_PORT)}</td></tr>
        <tr><td>Extended history</td><td>${esc(s.EXTENDED_HISTORY)}</td></tr>
        <tr><td>Device list changed</td><td>${fmt(l.TIMESTAMP_DEVICE_LISTS)}</td></tr>
        <tr><td>Engineer data changed</td><td>${fmt(l.TIMESTAMP_ENGINEERS)}</td></tr>
        <tr><td>Profiles changed</td><td>${fmt(l.TIMESTAMP_PROFILE_0)}</td></tr>
        <tr><td>System settings changed</td><td>${fmt(l.TIMESTAMP_SYSTEM)}</td></tr>
        <tr><td>Zones</td><td><div class="val-list">${Object.entries(status.zones || {}).sort((a, b) => a[1] - b[1]).map(([n, id]) => `<span>${esc(n)} <span class="muted">(${id})</span></span>`).join("")}</div></td></tr>
      </tbody></table>
      <details style="margin-top:8px"><summary class="muted">Raw GET_SYSTEM</summary><pre class="mono out">${esc(JSON.stringify(s, null, 1))}</pre></details>
    </div>
  </div>`;
  // Hub address + discovery state (the tray app finds the hub itself; the Node server just reports its config).
  api("/api/hub").then((h) => {
    const cell = $("#h-hubaddr"); if (!cell) return;
    cell.innerHTML = `${h.host ? `<span class="mono">${esc(h.host)}:${esc(h.port)}</span>` : chip(h.searching ? "Searching the network…" : "Not found", "warn")}${h.lastFound ? ` <span class="muted">· found ${new Date(h.lastFound).toLocaleTimeString()}</span>` : ""}${h.lastError && !h.host ? ` <span class="muted">· ${esc(h.lastError)}</span>` : ""} <button class="btn small" id="h-rediscover">Find hub again</button>`;
    $("#h-rediscover").onclick = () => action("rediscover", {}, "Searching the network for the hub…");
  }).catch(() => { const cell = $("#h-hubaddr"); if (cell) cell.innerHTML = `<span class="mono">${esc(status.hubHost || "configured in config.json")}</span>`; });
  $("#h-ntp-on").onclick = () => action("ntp", { on: true });
  $("#h-ntp-off").onclick = () => action("ntp", { on: false });
  $("#h-sync").onclick = () => action("syncTimeFromPc", {}, "Hub clock set");
  $("#h-tz-btn").onclick = () => action("timezone", { tz: $("#h-tz").value });
  $("#h-dst-on").onclick = () => action("dst", { on: true, region: $("#h-dstregion").value });
  $("#h-dst-off").onclick = () => action("dst", { on: false });
  $("#h-away").onclick = () => action("away", { on: !l.HUB_AWAY });
  $("#h-hol-cancel") && ($("#h-hol-cancel").onclick = () => action("cancelHoliday"));
  $("#h-hol-set").onclick = () => { const a = $("#h-hol-start").value, b = $("#h-hol-end").value; if (!a || !b) return toast("Pick both dates", true); return action("holiday", { start: a, end: b }, "Holiday set"); };
  $("#h-format-btn").onclick = () => confirmThen("Changing the schedule format rewrites every zone's programme. Continue?", () => action("setFormat", { format: $("#h-format").value }));
  $("#h-unit-btn").onclick = () => action("setTempFormat", { unit: $("#h-unit").value.replace("°", "") });
  $("#h-identify").onclick = () => action("identify", {}, "Hub LED flashing");
  $("#h-chan-btn").onclick = () => {
    const c = $("#h-chan").value;
    if (String(c) === String(s.ZIGBEE_CHANNEL)) return toast("Already on channel " + c);
    return confirmThen(`Move the ZigBee network from channel ${s.ZIGBEE_CHANNEL} to ${c}?\n\nEvery thermostat will drop off and re-join, which can take several minutes. Occasionally a stat needs re-pairing afterwards.`, () => action("setChannel", { channel: c }, `Changing to channel ${c}, stats are re-joining…`));
  };
}
function formatCode(f) { return { NONPROGRAMMABLE: 0, "5DAY/2DAY": 1, "7DAY": 2, "24HOURSFIXED": 4 }[f]; }

// ---------- connectivity history ----------
// Snapshots come from /api/history. Each one is the full state at that moment; a state holds
// until the next snapshot. A gap longer than ~2 heartbeats means the dashboard was not running.
let histRangeH = 24;
const fmtDur = (ms) => { const m = Math.round(ms / 60000); if (m < 1) return "<1 min"; if (m < 60) return m + " min"; const h = Math.floor(m / 60); if (h < 48) return h + " h " + (m % 60 ? (m % 60) + " min" : ""); return Math.floor(h / 24) + " d " + (h % 24) + " h"; };
const fmtWhen = (t) => new Date(t).toLocaleString(undefined, { weekday: "short", day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" });
$("#ranges").addEventListener("click", (e) => { const b = e.target.closest("button[data-range]"); if (!b) return; $$("#ranges button").forEach((x) => x.classList.toggle("active", x === b)); histRangeH = Number(b.dataset.range); loadHistory(); });
$("#histrefresh").addEventListener("click", loadHistory);

async function loadHistory() {
  const now = Date.now();
  const since = histRangeH ? now - histRangeH * 3600000 : 0;
  let h;
  try { h = await api("/api/history?since=" + since); } catch (e) { $("#timeline").innerHTML = `<span class="chip bad">${esc(e.message)}</span>`; return; }
  const snaps = h.snapshots;
  if (!snaps.length) { $("#timeline").innerHTML = '<p class="muted">No history yet. The dashboard records a snapshot every minute while it runs.</p>'; return; }
  const gapMs = Math.max(h.heartbeatMs * 2, h.pollMs * 5);
  const start = histRangeH ? since : snaps[0].t;
  const end = h.now;
  // Entities: hub, NTP, then every zone ever seen.
  const zoneSet = new Set();
  snaps.forEach((s) => (s.zones || []).forEach((z) => zoneSet.add(z)));
  const zones = Array.from(zoneSet);
  const rows = [{ key: "hub", label: "Hub (192.168.1.46)" }, { key: "ntp", label: "NTP time sync" }, ...zones.map((z) => ({ key: "z:" + z, label: z }))];
  // State of each entity at snapshot s: "ok" | "off" | "warn" | "stopped" | "none"
  const stateAt = (s, key) => {
    if (s.hub === null) return "none";
    if (!s.hub) return key === "hub" ? "off" : "none";
    if (key === "hub") return "ok";
    if (key === "ntp") return s.ntp === "Running" ? "ok" : "stopped";
    const z = key.slice(2);
    if (!(s.zones || []).includes(z)) return "none";
    if ((s.off || []).includes(z)) return "off";
    if ((s.low || []).includes(z)) return "warn";
    return "ok";
  };
  // Build segments per row.
  const segs = {};
  for (const r of rows) {
    const list = [];
    for (let i = 0; i < snaps.length; i++) {
      const s = snaps[i], next = snaps[i + 1];
      let t0 = s.t, t1 = next ? next.t : end;
      let st = stateAt(s, r.key);
      if (t1 - t0 > gapMs) { // monitoring stopped somewhere in here
        list.push({ t0, t1: t0 + Math.min(gapMs / 2, t1 - t0), st });
        list.push({ t0: t0 + Math.min(gapMs / 2, t1 - t0), t1, st: "none" });
      } else list.push({ t0, t1, st });
    }
    // merge consecutive same-state, clip to window
    const merged = [];
    for (const g of list) {
      const a = Math.max(g.t0, start), b = Math.min(g.t1, end);
      if (b <= a) continue;
      const last = merged[merged.length - 1];
      if (last && last.st === g.st && last.t1 >= a) last.t1 = b; else merged.push({ t0: a, t1: b, st: g.st });
    }
    segs[r.key] = merged;
  }
  // Geometry: pixels per hour chosen so short ranges fill the box and long ones scroll.
  const box = $("#timeline");
  const boxW = Math.max(300, box.clientWidth - 140);
  const hours = (end - start) / 3600000;
  const pxPerHour = Math.max(boxW / hours, hours <= 6 ? 120 : hours <= 24 ? 60 : hours <= 72 ? 40 : 24);
  // TOP = time-axis strip (22px) + the "Device" header row (24px) so bars line up with the labels.
  const W = Math.ceil(hours * pxPerHour), RH = 24, TOP = 46, H = TOP + rows.length * RH + 6;
  const x = (t) => ((t - start) / 3600000) * pxPerHour;
  // Ticks: hourly for short ranges, 6-hourly / daily for long ones.
  const stepH = hours <= 12 ? 1 : hours <= 48 ? 3 : hours <= 168 ? 12 : 24;
  const ticks = [];
  const first = new Date(start); first.setMinutes(0, 0, 0); first.setHours(Math.ceil(first.getHours() / stepH) * stepH);
  for (let t = first.getTime(); t <= end; t += stepH * 3600000) {
    const d = new Date(t);
    const major = d.getHours() === 0;
    ticks.push({ x: x(t), major, label: major || stepH >= 24 ? d.toLocaleDateString(undefined, { weekday: "short", day: "numeric", month: "short" }) : `${String(d.getHours()).padStart(2, "0")}:00` });
  }
  const labelFor = { ok: "Online", off: "Offline", warn: "Low battery", stopped: "Stopped", none: "Not monitored" };
  box.innerHTML = `<div class="tl-labels"><div class="head">Device</div>${rows.map((r) => `<div title="${esc(r.label)}">${esc(r.label)}</div>`).join("")}</div>
    <div class="tl-scroll"><svg class="tl" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}">
      ${ticks.map((t) => `<line class="tick ${t.major ? "major" : ""}" x1="${t.x.toFixed(1)}" x2="${t.x.toFixed(1)}" y1="${TOP - 6}" y2="${H}"/><text x="${(t.x + 4).toFixed(1)}" y="12">${t.label}</text>`).join("")}
      ${rows.map((r, i) => `<line class="rowline" x1="0" x2="${W}" y1="${TOP + (i + 1) * RH}" y2="${TOP + (i + 1) * RH}"/>` + segs[r.key].map((g) => `<rect class="seg ${g.st}" x="${x(g.t0).toFixed(1)}" y="${TOP + i * RH + 5}" width="${Math.max(1.5, x(g.t1) - x(g.t0)).toFixed(1)}" height="${RH - 10}" rx="3" data-tip="${esc(r.label)} · ${labelFor[g.st]} · ${fmtWhen(g.t0)} → ${g.t1 >= end - 60000 ? "now" : fmtWhen(g.t1)} (${fmtDur(g.t1 - g.t0)})"/>`).join("")).join("")}
      <line class="now" x1="${x(end).toFixed(1)}" x2="${x(end).toFixed(1)}" y1="${TOP - 6}" y2="${H}"/>
    </svg></div>`;
  let tip = $(".tl-tip"); if (!tip) { tip = document.createElement("div"); tip.className = "tl-tip"; document.body.appendChild(tip); }
  const scroller = $(".tl-scroll", box);
  scroller.addEventListener("mousemove", (e) => { const r = e.target.closest("rect.seg"); if (!r) { tip.style.display = "none"; return; } tip.textContent = r.dataset.tip; tip.style.display = "block"; tip.style.left = Math.min(e.clientX + 12, window.innerWidth - 320) + "px"; tip.style.top = e.clientY + 14 + "px"; });
  scroller.addEventListener("mouseleave", () => (tip.style.display = "none"));
  // drag to pan
  let drag = null;
  scroller.addEventListener("mousedown", (e) => { drag = { x: e.clientX, left: scroller.scrollLeft }; scroller.classList.add("dragging"); });
  window.addEventListener("mousemove", (e) => { if (drag) scroller.scrollLeft = drag.left - (e.clientX - drag.x); });
  window.addEventListener("mouseup", () => { drag = null; scroller.classList.remove("dragging"); });
  scroller.scrollLeft = scroller.scrollWidth; // start at "now"

  // Events list (state transitions) and totals.
  const events = [];
  const totals = {};
  for (const r of rows) {
    const list = segs[r.key];
    const tot = { count: 0, ms: 0, longest: 0 };
    for (let i = 0; i < list.length; i++) {
      const g = list[i], prev = list[i - 1];
      const bad = g.st === "off" || g.st === "stopped" || g.st === "warn";
      if (bad) { tot.count++; tot.ms += g.t1 - g.t0; tot.longest = Math.max(tot.longest, g.t1 - g.t0); }
      if (!prev || prev.st === g.st) continue;
      if (g.st === "none") { if (r.key === "hub") events.push({ t: g.t0, cls: "ev-info", text: "Monitoring gap (dashboard not running)", dur: fmtDur(g.t1 - g.t0) }); continue; }
      else if (g.st === "ok" && prev.st !== "none") events.push({ t: g.t0, cls: "ev-ok", text: `${r.label} back ${r.key === "ntp" ? "running" : "online"}`, dur: fmtDur(prev.t1 - prev.t0) + (prev.st === "warn" ? " on low battery" : " " + labelFor[prev.st].toLowerCase()) });
      else if (bad) events.push({ t: g.t0, cls: g.st === "warn" ? "ev-warn" : "ev-off", text: `${r.label} ${g.st === "warn" ? "low battery" : g.st === "stopped" ? "stopped" : "went offline"}`, dur: g.t1 >= end - 60000 ? "ongoing" : fmtDur(g.t1 - g.t0) });
    }
    totals[r.label] = tot;
  }
  snaps.filter((s) => s.note && s.t >= start).forEach((s) => events.push({ t: s.t, cls: "ev-info", text: s.note[0].toUpperCase() + s.note.slice(1), dur: "" }));
  events.sort((a, b) => b.t - a.t);
  $("#histevents tbody").innerHTML = events.length ? events.slice(0, 300).map((e) => `<tr><td style="white-space:nowrap">${fmtWhen(e.t)}</td><td class="${e.cls}">${esc(e.text)}</td><td>${esc(e.dur)}</td></tr>`).join("") : '<tr><td colspan="3" class="muted">No drop-outs in this range.</td></tr>';
  $("#histtotals tbody").innerHTML = rows.map((r) => { const t = totals[r.label]; return `<tr><td>${esc(r.label)}</td><td>${t.count}</td><td class="${t.ms ? "ev-off" : ""}">${t.ms ? fmtDur(t.ms) : "—"}</td><td>${t.longest ? fmtDur(t.longest) : "—"}</td></tr>`; }).join("");
  const bad = rows.filter((r) => totals[r.label].count).length;
  $("#histsummary").textContent = `· ${snaps.length} snapshots · ${bad ? bad + " device" + (bad > 1 ? "s" : "") + " with drop-outs" : "no drop-outs"} in this range`;
}

// ---------- host ----------
// The machine neoHub runs on (a PC or a Raspberry Pi): refreshed every two seconds while the tab is open.
let hostTimer = null;
const bytes = (n) => { if (n == null) return "—"; const u = ["B", "KB", "MB", "GB", "TB"]; let i = 0; while (n >= 1024 && i < u.length - 1) { n /= 1024; i++; } return `${n.toFixed(n >= 100 || i === 0 ? 0 : 1)} ${u[i]}`; };
const since = (ms) => { if (!ms) return "—"; let s = Math.max(0, Math.round((Date.now() - ms) / 1000)); const d = Math.floor(s / 86400); s -= d * 86400; const h = Math.floor(s / 3600); s -= h * 3600; const m = Math.floor(s / 60); return d ? `${d} day${d === 1 ? "" : "s"}, ${h} h` : h ? `${h} h ${m} min` : `${m} min`; };
async function loadHost() {
  clearTimeout(hostTimer);
  if (!$("#tab-host").classList.contains("active") || document.hidden) return;
  try { renderHost(await api("/api/host")); $("#host-live").textContent = "· updated " + new Date().toLocaleTimeString(); }
  catch (e) { $("#host-gauges").innerHTML = `<p class="muted">${esc(e.message)}</p>`; }
  finally { hostTimer = setTimeout(loadHost, 2000); }
}
document.addEventListener("visibilitychange", () => { if (!document.hidden) loadHost(); });
function renderHost(h) {
  const m = h.machine, live = h.live, app = h.app, net = h.network;
  $("#host-title").textContent = m.name;
  $("#host-sub").textContent = `${m.model || m.name} · ${m.os}`;
  const level = (pct) => (pct == null ? "" : pct >= 90 ? "bad" : pct >= 75 ? "warn" : "ok");
  const gauge = (icon, label, pct, big, sub) => `<div class="gauge">${icon}<div><div class="glabel">${label}</div><div class="gnum">${big}</div>
    ${pct == null ? "" : `<div class="gbar ${level(pct)}"><span style="width:${Math.min(100, Math.max(0, pct)).toFixed(0)}%"></span></div>`}${sub ? `<div class="gsub">${sub}</div>` : ""}</div></div>`;
  const memUsed = live.memoryTotal != null && live.memoryAvailable != null ? live.memoryTotal - live.memoryAvailable : null;
  const memPct = memUsed != null ? (100 * memUsed) / live.memoryTotal : null;
  const diskUsed = live.diskTotal != null && live.diskFree != null ? live.diskTotal - live.diskFree : null;
  const diskPct = diskUsed != null ? (100 * diskUsed) / live.diskTotal : null;
  const t = live.temperatureCelsius;
  const cpuIcon = '<svg viewBox="0 0 24 24"><rect x="5" y="5" width="14" height="14" rx="2"/><rect x="9" y="9" width="6" height="6"/><path d="M9 2v3M15 2v3M9 19v3M15 19v3M2 9h3M2 15h3M19 9h3M19 15h3"/></svg>';
  const memIcon = '<svg viewBox="0 0 24 24"><rect x="3" y="8" width="18" height="9" rx="1"/><path d="M7 17v3M12 17v3M17 17v3M7 8V5M12 8V5M17 8V5"/></svg>';
  const diskIcon = '<svg viewBox="0 0 24 24"><ellipse cx="12" cy="6" rx="8" ry="3"/><path d="M4 6v12c0 1.7 3.6 3 8 3s8-1.3 8-3V6M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3"/></svg>';
  const tempIcon = '<svg viewBox="0 0 24 24"><path d="M10 4a2 2 0 0 1 4 0v9.5a4 4 0 1 1-4 0V4Z"/></svg>';
  $("#host-gauges").innerHTML = [
    gauge(cpuIcon, "Processor", live.cpuPercent, live.cpuPercent == null ? "…" : `${live.cpuPercent.toFixed(0)}<small>%</small>`, `${m.cores} core${m.cores === 1 ? "" : "s"}`),
    t == null ? "" : `<div class="gauge temp ${t >= 80 ? "bad" : t >= 70 ? "warn" : ""}">${tempIcon}<div><div class="glabel">Processor temperature</div><div class="gnum">${t.toFixed(1)}<small>°C</small></div><div class="gsub">${t >= 80 ? "Hot: it will slow itself down" : t >= 70 ? "Warm" : "Fine"}</div></div></div>`,
    gauge(memIcon, "Memory", memPct, memUsed == null ? "—" : bytes(memUsed), memUsed == null ? "" : `used of ${bytes(live.memoryTotal)} · <b>${bytes(live.memoryAvailable)} spare</b>`),
    gauge(diskIcon, "Disk space", diskPct, diskUsed == null ? "—" : bytes(diskUsed), diskUsed == null ? "" : `used of ${bytes(live.diskTotal)} · <b>${bytes(live.diskFree)} free</b>`),
    gauge(ICON.flame, "neoHeat itself", app.cpuPercent, bytes(app.memory), `memory · ${app.cpuPercent == null ? "…" : app.cpuPercent.toFixed(1) + "% processor"}`),
  ].join("");
  const rows = (pairs) => pairs.filter(([, v]) => v != null && v !== "").map(([k, v]) => `<tr><td>${k}</td><td>${v}</td></tr>`).join("");
  $("#host-machine").innerHTML = rows([
    ["Name", esc(m.name)], ["Model", m.model && esc(m.model)], ["Operating system", esc(m.os)], ["Kernel", m.kernel && esc(m.kernel)],
    ["Processor", `${m.processor ? esc(m.processor) + ", " : ""}${m.cores} core${m.cores === 1 ? "" : "s"} (${esc(m.architecture)})`],
    ["Up for", since(m.upSince)], ["Time here", new Date(m.now).toLocaleString()], ["Time zone", esc(m.timeZone)],
  ]);
  $("#host-network").innerHTML = rows([
    ["Open neoHeat at", net.urls.map((u) => `<a href="${esc(u)}">${esc(u)}</a>`).join("<br>")],
    ...net.interfaces.map((n) => [`${esc(n.kind)} <span class="muted">${esc(n.name)}</span>`, n.addresses.map(esc).join("<br>")]),
  ]);
  $("#host-app").innerHTML = rows([
    ["Version", esc(app.version)], ["Running on", esc(app.runtime)], ["Running for", since(app.started)],
    ["Hub", app.hubHost ? `<span class="mono">${esc(app.hubHost)}</span>` : "searching…"],
    ["Data", `<span class="mono">${esc(app.data)}</span> <span class="muted">(${bytes(app.dataSize)})</span>`],
  ]);
}

// ---------- phone link ----------
// The tray / Pi app knows the LAN addresses and draws the QR code; the Node dev server does not.
async function renderPhone() {
  const box = $("#phonedetail");
  try {
    const r = await api("/api/addresses");
    if (!r.primary) throw new Error("No network address found. Is this computer connected to your home network?");
    box.innerHTML = `<div class="cols">
      <div class="panel phone"><h3><span class="ico">${ICON.home}</span>Open on your phone</h3>
        <div class="row gap wrap" style="align-items:flex-start">
          <img class="qr" src="/qr.png?text=${encodeURIComponent(r.primary)}" alt="QR code for ${esc(r.primary)}" width="220" height="220">
          <div style="min-width:0"><p>Point your phone's camera at the code, or type the address:</p><div class="url mono">${esc(r.primary)}</div>
          <p class="muted">Then tap Share and "Add to Home Screen" (iPhone) or the menu's "Add to Home screen" (Android) to keep it as an app.${r.urls.length > 1 ? "<br>Other addresses: " + r.urls.slice(1).map(esc).join(", ") : ""}</p>
          ${r.nameUrl ? `<div class="alt-name"><div>Or type <span class="url mono">${esc(r.nameUrl)}</span></div><p class="muted">${ICON.info} Easier to remember, but a name like this may not work on all portable devices. If it does not open, use the address above.</p></div>` : ""}
          <a class="btn" href="/phone" target="_blank">Open full-screen QR</a></div>
        </div></div>
      <div class="panel"><h3><span class="ico">${ICON.info}</span>Good to know</h3>
        <ul class="tips">
          <li>Anyone on your home Wi-Fi can open this address. It is not reachable from outside the house.</li>
          <li>The address stays the same as long as this computer keeps its network address. If it changes, come back here for a new code.</li>
          <li>If a phone cannot open it, check it is on the same Wi-Fi (not mobile data or a guest network).</li>
        </ul></div>
    </div>`;
  } catch (e) {
    box.innerHTML = `<div class="panel"><h3><span class="ico">${ICON.home}</span>Open on your phone</h3><p class="muted">${esc(e.message)}</p><p class="muted">The phone link needs the tray app or the Raspberry Pi service; the development server does not provide it. Other devices can still use this computer's network address with port 8484.</p></div>`;
  }
}

// ---------- advanced ----------
$("#rawsend").addEventListener("click", async () => {
  const text = $("#rawcmd").value.trim(); if (!text) return;
  let cmd; try { cmd = JSON.parse(text); } catch { return toast("Not valid JSON", true); }
  const key = Object.keys(cmd)[0] || "";
  if (!/^(GET_|READ_|VIEW_|INFO|FIRMWARE|DEVICES_SN|STATISTICS|ENGINEERS_DATA)/.test(key) && !confirm(`"${key}" looks like a write command. Send it to the hub?`)) return;
  $("#rawout").textContent = "…";
  try { const r = await api("/api/action", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ action: "raw", command: cmd }) }); $("#rawout").textContent = JSON.stringify(r.result, null, 1); loadStatus(true); }
  catch (e) { $("#rawout").textContent = "Error: " + e.message; }
  loadLog();
});
$("#rawcmd").addEventListener("keydown", (e) => { if (e.key === "Enter") $("#rawsend").click(); });
$("#logrefresh").addEventListener("click", loadLog);
async function loadLog() {
  const log = await api("/api/log");
  $("#logtable tbody").innerHTML = log.map((l) => `<tr><td>${new Date(l.at).toLocaleTimeString()}</td><td>${l.ms}</td><td class="mono">${esc(l.cmd)}</td><td class="mono ${l.ok ? "" : "chip bad"}">${esc(l.reply)}</td></tr>`).join("");
}

// ---------- tabs, clock & refresh ----------
function switchTab(name) {
  document.body.dataset.tab = name;
  $$("#tabs button").forEach((b) => b.classList.toggle("active", b.dataset.tab === name));
  $$(".tab").forEach((t) => t.classList.toggle("active", t.id === "tab-" + name));
  window.scrollTo({ top: 0 });
  if (!status) return;
  if (name === "zone") renderZoneDetail();
  if (name === "schedule") loadSchedule();
  if (name === "profiles") renderProfiles();
  if (name === "hub") renderHub();
  if (name === "history") loadHistory();
  if (name === "phone") renderPhone();
  if (name === "host") loadHost();
  if (name === "advanced") loadLog();
}
$("#tabs").addEventListener("click", (e) => { const b = e.target.closest("button[data-tab]"); if (b) switchTab(b.dataset.tab); });
$("#refresh").addEventListener("click", () => loadStatus(true));
function tickClock() {
  const d = new Date();
  $("#clocktime").textContent = `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
  $("#clockdate").textContent = d.toLocaleDateString(undefined, { weekday: "short", day: "numeric", month: "short" });
}
tickClock();
setInterval(tickClock, 15000);
// Deep link: /#phone (from the tray's "Phone link") opens that tab once status has loaded.
const hashTab = location.hash.replace("#", "");
if (hashTab && $(`#tab-${hashTab}`)) { const once = setInterval(() => { if (status) { clearInterval(once); switchTab(hashTab); } }, 200); }
let resizeT;
window.addEventListener("resize", () => { clearTimeout(resizeT); resizeT = setTimeout(() => { if (status && $("#tab-zone").classList.contains("active")) renderZoneDetail(); }, 200); });
refreshTimer = setInterval(() => { if (!document.hidden) loadStatus(); }, 15000);
// Warm the photo cache for the other tabs once the first page is up.
window.addEventListener("load", () => setTimeout(() => PHOTOS.forEach((p) => { const i = new Image(); i.src = p; }), 2500));
loadStatus(true);
