package server

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

// phoneRoutes serves the QR code and the "open on your phone" page.
func (s *Server) phoneRoutes() {
	// PNG QR code for any text (used by the Hub tab and the /phone page).
	s.mux.HandleFunc("GET /qr.png", func(w http.ResponseWriter, r *http.Request) {
		text := r.URL.Query().Get("text")
		if text == "" || len(text) > 500 {
			s.fail(w, http.StatusBadRequest, fmt.Errorf("text required"))
			return
		}
		png, err := qrcode.Encode(text, qrcode.Medium, 320)
		if err != nil {
			s.fail(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(png)
	})
	// The addresses other devices can use.
	s.mux.HandleFunc("GET /api/addresses", func(w http.ResponseWriter, r *http.Request) {
		urls := s.lanURLs()
		primary := ""
		if len(urls) > 0 {
			primary = urls[0]
		}
		writeJSON(w, http.StatusOK, map[string]any{"urls": urls, "primary": primary})
	})
	// A standalone page the tray menu opens: big QR code plus the address.
	s.mux.HandleFunc("GET /phone", func(w http.ResponseWriter, r *http.Request) {
		urls := s.lanURLs()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if len(urls) == 0 {
			fmt.Fprint(w, phonePage("", nil))
			return
		}
		fmt.Fprint(w, phonePage(urls[0], urls[1:]))
	})
}

// lanURLs lists dashboard URLs on this machine's LAN addresses, best first.
func (s *Server) lanURLs() []string {
	if s.opt.Addresses == nil {
		return nil
	}
	var out []string
	for _, ip := range s.opt.Addresses() {
		out = append(out, fmt.Sprintf("http://%s:%d/", ip, s.opt.ListenPort))
	}
	return out
}

func phonePage(primary string, others []string) string {
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>neoHub on your phone</title>
<link rel="icon" href="/icons/icon.svg"><style>
body{margin:0;min-height:100vh;display:grid;place-items:center;background:#1a110b;color:#f7f0e6;font:15px/1.5 Inter,system-ui,sans-serif}
a.back{position:fixed;top:14px;left:16px;color:#d5c7b4;text-decoration:none;font-size:14px}
.card{background:rgba(22,15,11,.8);border:1px solid rgba(255,255,255,.1);border-radius:18px;padding:28px 32px;text-align:center;max-width:420px;box-shadow:0 10px 30px rgba(0,0,0,.35)}
h1{font:500 30px/1.1 "Playfair Display",Georgia,serif;margin:0 0 6px}p{margin:6px 0;color:#d5c7b4}
img{width:260px;height:260px;border-radius:12px;background:#fff;padding:12px;margin:14px 0}
.url{font:600 18px ui-monospace,Consolas,monospace;color:#f3b562;word-break:break-all}.alt{font-size:12.5px;color:#a3937f;margin-top:10px}
</style></head><body><a class="back" href="/#phone">‹ Back to the dashboard</a><div class="card"><h1>Phone link</h1>`)
	if primary == "" {
		b.WriteString(`<p>No network address found. Is this PC connected to your home network?</p></div></body></html>`)
		return b.String()
	}
	b.WriteString(`<p>Point your phone's camera at the code, or type the address.</p>`)
	fmt.Fprintf(&b, `<img src="/qr.png?text=%s" alt="QR code for %s"><div class="url">%s</div>`, url.QueryEscape(primary), html.EscapeString(primary), html.EscapeString(primary))
	b.WriteString(`<p class="alt">Then tap Share and "Add to Home Screen" to keep it as an app.`)
	if len(others) > 0 {
		b.WriteString(`<br>Other addresses: `)
		for i, u := range others {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(html.EscapeString(u))
		}
	}
	b.WriteString(`</p></div></body></html>`)
	return b.String()
}
