package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestGenerateTOTPSecret(t *testing.T) {
	t.Run("generates valid secret", func(t *testing.T) {
		setup, err := GenerateTOTPSecret("ForgeCommerce", "admin@example.com")
		if err != nil {
			t.Fatalf("GenerateTOTPSecret returned error: %v", err)
		}

		if setup.Secret == "" {
			t.Fatal("expected non-empty secret")
		}

		if setup.URL == "" {
			t.Fatal("expected non-empty URL")
		}

		if !strings.HasPrefix(setup.URL, "otpauth://totp/") {
			t.Fatalf("expected otpauth:// URL, got: %s", setup.URL)
		}

		if !strings.Contains(setup.URL, "ForgeCommerce") {
			t.Fatalf("expected URL to contain issuer, got: %s", setup.URL)
		}

		if !strings.Contains(setup.URL, "admin%40example.com") && !strings.Contains(setup.URL, "admin@example.com") {
			t.Fatalf("expected URL to contain account name, got: %s", setup.URL)
		}

		if len(setup.QRCode) == 0 {
			t.Fatal("expected non-empty QR code PNG")
		}

		// QR code should start with PNG header.
		pngHeader := []byte{0x89, 0x50, 0x4E, 0x47}
		if len(setup.QRCode) < 4 {
			t.Fatal("QR code too short to be a valid PNG")
		}
		for i, b := range pngHeader {
			if setup.QRCode[i] != b {
				t.Fatalf("QR code does not start with PNG header at byte %d: got 0x%02X, want 0x%02X", i, setup.QRCode[i], b)
			}
		}
	})
}

func TestValidateTOTPCode(t *testing.T) {
	t.Run("generate and validate code", func(t *testing.T) {
		setup, err := GenerateTOTPSecret("ForgeCommerce", "admin@example.com")
		if err != nil {
			t.Fatalf("GenerateTOTPSecret returned error: %v", err)
		}

		// Generate a valid code for the current time.
		code, err := totp.GenerateCode(setup.Secret, time.Now())
		if err != nil {
			t.Fatalf("totp.GenerateCode returned error: %v", err)
		}

		if !ValidateTOTPCode(code, setup.Secret) {
			t.Fatal("ValidateTOTPCode should return true for a valid code")
		}
	})

	t.Run("invalid code rejected", func(t *testing.T) {
		setup, err := GenerateTOTPSecret("ForgeCommerce", "admin@example.com")
		if err != nil {
			t.Fatalf("GenerateTOTPSecret returned error: %v", err)
		}

		if ValidateTOTPCode("000000", setup.Secret) {
			// This could theoretically pass if the code happens to be 000000,
			// but the probability is 1 in 1,000,000 so we accept the risk.
			t.Log("warning: 000000 happened to be a valid code (extremely unlikely)")
		}

		if ValidateTOTPCode("invalid", setup.Secret) {
			t.Fatal("ValidateTOTPCode should return false for a non-numeric code")
		}
	})
}

func TestGenerateRecoveryCodes(t *testing.T) {
	t.Run("generates correct number of codes", func(t *testing.T) {
		codes, err := GenerateRecoveryCodes()
		if err != nil {
			t.Fatalf("GenerateRecoveryCodes returned error: %v", err)
		}

		if len(codes.Plaintext) != 8 {
			t.Fatalf("expected 8 plaintext codes, got %d", len(codes.Plaintext))
		}

		if len(codes.Hashed) != 8 {
			t.Fatalf("expected 8 hashed codes, got %d", len(codes.Hashed))
		}
	})

	t.Run("codes have correct format XXXX-XXXX", func(t *testing.T) {
		codes, err := GenerateRecoveryCodes()
		if err != nil {
			t.Fatalf("GenerateRecoveryCodes returned error: %v", err)
		}

		for i, code := range codes.Plaintext {
			if len(code) != 9 { // XXXX-XXXX = 9 chars
				t.Fatalf("code %d has wrong length: %q (expected 9 chars)", i, code)
			}

			if code[4] != '-' {
				t.Fatalf("code %d missing dash at position 4: %q", i, code)
			}

			// Check all characters (excluding dash) are uppercase alphanumeric.
			for j, ch := range code {
				if j == 4 {
					continue // skip dash
				}
				if !strings.ContainsRune(recoveryCodeAlphabet, ch) {
					t.Fatalf("code %d contains invalid character %q at position %d: %q", i, string(ch), j, code)
				}
			}
		}
	})

	t.Run("all codes are unique", func(t *testing.T) {
		codes, err := GenerateRecoveryCodes()
		if err != nil {
			t.Fatalf("GenerateRecoveryCodes returned error: %v", err)
		}

		seen := make(map[string]bool)
		for _, code := range codes.Plaintext {
			if seen[code] {
				t.Fatalf("duplicate recovery code: %q", code)
			}
			seen[code] = true
		}
	})

	t.Run("hashes are valid bcrypt", func(t *testing.T) {
		codes, err := GenerateRecoveryCodes()
		if err != nil {
			t.Fatalf("GenerateRecoveryCodes returned error: %v", err)
		}

		for i, hash := range codes.Hashed {
			if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") {
				t.Fatalf("code %d hash is not bcrypt: %q", i, hash)
			}
		}
	})
}

