package utils

import "testing"

func TestGenerateRoomCode_DefaultPattern(t *testing.T) {
	code, err := GenerateRoomCode(6)
	if err != nil {
		t.Fatalf("GenerateRoomCode returned error: %v", err)
	}

	if len(code) != 6 {
		t.Fatalf("code length = %d, want 6", len(code))
	}

	for index, char := range code {
		switch {
		case index < 4 && (char < 'A' || char > 'Z'):
			t.Fatalf("code[%d] = %q, want uppercase letter", index, char)
		case index >= 4 && (char < '2' || char > '9'):
			t.Fatalf("code[%d] = %q, want digit 2-9", index, char)
		}
	}
}
