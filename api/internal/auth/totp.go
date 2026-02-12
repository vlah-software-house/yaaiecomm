package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"github.com/pquerna/otp/totp"
	"github.com/skip2/go-qrcode"
	"golang.org/x/crypto/bcrypt"
)

const (
	recoveryCodeCount  = 8
	recoveryCodeLength = 4 // each half: XXXX-XXXX
)

// recoveryCodeAlphabet is the set of characters used for recovery codes.
// Uppercase alphanumeric, excluding ambiguous characters (0/O, 1/I/L).
const recoveryCodeAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"

// TOTPSetup contains the result of generating a new TOTP secret.
type TOTPSetup struct {
	Secret string // base32-encoded secret key
	URL    string // otpauth:// URL
	QRCode []byte // PNG image of the QR code
}

// RecoveryCodes contains the result of generating recovery codes.
type RecoveryCodes struct {
	Plaintext []string // codes to show the user (e.g., "A3BK-9XMZ")
	Hashed    []string // bcrypt hashes to store in database
}

// GenerateTOTPSecret generates a new TOTP secret for a user.
// Returns the secret key (base32), the OTP URL, and a QR code PNG as bytes.
// Parameters: issuer (e.g., "ForgeCommerce"), accountName (user email).
func GenerateTOTPSecret(issuer, accountName string) (*TOTPSetup, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
	})
	if err != nil {
		return nil, fmt.Errorf("generating TOTP secret: %w", err)
	}

	// Generate QR code PNG at 256x256 pixels.
	qrPNG, err := qrcode.Encode(key.URL(), qrcode.Medium, 256)
	if err != nil {
		return nil, fmt.Errorf("generating QR code: %w", err)
	}

	return &TOTPSetup{
		Secret: key.Secret(),
		URL:    key.URL(),
		QRCode: qrPNG,
	}, nil
}

// ValidateTOTPCode validates a TOTP code against a secret.
// Returns true if the code is valid for the current time window.
func ValidateTOTPCode(code, secret string) bool {
	return totp.Validate(code, secret)
}

// GenerateRecoveryCodes generates 8 random recovery codes.
// Returns the plaintext codes (to show the user) and their bcrypt hashes (to store).
// Recovery code format: XXXX-XXXX where X is uppercase alphanumeric.
func GenerateRecoveryCodes() (*RecoveryCodes, error) {
	codes := &RecoveryCodes{
		Plaintext: make([]string, recoveryCodeCount),
		Hashed:    make([]string, recoveryCodeCount),
	}

	for i := range recoveryCodeCount {
		code, err := generateRecoveryCode()
		if err != nil {
			return nil, fmt.Errorf("generating recovery code %d: %w", i, err)
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hashing recovery code %d: %w", i, err)
		}

		codes.Plaintext[i] = code
		codes.Hashed[i] = string(hash)
	}

	return codes, nil
}

// ValidateRecoveryCode checks a code against the list of hashed codes.
// Returns the index of the matched code (so it can be burned/removed) or -1 if no match.
func ValidateRecoveryCode(code string, hashedCodes []string) int {
	// Normalize: uppercase, trim whitespace.
	normalized := strings.ToUpper(strings.TrimSpace(code))

	for i, hashed := range hashedCodes {
		if bcrypt.CompareHashAndPassword([]byte(hashed), []byte(normalized)) == nil {
			return i
		}
	}

	return -1
}

// generateRecoveryCode generates a single recovery code in XXXX-XXXX format.
func generateRecoveryCode() (string, error) {
	alphabetLen := big.NewInt(int64(len(recoveryCodeAlphabet)))
	totalLen := recoveryCodeLength*2 + 1 // two groups plus dash
	buf := make([]byte, 0, totalLen)

	for i := range recoveryCodeLength * 2 {
		if i == recoveryCodeLength {
			buf = append(buf, '-')
		}

		idx, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			return "", fmt.Errorf("generating random character: %w", err)
		}

		buf = append(buf, recoveryCodeAlphabet[idx.Int64()])
	}

	return string(buf), nil
}
