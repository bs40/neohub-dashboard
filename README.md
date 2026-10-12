# neoHub Dashboard

A small local web dashboard for a Heatmiser neoHub (gen 2). It talks to the hub over the
legacy local API on TCP port 4242 (no token needed) and serves a web UI you can open from
any device on your home network.

## Run it

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
