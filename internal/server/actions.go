package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bs40/neohub-dashboard/internal/hub"
)

// actions is the allowlist of things the UI may do, mirroring the Node server. Each takes
// the POSTed parameters and returns the hub's reply (or a list of replies).
type actionFn func(h *hub.Hub, p map[string]any) (any, error)

var actions = map[string]actionFn{
	// --- zone control ---
	"setTemp": func(h *hub.Hub, p map[string]any) (any, error) {
		t, err := num(p, "temp", 5, 35)
		if err != nil {
			return nil, err
		}
		z, err := zones(p)
		if err != nil {
			return nil, err
		}
		return send(h, cmd("SET_TEMP", []any{t, z}), "temperature was set")
	},
	"hold": func(h *hub.Hub, p map[string]any) (any, error) {
		t, err := num(p, "temp", 5, 35)
		if err != nil {
			return nil, err
		}
		hrs, err := numDefault(p, "hours", 0, 99, 0)
		if err != nil {
			return nil, err
		}
		mins, err := numDefault(p, "minutes", 0, 59, 0)
		if err != nil {
			return nil, err
		}
		z, err := zones(p)
		if err != nil {
			return nil, err
		}
		var out []any
		for _, name := range z {
			r, err := send(h, cmd("HOLD", []any{map[string]any{"temp": t, "hours": hrs, "minutes": mins, "id": name}, []string{name}}), "hold")
			if err != nil {
				return nil, err
			}
			out = append(out, r)
		}
		return out, nil
	},
	"cancelHold": func(h *hub.Hub, p map[string]any) (any, error) {
		z, err := zones(p)
		if err != nil {
			return nil, err
		}
		var out []any
		for _, name := range z {
			r, err := send(h, cmd("HOLD", []any{map[string]any{"temp": 20, "hours": 0, "minutes": 0, "id": name}, []string{name}}), "hold")
			if err != nil {
				return nil, err
			}
			out = append(out, r)
		}
		return out, nil
	},
	"standby": onOffZones("FROST_ON", "FROST_OFF", "frost"),
	"timer":   onOffZones("TIMER_ON", "TIMER_OFF", "timer"),
	"lock": func(h *hub.Hub, p map[string]any) (any, error) {
		pin := fmt.Sprint(p["pin"])
		if !regexp.MustCompile(`^\d{4}$`).MatchString(pin) {
			return nil, errors.New("PIN must be 4 digits")
		}
		z, err := zones(p)
		if err != nil {
			return nil, err
		}
		digits := make([]int, 4)
		for i, c := range pin {
			digits[i] = int(c - '0')
		}
		return send(h, cmd("LOCK", []any{digits, z}), "lock")
	},
	"unlock": func(h *hub.Hub, p map[string]any) (any, error) {
		z, err := zones(p)
		if err != nil {
			return nil, err
		}
		return send(h, cmd("UNLOCK", z), "unlock")
	},
	"renameZone": func(h *hub.Hub, p map[string]any) (any, error) {
		from, to := str(p, "from"), str(p, "to")
		if from == "" || to == "" {
			return nil, errors.New("from/to required")
		}
		return send(h, cmd("ZONE_TITLE", []string{from, to}))
	},

	// --- engineer settings ---
	"setFrost":     zoneValue("SET_FROST", "temp", 5, 17, "set"),
	"setDiff":      zoneValue("SET_DIFF", "value", 0, 3, "set"),
	"setDelay":     zoneValue("SET_DELAY", "value", 0, 15, "set"),
	"setPreheat":   zoneValue("SET_PREHEAT", "value", 0, 5, "set"),
	"setFloor":     zoneValue("SET_FLOOR", "value", 20, 40, ""),
	"setUserLimit": zoneValue("USER_LIMIT", "value", 0, 35, "limit"),

	// --- schedules / profiles ---
	"setComfortLevels": func(h *hub.Hub, p map[string]any) (any, error) {
		zone := str(p, "zone")
		levels, ok := p["levels"].(map[string]any)
		if zone == "" || !ok {
			return nil, errors.New("zone and levels required")
		}
		return send(h, cmd("SET_COMFORT_LEVELS", []any{levels, []string{zone}}))
	},
	"runProfile": func(h *hub.Hub, p map[string]any) (any, error) {
		id, err := num(p, "profileId", 1, 999)
		if err != nil {
			return nil, err
		}
		z, err := zones(p)
		if err != nil {
			return nil, err
		}
		return send(h, cmd("RUN_PROFILE_ID", []any{int(id), z}), "profile")
	},
	"clearProfile": func(h *hub.Hub, p map[string]any) (any, error) {
		z, err := zones(p)
		if err != nil {
			return nil, err
		}
		return send(h, cmd("CLEAR_CURRENT_PROFILE", z), "profile")
	},
	"storeProfile": func(h *hub.Hub, p map[string]any) (any, error) {
		name := str(p, "name")
		if name == "" || p["info"] == nil {
			return nil, errors.New("name and info required")
		}
		return send(h, cmd("STORE_PROFILE", map[string]any{"name": name, "info": p["info"]}))
	},
	"deleteProfile": func(h *hub.Hub, p map[string]any) (any, error) {
		name := str(p, "name")
		if name == "" {
			return nil, errors.New("name required")
		}
		return send(h, cmd("CLEAR_PROFILE", name), "profile")
	},
	"renameProfile": func(h *hub.Hub, p map[string]any) (any, error) {
		from, to := str(p, "from"), str(p, "to")
		if from == "" || to == "" {
			return nil, errors.New("from/to required")
		}
		return send(h, cmd("PROFILE_TITLE", []string{from, to}), "profile")
	},

	// --- hub-wide ---
	"away": func(h *hub.Hub, p map[string]any) (any, error) {
		if truthy(p["on"]) {
			return send(h, cmd("AWAY_ON", 0), "away")
		}
		return send(h, cmd("AWAY_OFF", 0), "away")
	},
	"holiday": func(h *hub.Hub, p map[string]any) (any, error) {
		start, err := holidayStamp(str(p, "start"))
		if err != nil {
			return nil, err
		}
		end, err := holidayStamp(str(p, "end"))
		if err != nil {
			return nil, err
		}
		return send(h, cmd("HOLIDAY", []string{start, end}))
	},
	"cancelHoliday": func(h *hub.Hub, p map[string]any) (any, error) {
		return send(h, cmd("CANCEL_HOLIDAY", 0), "holiday")
	},
	"setFormat": func(h *hub.Hub, p map[string]any) (any, error) {
		f := str(p, "format")
		switch f {
		case "NONPROGRAMMABLE", "24HOURSFIXED", "5DAY/2DAY", "7DAY":
		default:
			return nil, errors.New("format must be one of NONPROGRAMMABLE, 24HOURSFIXED, 5DAY/2DAY, 7DAY")
		}
		return send(h, cmd("SET_FORMAT", f), "format")
	},
	"setTempFormat": func(h *hub.Hub, p map[string]any) (any, error) {
		u := str(p, "unit")
		if u != "C" && u != "F" {
			return nil, errors.New("unit must be C or F")
		}
		return send(h, cmd("SET_TEMP_FORMAT", u), "format")
	},
	"timezone": func(h *hub.Hub, p map[string]any) (any, error) {
		tz, err := num(p, "tz", -12, 14)
		if err != nil {
			return nil, err
		}
		return send(h, cmd("TIME_ZONE", tz), "timezone")
	},
	"dst": func(h *hub.Hub, p map[string]any) (any, error) {
		region := str(p, "region")
		if region == "" {
			region = "UK"
		}
		if region != "UK" && region != "EU" && region != "NZ" {
			return nil, errors.New("region must be UK, EU or NZ")
		}
		if truthy(p["on"]) {
			return send(h, cmd("DST_ON", region), "dst")
		}
		return send(h, cmd("DST_OFF", 0), "dst")
	},
	"ntp": func(h *hub.Hub, p map[string]any) (any, error) {
		if truthy(p["on"]) {
			return send(h, cmd("NTP_ON", 0), "ntp")
		}
		return send(h, cmd("NTP_OFF", 0), "ntp")
	},
	"setDateTime": func(h *hub.Hub, p map[string]any) (any, error) {
		y, err := num(p, "year", 2020, 2099)
		if err != nil {
			return nil, err
		}
		mo, err := num(p, "month", 1, 12)
		if err != nil {
			return nil, err
		}
		d, err := num(p, "day", 1, 31)
		if err != nil {
			return nil, err
		}
		hr, err := num(p, "hour", 0, 23)
		if err != nil {
			return nil, err
		}
		mi, err := num(p, "minute", 0, 59)
		if err != nil {
			return nil, err
		}
		return setDateTime(h, int(y), int(mo), int(d), int(hr), int(mi))
	},
	"syncTimeFromPc": func(h *hub.Hub, p map[string]any) (any, error) {
		n := time.Now()
		return setDateTime(h, n.Year(), int(n.Month()), n.Day(), n.Hour(), n.Minute())
	},
	"identify": func(h *hub.Hub, p map[string]any) (any, error) {
		return send(h, cmd("IDENTIFY", 0), "flash")
	},
	// ZigBee channel 11..26. Every stat re-joins afterwards, which can take minutes.
	"setChannel": func(h *hub.Hub, p map[string]any) (any, error) {
		c, err := num(p, "channel", 11, 26)
		if err != nil {
			return nil, err
		}
		return send(h, cmd("SET_CHANNEL", int(c)), "channel")
	},

	// --- advanced: raw command, any single-key object ---
	"raw": func(h *hub.Hub, p map[string]any) (any, error) {
		var c map[string]any
		switch v := p["command"].(type) {
		case string:
			if err := json.Unmarshal([]byte(v), &c); err != nil {
				return nil, fmt.Errorf("command is not valid JSON: %w", err)
			}
		case map[string]any:
			c = v
		}
		if len(c) != 1 {
			return nil, errors.New("command must be a single-key JSON object")
		}
		reply, err := h.Send(c)
		if err != nil {
			return nil, err
		}
		return reply, nil
	},
}

