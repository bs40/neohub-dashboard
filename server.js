// neoHub dashboard server. No external dependencies.
const http = require("http");
const fs = require("fs");
const path = require("path");
const { NeoHub } = require("./hub");

const config = JSON.parse(fs.readFileSync(path.join(__dirname, "config.json"), "utf8"));
const hub = new NeoHub({ host: config.hubHost, port: config.hubPort, timeoutMs: config.hubTimeoutMs });
const PUBLIC = path.join(__dirname, "public");
const IMG = path.join(__dirname, "img");

// ---------- helpers ----------
const isStr = (v) => typeof v === "string" && v.length > 0;
const num = (v, lo, hi) => {
  const n = Number(v);
  if (!Number.isFinite(n) || n < lo || n > hi) throw new Error(`Value ${v} out of range ${lo}..${hi}`);
  return n;
};
const zones = (v) => {
  if (!Array.isArray(v) || !v.length || !v.every(isStr)) throw new Error("zones must be a non-empty array of zone names");
  return v;
};
const pad2 = (n) => String(n).padStart(2, "0");

function expectResult(reply, ...okTexts) {
  if (reply && reply.error) throw new Error(String(reply.error));
  if (okTexts.length && reply && typeof reply.result === "string" && !okTexts.some((t) => reply.result.toLowerCase().includes(t))) {
    throw new Error("Unexpected hub reply: " + JSON.stringify(reply));
  }
  return reply;
}

// ---------- status (cached) ----------
let statusCache = { at: 0, data: null, pending: null };
async function getStatus(force) {
  const now = Date.now();
  if (!force && statusCache.data && now - statusCache.at < config.statusCacheMs) return statusCache.data;
  if (statusCache.pending) return statusCache.pending;
  statusCache.pending = (async () => {
    const [live, system, engineers, profiles, holiday, firmware, zonesMap, serials] = await Promise.all([
      hub.send({ GET_LIVE_DATA: 0 }), hub.send({ GET_SYSTEM: 0 }), hub.send({ GET_ENGINEERS: 0 }),
      hub.send({ GET_PROFILES: 0 }), hub.send({ GET_HOLIDAY: 0 }), hub.send({ FIRMWARE: 0 }),
      hub.send({ GET_ZONES: 0 }), hub.send({ DEVICES_SN: 0 }),
    ]);
    const data = { fetchedAt: new Date().toISOString(), serverTime: Date.now(), live, system, engineers, profiles, holiday, firmware, zones: zonesMap, serials };
    statusCache = { at: Date.now(), data, pending: null };
    return data;
  })();
  try { return await statusCache.pending; } finally { statusCache.pending = null; }
}
const invalidate = () => { statusCache.at = 0; };

// ---------- connectivity history ----------
// Polls the hub every `historyPollMs` and appends a snapshot line to data/history.jsonl whenever
// anything changes (hub reachable, NTP state, offline or low-battery stats), plus a heartbeat
// snapshot every `historyHeartbeatMs` so the timeline can tell "fine" from "not monitored".
const DATA_DIR = path.join(__dirname, "data");
const HISTORY_FILE = path.join(DATA_DIR, "history.jsonl");
const POLL_MS = config.historyPollMs || 60000;
const HEARTBEAT_MS = config.historyHeartbeatMs || 10 * 60000;
let history = [];
let lastSnap = null;
function loadHistory() {
  try {
    fs.mkdirSync(DATA_DIR, { recursive: true });
    if (fs.existsSync(HISTORY_FILE)) {
      history = fs.readFileSync(HISTORY_FILE, "utf8").split("\n").filter(Boolean).map((l) => { try { return JSON.parse(l); } catch { return null; } }).filter(Boolean);
      lastSnap = history[history.length - 1] || null;
    }
  } catch (e) { console.error("history load failed:", e.message); }
}
function sameSnap(a, b) {
  if (!a || !b) return false;
  return a.hub === b.hub && a.ntp === b.ntp && JSON.stringify(a.off) === JSON.stringify(b.off) && JSON.stringify(a.low) === JSON.stringify(b.low) && JSON.stringify(a.zones) === JSON.stringify(b.zones);
}
function record(snap, force) {
  if (!force && sameSnap(lastSnap, snap) && snap.t - lastSnap.t < HEARTBEAT_MS) return;
  history.push(snap);
  lastSnap = snap;
  fs.appendFile(HISTORY_FILE, JSON.stringify(snap) + "\n", () => {});
}
async function pollOnce(force) {
  const t = Date.now();
  try {
    const s = await getStatus(true);
    const devs = (s.live.devices || []).slice().sort((a, b) => a.DEVICE_ID - b.DEVICE_ID);
    record({
      t, hub: true,
      ntp: s.system.NTP_ON,
      drift: Math.round(s.system.UTC - t / 1000),
      off: devs.filter((d) => d.OFFLINE).map((d) => d.ZONE_NAME).sort(),
      low: devs.filter((d) => d.LOW_BATTERY).map((d) => d.ZONE_NAME).sort(),
      zones: devs.map((d) => d.ZONE_NAME),
    }, force);
  } catch (e) {
    record({ t, hub: false, err: String(e.message || e).slice(0, 120), zones: lastSnap?.zones || [] }, force);
  }
}
loadHistory();
record({ t: Date.now(), hub: null, note: "dashboard started", zones: lastSnap?.zones || [] }, true);
pollOnce(true);
setInterval(() => pollOnce(false), POLL_MS);
process.on("SIGINT", () => { record({ t: Date.now(), hub: null, note: "dashboard stopped", zones: lastSnap?.zones || [] }, true); setTimeout(() => process.exit(0), 150); });

