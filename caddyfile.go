package caddyutmtracker

import (
	"strconv"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func init() {
	httpcaddyfile.RegisterHandlerDirective("utm_tracker", parseCaddyfile)
	httpcaddyfile.RegisterDirectiveOrder("utm_tracker", httpcaddyfile.Before, "reverse_proxy")
}

func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var t UTMTracker
	err := t.UnmarshalCaddyfile(h.Dispenser)
	return &t, err
}

// UnmarshalCaddyfile parses the Caddyfile configuration for utm_tracker.
//
//	utm_tracker {
//	    client_id          "op_client_abc123"
//	    client_secret      "op_secret_xyz789"
//	    endpoint           "https://api.openpanel.dev"
//	    cookie_name        "_vid"
//	    cookie_domain      ".example.com"
//	    cookie_max_age     365d
//	    cookie_secure      true
//	    cookie_http_only   true
//	    cookie_same_site   lax
//	    session_cookie     "_sid"
//	    session_max_age    30m
//	    strip_params       false
//	    only_with_params   false
//	    track_methods      GET
//	    ignore_paths       /api/* /static/*
//	    track_paths        /*
//	    batch_size         50
//	    flush_interval     5s
//	    extra_params       ref via partner_id
//	    bot_patterns       MyCustomBot AnotherBot
//	}
func (t *UTMTracker) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // consume directive name

	for d.NextBlock(0) {
		switch d.Val() {
		case "client_id":
			if !d.NextArg() {
				return d.ArgErr()
			}
			t.ClientID = d.Val()

		case "client_secret":
			if !d.NextArg() {
				return d.ArgErr()
			}
			t.ClientSecret = d.Val()

		case "endpoint":
			if !d.NextArg() {
				return d.ArgErr()
			}
			t.Endpoint = d.Val()

		case "cookie_name":
			if !d.NextArg() {
				return d.ArgErr()
			}
			t.CookieName = d.Val()

		case "cookie_domain":
			if !d.NextArg() {
				return d.ArgErr()
			}
			t.CookieDomain = d.Val()

		case "cookie_max_age":
			if !d.NextArg() {
				return d.ArgErr()
			}
			dur, err := parseDuration(d.Val())
			if err != nil {
				return d.Errf("invalid cookie_max_age: %v", err)
			}
			t.CookieMaxAge = int(dur.Seconds())

		case "cookie_secure":
			if !d.NextArg() {
				return d.ArgErr()
			}
			val, err := strconv.ParseBool(d.Val())
			if err != nil {
				return d.Errf("invalid cookie_secure: %v", err)
			}
			t.CookieSecure = &val

		case "cookie_http_only":
			if !d.NextArg() {
				return d.ArgErr()
			}
			val, err := strconv.ParseBool(d.Val())
			if err != nil {
				return d.Errf("invalid cookie_http_only: %v", err)
			}
			t.CookieHTTPOnly = &val

		case "cookie_same_site":
			if !d.NextArg() {
				return d.ArgErr()
			}
			t.CookieSameSite = d.Val()

		case "session_cookie":
			if !d.NextArg() {
				return d.ArgErr()
			}
			t.SessionCookie = d.Val()

		case "session_max_age":
			if !d.NextArg() {
				return d.ArgErr()
			}
			dur, err := parseDuration(d.Val())
			if err != nil {
				return d.Errf("invalid session_max_age: %v", err)
			}
			t.SessionMaxAge = int(dur.Seconds())

		case "strip_params":
			if !d.NextArg() {
				return d.ArgErr()
			}
			val, err := strconv.ParseBool(d.Val())
			if err != nil {
				return d.Errf("invalid strip_params: %v", err)
			}
			t.StripParams = val

		case "only_with_params":
			if !d.NextArg() {
				return d.ArgErr()
			}
			val, err := strconv.ParseBool(d.Val())
			if err != nil {
				return d.Errf("invalid only_with_params: %v", err)
			}
			t.OnlyWithParams = val

		case "track_methods":
			args := d.RemainingArgs()
			if len(args) == 0 {
				return d.ArgErr()
			}
			t.TrackMethods = args

		case "ignore_paths":
			args := d.RemainingArgs()
			if len(args) == 0 {
				return d.ArgErr()
			}
			t.IgnorePaths = args

		case "track_paths":
			args := d.RemainingArgs()
			if len(args) == 0 {
				return d.ArgErr()
			}
			t.TrackPaths = args

		case "batch_size":
			if !d.NextArg() {
				return d.ArgErr()
			}
			val, err := strconv.Atoi(d.Val())
			if err != nil {
				return d.Errf("invalid batch_size: %v", err)
			}
			t.BatchSize = val

		case "flush_interval":
			if !d.NextArg() {
				return d.ArgErr()
			}
			t.FlushInterval = d.Val()

		case "extra_params":
			args := d.RemainingArgs()
			if len(args) == 0 {
				return d.ArgErr()
			}
			t.ExtraParams = args

		case "bot_patterns":
			args := d.RemainingArgs()
			if len(args) == 0 {
				return d.ArgErr()
			}
			t.BotPatterns = args

		default:
			return d.Errf("unrecognized option: %s", d.Val())
		}
	}

	return nil
}

// parseDuration parses a duration string that supports 'd' for days
// in addition to standard Go durations.
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)

	if strings.HasSuffix(s, "d") {
		numStr := strings.TrimSuffix(s, "d")
		days, err := strconv.Atoi(numStr)
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	return time.ParseDuration(s)
}
