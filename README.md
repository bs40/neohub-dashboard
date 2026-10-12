# neoHub Dashboard

A small local web dashboard for a Heatmiser neoHub (gen 2). It talks to the hub over the
legacy local API on TCP port 4242 (no token needed) and serves a web UI you can open from
any device on your home network.

It comes in two forms that share the same front-end:

- **Windows tray app (Go)**: one `neoHub.exe` that finds the hub on the network by itself,
  sits in the system tray and serves the dashboard. Installed by `neoHub-Setup-x.y.z.exe`.
- **Node script**: `npm start` runs the same server from source, using the hub address in
  `config.json`. Handy for development.

## Windows tray app

Run the installer from `dist\` (built by `build.cmd`, see below). It offers to start neoHub
at sign-in and to add the firewall rule for phones and tablets, and it retires the older
Node-based scheduled task if that was set up.

The tray icon's menu has **Open dashboard**, the hub it found, **Find hub again**,
**Start with Windows** and **Quit**. Settings, the connectivity history and a log live in
`%LOCALAPPDATA%\neoHub`.

Discovery tries the last known address first, then sweeps each local /24 subnet for a
device answering the hub protocol on port 4242 (about a second on a home network). The hub
does not answer the Heatmiser app's "hubseek" broadcast on current firmware, so that is not
used. If the hub stops answering the app searches again automatically.

Run `neoHub.exe -console` to keep a terminal and skip the tray, or `-port 9000` to serve
on another port for one run.

### Building

Needs Go 1.24+ and, for the installer, Inno Setup 6 (`winget install JRSoftware.InnoSetup`).

```bash
build.cmd
```

This embeds the icon and version info, builds `dist\neoHub.exe` (no console window, the
`public` and `img` folders are compiled in) and then `dist\neoHub-Setup-1.0.0.exe`. Set
`NEOHUB_VERSION` to change the version. Go sources: `main.go`, `autostart_windows.go` and
`internal/` (`hub` client, `discover`, `history`, `server`, `config`).

## Node version

```bash
npm start
```

Then open <http://localhost:8484> on this PC, or `http://<this-pc-ip>:8484` from a phone
or tablet on the same network (Windows Firewall must allow TCP 8484 for that).

No dependencies are installed: it is plain Node (v18 or newer) with no build step.

## Configuration

`config.json`:

| Key | Meaning |
|---|---|
| `hubHost` | IP of the neoHub (`192.168.1.46`) |
| `hubPort` | `4242`, the legacy local API |
| `listenHost` | `0.0.0.0` for LAN access, `127.0.0.1` for this PC only |
| `listenPort` | Port the dashboard serves on (`8484`) |
| `statusCacheMs` | How long a full status read is reused before asking the hub again |
| `hubTimeoutMs` | Give up on a hub command after this long |

## What it does

- **Overview**: every zone with current and target temperature, state chips, last 24 h
  sparkline, and quick +/−, standby and end-hold buttons. Bulk standby on/off.
- **Zone**: set temperature, timed hold, standby, keypad lock, rename, apply a stored
  profile, and the engineer settings (frost temperature, switching differential, output
  delay, max preheat, floor limit, user limit). Device info and serial.
- **Schedules**: edit a zone's own comfort levels (wake / leave / return / sleep), copy
  them to other zones. Saving takes the zone off any stored profile; re-apply the profile
  from the Zone tab if you want it linked again.
- **Profiles**: view, apply, rename, delete and create stored profiles.
- **Hub**: clock, NTP on/off, sync time from this device, time zone, DST, away, holiday,
  schedule format, temperature unit, flash the hub LED, and system info.
- **History**: a scrollable connectivity timeline. The server polls the hub once a minute and
  appends a snapshot to `data/history.jsonl` whenever the hub reachability, NTP state, or any
  stat's offline / low-battery flag changes (plus a heartbeat every 10 minutes). The tab shows
  one bar per device for 6 h to 30 days or all history, an event list with durations, and
  per-device totals. Gaps where the dashboard was not running show as "not monitored".
- **Advanced**: send any raw single-key JSON command to the hub and see the command log.

## iPhone home screen

Open the dashboard in Safari, tap Share, then "Add to Home Screen". It installs with the
flame icon (`public/icons/`) and runs full-screen without the Safari chrome.

## Files

- `server.js`: HTTP server and the allowlist of actions the UI may run.
- `hub.js`: tiny TCP client for port 4242 (one command at a time, `\0` terminated).
- `public/`: the front-end (`index.html`, `app.js`, `app.css`).
- `img/`: the autumn photos used as page backgrounds. They are served at `/img/<filename>`;
  the page-to-photo mapping is at the top of `app.css` (the `PHOTOS` list in `app.js` only
  pre-warms the browser cache).

## Look

Dark glass panels over the photos, Playfair Display headings (falls back to Georgia when
offline) and an amber accent. Each tab has its own background: Overview, Zone, Schedules,
Profiles, Hub and Advanced.

## Notes

- The hub only reliably handles one connection at a time, so every command is queued.
- Heatmiser may drop the legacy port in a future firmware. The hub also has a token-secured
  websocket API on port 4243; `hub.js` is the only file that would need a second transport.