// ---------- actions (the allowlist of things the UI may do) ----------
const actions = {
  // --- zone control ---
  async setTemp({ zones: z, temp }) {
    return expectResult(await hub.send({ SET_TEMP: [num(temp, 5, 35), zones(z)] }), "temperature was set");
  },
  async hold({ zones: z, temp, hours = 0, minutes = 0 }) {
    const t = num(temp, 5, 35), h = num(hours, 0, 99), m = num(minutes, 0, 59);
    const out = [];
    for (const name of zones(z)) out.push(expectResult(await hub.send({ HOLD: [{ temp: t, hours: h, minutes: m, id: name }, [name]] }), "hold"));
    return out;
  },
  async cancelHold({ zones: z }) {
    const out = [];
    for (const name of zones(z)) out.push(expectResult(await hub.send({ HOLD: [{ temp: 20, hours: 0, minutes: 0, id: name }, [name]] }), "hold"));
    return out;
  },
  async standby({ zones: z, on }) {
    return expectResult(await hub.send(on ? { FROST_ON: zones(z) } : { FROST_OFF: zones(z) }), "frost");
  },
  async timer({ zones: z, on }) {
    return expectResult(await hub.send(on ? { TIMER_ON: zones(z) } : { TIMER_OFF: zones(z) }), "timer");
  },
  async lock({ zones: z, pin }) {
    if (!/^\d{4}$/.test(String(pin))) throw new Error("PIN must be 4 digits");
    return expectResult(await hub.send({ LOCK: [String(pin).split("").map(Number), zones(z)] }), "lock");
  },
  async unlock({ zones: z }) { return expectResult(await hub.send({ UNLOCK: zones(z) }), "unlock"); },
  async renameZone({ from, to }) {
    if (!isStr(from) || !isStr(to)) throw new Error("from/to required");
    return expectResult(await hub.send({ ZONE_TITLE: [from, to] }));
  },

  // --- engineer settings ---
  async setFrost({ zones: z, temp }) { return expectResult(await hub.send({ SET_FROST: [num(temp, 5, 17), zones(z)] }), "set"); },
  async setDiff({ zones: z, value }) { return expectResult(await hub.send({ SET_DIFF: [num(value, 0, 3), zones(z)] }), "set"); },
  async setDelay({ zones: z, value }) { return expectResult(await hub.send({ SET_DELAY: [num(value, 0, 15), zones(z)] }), "set"); },
  async setPreheat({ zones: z, value }) { return expectResult(await hub.send({ SET_PREHEAT: [num(value, 0, 5), zones(z)] }), "set"); },
  async setFloor({ zones: z, value }) { return expectResult(await hub.send({ SET_FLOOR: [num(value, 20, 40), zones(z)] })); },
  async setUserLimit({ zones: z, value }) { return expectResult(await hub.send({ USER_LIMIT: [num(value, 0, 35), zones(z)] }), "limit"); },

  // --- schedules / profiles ---
  async setComfortLevels({ zone, levels }) {
    if (!isStr(zone) || !levels || typeof levels !== "object") throw new Error("zone and levels required");
    return expectResult(await hub.send({ SET_COMFORT_LEVELS: [levels, [zone]] }));
  },
  async runProfile({ profileId, zones: z }) {
    return expectResult(await hub.send({ RUN_PROFILE_ID: [num(profileId, 1, 999), zones(z)] }), "profile");
  },
  async clearProfile({ zones: z }) { return expectResult(await hub.send({ CLEAR_CURRENT_PROFILE: zones(z) }), "profile"); },
  async storeProfile({ name, info }) {
    if (!isStr(name) || !info) throw new Error("name and info required");
    return expectResult(await hub.send({ STORE_PROFILE: { name, info } }));
  },
  async deleteProfile({ name }) { if (!isStr(name)) throw new Error("name required"); return expectResult(await hub.send({ CLEAR_PROFILE: name }), "profile"); },
  async renameProfile({ from, to }) {
    if (!isStr(from) || !isStr(to)) throw new Error("from/to required");
    return expectResult(await hub.send({ PROFILE_TITLE: [from, to] }), "profile");
  },

  // --- hub-wide ---
  async away({ on }) { return expectResult(await hub.send(on ? { AWAY_ON: 0 } : { AWAY_OFF: 0 }), "away"); },
  async holiday({ start, end }) {
    const fmt = (iso) => {
      const d = new Date(iso);
      if (isNaN(d)) throw new Error("bad date " + iso);
      return `${pad2(d.getHours())}${pad2(d.getMinutes())}00${pad2(d.getDate())}${pad2(d.getMonth() + 1)}${d.getFullYear()}`;
    };
    return expectResult(await hub.send({ HOLIDAY: [fmt(start), fmt(end)] }));
  },
  async cancelHoliday() { return expectResult(await hub.send({ CANCEL_HOLIDAY: 0 }), "holiday"); },
  async setFormat({ format }) {
    const ok = ["NONPROGRAMMABLE", "24HOURSFIXED", "5DAY/2DAY", "7DAY"];
    if (!ok.includes(format)) throw new Error("format must be one of " + ok.join(", "));
    return expectResult(await hub.send({ SET_FORMAT: format }), "format");
  },
  async setTempFormat({ unit }) {
    if (!["C", "F"].includes(unit)) throw new Error("unit must be C or F");
    return expectResult(await hub.send({ SET_TEMP_FORMAT: unit }), "format");
  },
  async timezone({ tz }) { return expectResult(await hub.send({ TIME_ZONE: num(tz, -12, 14) }), "timezone"); },
  async dst({ on, region = "UK" }) {
    if (!["UK", "EU", "NZ"].includes(region)) throw new Error("region must be UK, EU or NZ");
    return expectResult(await hub.send(on ? { DST_ON: region } : { DST_OFF: 0 }), "dst");
  },
  async ntp({ on }) { return expectResult(await hub.send(on ? { NTP_ON: 0 } : { NTP_OFF: 0 }), "ntp"); },
  async setDateTime({ year, month, day, hour, minute }) {
    const r1 = expectResult(await hub.send({ SET_DATE: [num(year, 2020, 2099), num(month, 1, 12), num(day, 1, 31)] }), "date");
    const r2 = expectResult(await hub.send({ SET_TIME: [num(hour, 0, 23), num(minute, 0, 59)] }), "time");
    return [r1, r2];
  },
  async syncTimeFromPc() {
    const d = new Date();
    return actions.setDateTime({ year: d.getFullYear(), month: d.getMonth() + 1, day: d.getDate(), hour: d.getHours(), minute: d.getMinutes() });
  },
  async identify() { return expectResult(await hub.send({ IDENTIFY: 0 }), "flash"); },
  // ZigBee channel: 11..26. Every stat re-joins afterwards, which can take minutes.
  async setChannel({ channel }) { return expectResult(await hub.send({ SET_CHANNEL: num(channel, 11, 26) }), "channel"); },

  // --- advanced: raw command, any single-key object ---
  async raw({ command }) {
    let cmd = command;
    if (typeof cmd === "string") cmd = JSON.parse(cmd);
    if (!cmd || typeof cmd !== "object" || Object.keys(cmd).length !== 1) throw new Error("command must be a single-key JSON object");
    return hub.send(cmd);
  },
};