func TestValidateRecoveryCode(t *testing.T) {
	t.Run("valid code returns correct index", func(t *testing.T) {
		codes, err := GenerateRecoveryCodes()
		if err != nil {
			t.Fatalf("GenerateRecoveryCodes returned error: %v", err)
		}

		// Validate the 3rd code (index 2).
		idx := ValidateRecoveryCode(codes.Plaintext[2], codes.Hashed)
		if idx != 2 {
			t.Fatalf("expected index 2, got %d", idx)
		}
	})

	t.Run("invalid code returns -1", func(t *testing.T) {
		codes, err := GenerateRecoveryCodes()
		if err != nil {
			t.Fatalf("GenerateRecoveryCodes returned error: %v", err)
		}

		idx := ValidateRecoveryCode("ZZZZ-ZZZZ", codes.Hashed)
		if idx != -1 {
			t.Fatalf("expected -1 for invalid code, got %d", idx)
		}
	})

	t.Run("case insensitive matching", func(t *testing.T) {
		codes, err := GenerateRecoveryCodes()
		if err != nil {
			t.Fatalf("GenerateRecoveryCodes returned error: %v", err)
		}

		// Recovery codes are uppercase, but validation should accept lowercase.
		lowered := strings.ToLower(codes.Plaintext[0])
		idx := ValidateRecoveryCode(lowered, codes.Hashed)
		if idx != 0 {
			t.Fatalf("expected index 0 for lowercase code, got %d", idx)
		}
	})

	t.Run("whitespace trimmed", func(t *testing.T) {
		codes, err := GenerateRecoveryCodes()
		if err != nil {
			t.Fatalf("GenerateRecoveryCodes returned error: %v", err)
		}

		padded := "  " + codes.Plaintext[0] + "  "
		idx := ValidateRecoveryCode(padded, codes.Hashed)
		if idx != 0 {
			t.Fatalf("expected index 0 for padded code, got %d", idx)
		}
	})

	t.Run("burning a code - simulate removal", func(t *testing.T) {
		codes, err := GenerateRecoveryCodes()
		if err != nil {
			t.Fatalf("GenerateRecoveryCodes returned error: %v", err)
		}

		// Use the first code.
		targetCode := codes.Plaintext[0]
		idx := ValidateRecoveryCode(targetCode, codes.Hashed)
		if idx != 0 {
			t.Fatalf("expected index 0, got %d", idx)
		}

		// Burn it: remove from the list.
		remaining := make([]string, 0, len(codes.Hashed)-1)
		for i, h := range codes.Hashed {
			if i != idx {
				remaining = append(remaining, h)
			}
		}

		if len(remaining) != 7 {
			t.Fatalf("expected 7 remaining codes, got %d", len(remaining))
		}

		// The burned code should no longer match.
		idx = ValidateRecoveryCode(targetCode, remaining)
		if idx != -1 {
			t.Fatalf("expected -1 for burned code, got %d", idx)
		}

		// Other codes should still work, but with shifted indices.
		idx = ValidateRecoveryCode(codes.Plaintext[1], remaining)
		if idx != 0 { // was index 1, now shifted to 0 after removal
			t.Fatalf("expected index 0 for shifted code, got %d", idx)
		}
	})

	t.Run("empty hashed list returns -1", func(t *testing.T) {
		idx := ValidateRecoveryCode("ABCD-1234", nil)
		if idx != -1 {
			t.Fatalf("expected -1 for nil list, got %d", idx)
		}

		idx = ValidateRecoveryCode("ABCD-1234", []string{})
		if idx != -1 {
			t.Fatalf("expected -1 for empty list, got %d", idx)
		}
	})
}

