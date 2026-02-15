package caddyutmtracker

import "testing"

func TestIsBot(t *testing.T) {
	tests := []struct {
		name          string
		ua            string
		extraPatterns []string
		want          bool
	}{
		{
			name: "Googlebot",
			ua:   "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
			want: true,
		},
		{
			name: "Bingbot",
			ua:   "Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
			want: true,
		},
		{
			name: "facebookexternalhit",
			ua:   "facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)",
			want: true,
		},
		{
			name: "normal browser Chrome",
			ua:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			want: false,
		},
		{
			name: "normal browser Firefox",
			ua:   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:121.0) Gecko/20100101 Firefox/121.0",
			want: false,
		},
		{
			name: "empty UA",
			ua:   "",
			want: true,
		},
		{
			name:          "custom bot pattern",
			ua:            "MyCustomCrawler/1.0",
			extraPatterns: []string{"MyCustomCrawler"},
			want:          true,
		},
		{
			name:          "custom pattern no match",
			ua:            "Mozilla/5.0 Normal Browser",
			extraPatterns: []string{"MyCustomCrawler"},
			want:          false,
		},
		{
			name: "GPTBot",
			ua:   "GPTBot/1.0",
			want: true,
		},
		{
			name: "ClaudeBot",
			ua:   "ClaudeBot/1.0",
			want: true,
		},
		{
			name: "HeadlessChrome",
			ua:   "Mozilla/5.0 HeadlessChrome/120.0",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBot(tt.ua, tt.extraPatterns)
			if got != tt.want {
				t.Errorf("isBot(%q) = %v, want %v", tt.ua, got, tt.want)
			}
		})
	}
}