// ---------- http ----------
const MIME = { ".html": "text/html; charset=utf-8", ".js": "text/javascript; charset=utf-8", ".css": "text/css; charset=utf-8", ".svg": "image/svg+xml", ".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".webp": "image/webp", ".ico": "image/x-icon", ".woff2": "font/woff2", ".webmanifest": "application/manifest+json", ".json": "application/json" };
const sendJson = (res, code, obj) => {
  const body = JSON.stringify(obj);
  res.writeHead(code, { "Content-Type": "application/json; charset=utf-8", "Cache-Control": "no-store" });
  res.end(body);
};
const readBody = (req) => new Promise((resolve, reject) => {
  let b = "";
  req.on("data", (c) => { b += c; if (b.length > 1e6) reject(new Error("body too large")); });
  req.on("end", () => resolve(b));
  req.on("error", reject);
});

const server = http.createServer(async (req, res) => {
  const url = new URL(req.url, "http://x");
  try {
    if (url.pathname === "/api/status" && req.method === "GET") return sendJson(res, 200, await getStatus(url.searchParams.has("force")));
    if (url.pathname === "/api/zone" && req.method === "GET") {
      const name = url.searchParams.get("name");
      if (!isStr(name)) return sendJson(res, 400, { error: "name required" });
      const [profile0, comfort, timer0, roc] = await Promise.all([
        hub.send({ GET_PROFILE_0: name }), hub.send({ READ_COMFORT_LEVELS: [name] }), hub.send({ GET_TIMER_0: name }), hub.send({ VIEW_ROC: [name] }),
      ]);
      return sendJson(res, 200, { profile0, comfort, timer0, roc });
    }
    if (url.pathname === "/api/templog" && req.method === "GET") {
      const name = url.searchParams.get("name");
      if (!isStr(name)) return sendJson(res, 400, { error: "name required" });
      return sendJson(res, 200, await hub.send({ GET_TEMPLOG: [name] }));
    }
    if (url.pathname === "/api/log" && req.method === "GET") return sendJson(res, 200, hub.log.slice().reverse());
    if (url.pathname === "/api/history" && req.method === "GET") {
      const since = Number(url.searchParams.get("since") || 0);
      // Include the last snapshot before `since` so the timeline knows the state at the left edge.
      let start = history.findIndex((h) => h.t >= since);
      if (start < 0) start = history.length;
      if (start > 0) start -= 1;
      return sendJson(res, 200, { now: Date.now(), pollMs: POLL_MS, heartbeatMs: HEARTBEAT_MS, snapshots: history.slice(start) });
    }
    if (url.pathname === "/api/config" && req.method === "GET") return sendJson(res, 200, { hubHost: config.hubHost, hubPort: config.hubPort, listenPort: config.listenPort });
    if (url.pathname === "/api/action" && req.method === "POST") {
      const body = JSON.parse((await readBody(req)) || "{}");
      const fn = actions[body.action];
      if (!fn) return sendJson(res, 400, { error: "unknown action " + body.action });
      const result = await fn(body);
      invalidate();
      return sendJson(res, 200, { ok: true, result });
    }
    if (req.method === "GET") {
      // /img/* is served from the project's img folder (background photos); everything else from public/.
      const isImg = url.pathname.startsWith("/img/");
      const root = isImg ? IMG : PUBLIC;
      const p = url.pathname === "/" ? "/index.html" : isImg ? url.pathname.slice(4) : url.pathname;
      const file = path.normalize(path.join(root, decodeURIComponent(p)));
      if (!file.startsWith(root) || !fs.existsSync(file) || fs.statSync(file).isDirectory()) { res.writeHead(404); return res.end("not found"); }
      res.writeHead(200, { "Content-Type": MIME[path.extname(file).toLowerCase()] || "application/octet-stream", "Cache-Control": isImg ? "public, max-age=86400" : "no-cache" });
      return fs.createReadStream(file).pipe(res);
    }
    res.writeHead(405); res.end();
  } catch (e) {
    sendJson(res, 500, { error: String(e.message || e) });
  }
});

server.listen(config.listenPort, config.listenHost, () => {
  console.log(`neoHub dashboard: http://localhost:${config.listenPort}  (hub ${config.hubHost}:${config.hubPort})`);
});
