package main

import "testing"

func TestGenerateShortName(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{"KralTürk", "KralTurk"},
		{"Christmas FM Non-Stop!", "Christmas-FM-Non-Stop"},
		{"DFM Дискач 90-x", "DFM-90-x"},
		{"  Jazz  &  Blues  ", "Jazz-Blues"},
		{"Café del Mar — Ibiza", "Cafe-del-Mar-Ibiza"},
		{"Ö3 Hitradio", "O3-Hitradio"},
		{"!!! ??? ***", "station"},
		{"", "station"},
		{"Rock_FM.101", "Rock_FM.101"},
	}

	for _, tt := range tests {
		got := GenerateShortName(tt.title)
		if got != tt.want {
			t.Errorf("GenerateShortName(%q) = %q, want %q", tt.title, got, tt.want)
		}
	}
}

func TestGenerateUniqueShortName(t *testing.T) {
	existing := []string{"KralTurk", "kralturk-2"}
	got := GenerateUniqueShortName("KralTürk", existing)
	if got != "KralTurk-3" {
		t.Fatalf("GenerateUniqueShortName = %q, want %q", got, "KralTurk-3")
	}
}

func TestValidateShortName(t *testing.T) {
	valid := []string{"KralTurk", "jazz_fm-1.0", "90-x"}
	for _, s := range valid {
		if err := ValidateShortName(s); err != nil {
			t.Errorf("ValidateShortName(%q) unexpected error: %v", s, err)
		}
	}

	invalid := []string{"", "Kral Türk", "rock/fm", "jazz!"}
	for _, s := range invalid {
		if err := ValidateShortName(s); err == nil {
			t.Errorf("ValidateShortName(%q) expected error, got nil", s)
		}
	}
}
