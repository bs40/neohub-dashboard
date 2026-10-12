// Minimal client for the Heatmiser neoHub legacy TCP API (port 4242).
// Each command is a single-key JSON object terminated with "\0".
// All commands are serialised through one queue: the hub copes badly with
// several simultaneous connections.
const net = require("net");

class NeoHub {
  constructor({ host, port = 4242, timeoutMs = 15000 }) {
    this.host = host;
    this.port = port;
    this.timeoutMs = timeoutMs;
    this.queue = Promise.resolve();
    this.log = [];
  }

  // Run a command. `cmd` is the full single-key object, e.g. {SET_TEMP:[20,["Hall"]]}.
  send(cmd) {
    const run = () => this._sendNow(cmd);
    const p = this.queue.then(run, run);
    this.queue = p.catch(() => {});
    return p;
  }

  _sendNow(cmd) {
    const payload = JSON.stringify(cmd);
    const started = Date.now();
    return new Promise((resolve, reject) => {
      let buf = "";
      let done = false;
      const sock = net.createConnection({ host: this.host, port: this.port });
      const finish = (err, val) => {
        if (done) return;
        done = true;
        clearTimeout(timer);
        sock.destroy();
        const entry = { at: new Date().toISOString(), ms: Date.now() - started, cmd: payload.slice(0, 200), ok: !err,
          reply: err ? String(err.message || err) : JSON.stringify(val).slice(0, 200) };
        this.log.push(entry);
        if (this.log.length > 200) this.log.shift();
        err ? reject(err) : resolve(val);
      };
      const timer = setTimeout(() => finish(new Error(`Hub timeout after ${this.timeoutMs} ms for ${payload.slice(0, 80)}`)), this.timeoutMs);
      sock.on("connect", () => sock.write(payload + "\0"));
      sock.on("data", (d) => {
        buf += d.toString("utf8");
        if (buf.endsWith("\0")) {
          const text = buf.replace(/\0+$/, "");
          try { finish(null, JSON.parse(text)); }
          catch (e) { finish(new Error("Hub returned non-JSON: " + text.slice(0, 200))); }
        }
      });
      sock.on("error", (e) => finish(e));
      sock.on("close", () => { if (!done) finish(new Error("Hub closed connection without a reply")); });
    });
  }
}

module.exports = { NeoHub };
