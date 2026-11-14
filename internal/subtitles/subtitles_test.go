package subtitles

import (
	"testing"
)

const sampleSRT = "\uFEFF1\r\n00:00:01,000 --> 00:00:03,500\r\n{\\an8}Hello\r\n\r\n2\r\n00:00:10,000 --> 00:00:12,000\r\nSecond line\r\n"

func TestSRTToVTT(t *testing.T) {
	got := SRTToVTT(sampleSRT, 0)
	want := "WEBVTT\n\n00:00:01.000 --> 00:00:03.500\nHello\n\n00:00:10.000 --> 00:00:12.000\nSecond line\n"
	if got != want {
		t.Fatalf("expected:\n%q\ngot:\n%q", want, got)
	}

	gotShifted := SRTToVTT(sampleSRT, 5)
	wantShifted := "WEBVTT\n\n00:00:05.000 --> 00:00:07.000\nSecond line\n"
	if gotShifted != wantShifted {
		t.Fatalf("expected:\n%q\ngot:\n%q", wantShifted, gotShifted)
	}

	// REQ-0006 scenario: +0.5s shift (offset = -0.5) delays cues by 500ms
	gotDelayed := SRTToVTT(sampleSRT, -0.5)
	wantDelayed := "WEBVTT\n\n00:00:01.500 --> 00:00:04.000\nHello\n\n00:00:10.500 --> 00:00:12.500\nSecond line\n"
	if gotDelayed != wantDelayed {
		t.Fatalf("expected:\n%q\ngot:\n%q", wantDelayed, gotDelayed)
	}
}

func TestSubtitleLanguageMatchingAndHostAllowlist(t *testing.T) {
	if !MatchesLanguage("eng", "en") || !MatchesLanguage("nld", "nl") || !MatchesLanguage("dut", "nl") || !MatchesLanguage("pob", "pt") {
		t.Fatalf("expected 3-letter codes to match 2-letter preferences")
	}
	if MatchesLanguage("eng", "off") || MatchesLanguage("fra", "en") {
		t.Fatalf("unexpected positive match")
	}

	if NormalizeSubtitleLang("eng") != "en" || NormalizeSubtitleLang("dut") != "nl" || NormalizeSubtitleLang("jpn") != "ja" || NormalizeSubtitleLang("off") != "off" {
		t.Fatalf("unexpected NormalizeSubtitleLang output")
	}

	if LanguageLabel("pob") != "Portuguese (Brazil)" || LanguageLabel("zht") != "Chinese (Traditional)" || LanguageLabel("eng") != "English" {
		t.Fatalf("unexpected LanguageLabel output")
	}

	if !IsAllowedSubtitleURL("https://subs5.strem.io/en/12345.srt") || !IsAllowedSubtitleURL("https://strem.io/sub.srt") {
		t.Fatalf("expected valid strem.io HTTPS URLs to be allowed")
	}
	if IsAllowedSubtitleURL("http://subs5.strem.io/en/12345.srt") ||
		IsAllowedSubtitleURL("https://evil-strem.io/sub.srt") ||
		IsAllowedSubtitleURL("http://127.0.0.1:8080/sub.srt") {
		t.Fatalf("expected non-HTTPS or non-strem.io URLs to be rejected")
	}
}