// ---------- helpers ----------

func cmd(name string, arg any) map[string]any { return map[string]any{name: arg} }

func onOffZones(onCmd, offCmd, okText string) actionFn {
	return func(h *hub.Hub, p map[string]any) (any, error) {
		z, err := zones(p)
		if err != nil {
			return nil, err
		}
		if truthy(p["on"]) {
			return send(h, cmd(onCmd, z), okText)
		}
		return send(h, cmd(offCmd, z), okText)
	}
}

func zoneValue(command, key string, lo, hi float64, okText string) actionFn {
	return func(h *hub.Hub, p map[string]any) (any, error) {
		v, err := num(p, key, lo, hi)
		if err != nil {
			return nil, err
		}
		z, err := zones(p)
		if err != nil {
			return nil, err
		}
		if okText == "" {
			return send(h, cmd(command, []any{v, z}))
		}
		return send(h, cmd(command, []any{v, z}), okText)
	}
}

func setDateTime(h *hub.Hub, y, mo, d, hr, mi int) (any, error) {
	r1, err := send(h, cmd("SET_DATE", []int{y, mo, d}), "date")
	if err != nil {
		return nil, err
	}
	r2, err := send(h, cmd("SET_TIME", []int{hr, mi}), "time")
	if err != nil {
		return nil, err
	}
	return []any{r1, r2}, nil
}

