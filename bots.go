package caddyutmtracker

import "strings"

// defaultBotPatterns contains User-Agent substrings for known bots.
var defaultBotPatterns = []string{
	"Googlebot",
	"Bingbot",
	"bingbot",
	"Slurp",
	"DuckDuckBot",
	"Baiduspider",
	"YandexBot",
	"facebookexternalhit",
	"Facebot",
	"Twitterbot",
	"LinkedInBot",
	"Applebot",
	"PingdomBot",
	"UptimeRobot",
	"AhrefsBot",
	"SemrushBot",
	"MJ12bot",
	"Sogou",
	"Exabot",
	"ia_archiver",
	"Mediapartners-Google",
	"APIs-Google",
	"AdsBot-Google",
	"Storebot-Google",
	"Bytespider",
	"GPTBot",
	"ClaudeBot",
	"CCBot",
	"PetalBot",
	"DataForSeoBot",
	"bot",
	"crawler",
	"spider",
	"headless",
	"PhantomJS",
	"HeadlessChrome",
}

// isBot checks if the User-Agent matches any known bot pattern.
func isBot(ua string, extraPatterns []string) bool {
	if ua == "" {
		return true // empty UA is suspicious, treat as bot
	}

	uaLower := strings.ToLower(ua)

	for _, pattern := range defaultBotPatterns {
		if strings.Contains(uaLower, strings.ToLower(pattern)) {
			return true
		}
	}

	for _, pattern := range extraPatterns {
		if strings.Contains(uaLower, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}
