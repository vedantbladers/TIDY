package engine

import (
	"testing"

	"tidy/config"
)

func TestIsIgnored(t *testing.T) {
	ignoreList := []string{
		"*.crdownload",
		"*.part",
		"*.tmp",
		".DS_Store",
		"desktop.ini",
	}

	tests := []struct {
		filename string
		expected bool
	}{
		{"download.crdownload", true},
		{"archive.zip.part", true},
		{"temp_data.tmp", true},
		{"TEMP_DATA.TMP", true}, // Case-insensitivity
		{".DS_Store", true},
		{"desktop.ini", true},
		{"document.pdf", false},
		{"image.png", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := IsIgnored(tt.filename, ignoreList)
			if result != tt.expected {
				t.Errorf("IsIgnored(%q) = %v; expected %v", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestMatchFile(t *testing.T) {
	cfg := &config.Config{
		Ignore: []string{
			"*.crdownload",
			"*.part",
			"*.tmp",
		},
		Rules: []config.Rule{
			{
				Name:    "Screenshots",
				Pattern: "Screenshot*",
				Dest:    "/test/Screenshots",
			},
			{
				Name:       "Images",
				Extensions: []string{".png", ".jpg", ".jpeg"},
				Dest:       "/test/Images",
			},
			{
				Name:       "Archives",
				Extensions: []string{".zip", ".tar.gz", ".tar"},
				Dest:       "/test/Compressed",
			},
			{
				Name:       "Documents",
				Extensions: []string{"pdf", "docx"}, // Notice: deliberately testing without leading dot
				Dest:       "/test/Documents",
			},
		},
	}

	tests := []struct {
		name            string
		filename        string
		expectedMatched bool
		expectedIgnored bool
		expectedRule    string
		expectedDest    string
	}{
		{
			name:            "first-match-wins: screenshot matches pattern before image extension",
			filename:        "Screenshot 2026-09-18.png",
			expectedMatched: true,
			expectedIgnored: false,
			expectedRule:    "Screenshots",
			expectedDest:    "/test/Screenshots",
		},
		{
			name:            "standard extension match",
			filename:        "vacation_photo.png",
			expectedMatched: true,
			expectedIgnored: false,
			expectedRule:    "Images",
			expectedDest:    "/test/Images",
		},
		{
			name:            "uppercase extension matches case-insensitively",
			filename:        "FAMILY_PORTRAIT.JPG",
			expectedMatched: true,
			expectedIgnored: false,
			expectedRule:    "Images",
			expectedDest:    "/test/Images",
		},
		{
			name:            "compound extension .tar.gz matches accurately",
			filename:        "server_backup.tar.gz",
			expectedMatched: true,
			expectedIgnored: false,
			expectedRule:    "Archives",
			expectedDest:    "/test/Compressed",
		},
		{
			name:            "extension configured without dot matches properly",
			filename:        "tax_return.pdf",
			expectedMatched: true,
			expectedIgnored: false,
			expectedRule:    "Documents",
			expectedDest:    "/test/Documents",
		},
		{
			name:            "ignored file is flagged and not matched to any rule",
			filename:        "ubuntu-24.04.iso.crdownload",
			expectedMatched: false,
			expectedIgnored: true,
		},
		{
			name:            "ignore precedence: screenshot that is still downloading is ignored",
			filename:        "Screenshot 2026.png.part",
			expectedMatched: false,
			expectedIgnored: true,
		},
		{
			name:            "unmatched file returns false",
			filename:        "unknown_file_type.xyz",
			expectedMatched: false,
			expectedIgnored: false,
		},
		{
			name:            "full path input extracts basename correctly",
			filename:        "/home/vedant/Downloads/vacation.jpg",
			expectedMatched: true,
			expectedIgnored: false,
			expectedRule:    "Images",
			expectedDest:    "/test/Images",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := MatchFile(tt.filename, cfg)

			if res.Matched != tt.expectedMatched {
				t.Errorf("Matched = %v; expected %v", res.Matched, tt.expectedMatched)
			}
			if res.Ignored != tt.expectedIgnored {
				t.Errorf("Ignored = %v; expected %v", res.Ignored, tt.expectedIgnored)
			}
			if res.RuleName != tt.expectedRule {
				t.Errorf("RuleName = %q; expected %q", res.RuleName, tt.expectedRule)
			}
			if res.DestDir != tt.expectedDest {
				t.Errorf("DestDir = %q; expected %q", res.DestDir, tt.expectedDest)
			}
		})
	}
}
