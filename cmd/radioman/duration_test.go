package main

import (
	"testing"
	"time"
)

func TestParseAutoStopDuration(t *testing.T) {
	valid := []struct {
		in   string
		want time.Duration
	}{
		{"", 0},
		{"0", 0},
		{"off", 0},
		{"OFF", 0},
		{"none", 0},
		{"false", 0},
		{"disabled", 0},
		{"0m", 0},
		{"0s", 0},
		{"0.0h", 0},
		{"5000", 5000 * time.Millisecond},
		{"500ms", 500 * time.Millisecond},
		{"250 msecs", 250 * time.Millisecond},
		{"1 millisecond", time.Millisecond},
		{"30s", 30 * time.Second},
		{"45 secs", 45 * time.Second},
		{"10m", 10 * time.Minute},
		{"10 mins", 10 * time.Minute},
		{".5m", 30 * time.Second},
		{"1.5h", 90 * time.Minute},
		{"2 hours", 2 * time.Hour},
		{"2d", 48 * time.Hour},
		{"2 days", 48 * time.Hour},
		{"1w", 7 * 24 * time.Hour},
		{"2 weeks", 14 * 24 * time.Hour},
		{"1y", 31557600 * time.Second},
		{" 15M ", 15 * time.Minute},
	}

	for _, tc := range valid {
		got, err := ParseAutoStopDuration(tc.in)
		if err != nil {
			t.Errorf("ParseAutoStopDuration(%q) unexpected error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseAutoStopDuration(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}

	invalid := []string{
		"-5m",
		"-10",
		"abc",
		"10x",
		".",
		"10m 5s",
		"9999999999y",
		"0.0000001ms",
	}

	for _, in := range invalid {
		if got, err := ParseAutoStopDuration(in); err == nil {
			t.Errorf("ParseAutoStopDuration(%q) = %v, expected error", in, got)
		}
	}
}

func TestNormalizeAutoStopValue(t *testing.T) {
	val, dur, err := NormalizeAutoStopValue("off")
	if err != nil || val != "0" || dur != 0 {
		t.Fatalf("NormalizeAutoStopValue(off) = (%q, %v, %v), want (\"0\", 0, nil)", val, dur, err)
	}

	val, dur, err = NormalizeAutoStopValue(" 10m ")
	if err != nil || val != "10m" || dur != 10*time.Minute {
		t.Fatalf("NormalizeAutoStopValue(\" 10m \") = (%q, %v, %v), want (\"10m\", 10m, nil)", val, dur, err)
	}
}

func TestFormatRemaining(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{-5 * time.Second, "0s"},
		{0, "0s"},
		{200 * time.Millisecond, "1s"},
		{45 * time.Second, "45s"},
		{9*time.Minute + 42*time.Second, "9m42s"},
		{10 * time.Minute, "10m"},
		{1*time.Hour + 23*time.Minute, "1h23m"},
		{2 * time.Hour, "2h"},
	}

	for _, tc := range cases {
		if got := FormatRemaining(tc.in); got != tc.want {
			t.Errorf("FormatRemaining(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