// expect checks a hub reply for {"error":...} and, when okTexts are given, that a string
// result mentions one of them.
func expect(reply json.RawMessage, err error, okTexts ...string) (json.RawMessage, error) {
	if err != nil {
		return nil, err
	}
	var r struct {
		Error  any `json:"error"`
		Result any `json:"result"`
	}
	_ = json.Unmarshal(reply, &r)
	if r.Error != nil {
		return nil, fmt.Errorf("%v", r.Error)
	}
	if res, ok := r.Result.(string); ok && len(okTexts) > 0 {
		lower := strings.ToLower(res)
		matched := false
		for _, t := range okTexts {
			if strings.Contains(lower, t) {
				matched = true
			}
		}
		if !matched {
			return nil, fmt.Errorf("unexpected hub reply: %s", reply)
		}
	}
	return reply, nil
}

func num(p map[string]any, key string, lo, hi float64) (float64, error) {
	v, ok := p[key]
	if !ok {
		return 0, fmt.Errorf("%s required", key)
	}
	return checkNum(v, key, lo, hi)
}

func numDefault(p map[string]any, key string, lo, hi, def float64) (float64, error) {
	v, ok := p[key]
	if !ok || v == nil || v == "" {
		return def, nil
	}
	return checkNum(v, key, lo, hi)
}

func checkNum(v any, key string, lo, hi float64) (float64, error) {
	var n float64
	switch t := v.(type) {
	case float64:
		n = t
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err != nil {
			return 0, fmt.Errorf("%s must be a number", key)
		}
		n = f
	default:
		return 0, fmt.Errorf("%s must be a number", key)
	}
	if n < lo || n > hi {
		return 0, fmt.Errorf("%s %v out of range %v..%v", key, n, lo, hi)
	}
	return n, nil
}

func zones(p map[string]any) ([]string, error) {
	raw, ok := p["zones"].([]any)
	if !ok || len(raw) == 0 {
		return nil, errors.New("zones must be a non-empty array of zone names")
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		s, ok := v.(string)
		if !ok || s == "" {
			return nil, errors.New("zones must be a non-empty array of zone names")
		}
		out = append(out, s)
	}
	return out, nil
}

func str(p map[string]any, key string) string {
	s, _ := p[key].(string)
	return strings.TrimSpace(s)
}

func truthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true" || t == "1"
	case float64:
		return t != 0
	}
	return false
}

// holidayStamp converts an ISO-ish local datetime ("2026-10-20T09:00") to the hub's
// HHMMSSDDMMYYYY form.
func holidayStamp(s string) (string, error) {
	if s == "" {
		return "", errors.New("holiday dates required")
	}
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02T15:04:05", time.RFC3339} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t.Format("150405021006"), nil
		}
	}
	return "", fmt.Errorf("bad date %q", s)
}

// send runs a command and applies expect to the reply.
func send(h *hub.Hub, c map[string]any, okTexts ...string) (json.RawMessage, error) {
	reply, err := h.Send(c)
	return expect(reply, err, okTexts...)
}
