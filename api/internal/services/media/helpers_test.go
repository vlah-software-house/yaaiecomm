package media

import "testing"

// --------------------------------------------------------------------------
// Tests for keyFromURL
// --------------------------------------------------------------------------

func TestKeyFromURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "local URL",
			url:  "/media/abc-123-image.jpg",
			want: "abc-123-image.jpg",
		},
		{
			name: "S3 URL",
			url:  "https://cdn.example.com/abc-123-image.jpg",
			want: "abc-123-image.jpg",
		},
		{
			name: "S3 URL with path",
			url:  "https://bucket.s3.amazonaws.com/uploads/abc-123-image.jpg",
			want: "abc-123-image.jpg",
		},
		{
			name: "empty string",
			url:  "",
			want: "",
		},
		{
			name: "just a filename",
			url:  "image.jpg",
			want: "image.jpg",
		},
		{
			name: "local URL with nested path",
			url:  "/media/subdir/image.png",
			want: "subdir/image.png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := keyFromURL(tt.url)
			if got != tt.want {
				t.Errorf("keyFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

// --------------------------------------------------------------------------
// Tests for isValidImageMagicBytes
// --------------------------------------------------------------------------

func TestIsValidImageMagicBytes(t *testing.T) {
	tests := []struct {
		name string
		buf  []byte
		want bool
	}{
		{
			name: "valid JPEG (FF D8 FF E0)",
			buf:  []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10},
			want: true,
		},
		{
			name: "valid JPEG (FF D8 FF E1 — EXIF)",
			buf:  []byte{0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x00},
			want: true,
		},
		{
			name: "valid PNG",
			buf:  []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
			want: true,
		},
		{
			name: "valid GIF87a",
			buf:  []byte{0x47, 0x49, 0x46, 0x38, 0x37, 0x61},
			want: true,
		},
		{
			name: "valid GIF89a",
			buf:  []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61},
			want: true,
		},
		{
			name: "valid WebP",
			buf:  []byte{'R', 'I', 'F', 'F', 0x00, 0x00, 0x00, 0x00, 'W', 'E', 'B', 'P'},
			want: true,
		},
		{
			name: "too short — 3 bytes",
			buf:  []byte{0xFF, 0xD8, 0xFF},
			want: false,
		},
		{
			name: "too short — 0 bytes",
			buf:  []byte{},
			want: false,
		},
		{
			name: "nil buffer",
			buf:  nil,
			want: false,
		},
		{
			name: "random bytes",
			buf:  []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05},
			want: false,
		},
		{
			name: "PDF magic bytes",
			buf:  []byte{0x25, 0x50, 0x44, 0x46, 0x2D},
			want: false,
		},
		{
			name: "ZIP magic bytes",
			buf:  []byte{0x50, 0x4B, 0x03, 0x04},
			want: false,
		},
		{
			name: "WebP with only 11 bytes — too short for full check",
			buf:  []byte{'R', 'I', 'F', 'F', 0x00, 0x00, 0x00, 0x00, 'W', 'E', 'B'},
			want: false,
		},
		{
			name: "RIFF but not WebP",
			buf:  []byte{'R', 'I', 'F', 'F', 0x00, 0x00, 0x00, 0x00, 'A', 'V', 'I', ' '},
			want: false,
		},
		{
			name: "exactly 4 bytes — PNG",
			buf:  []byte{0x89, 0x50, 0x4E, 0x47},
			want: true,
		},
		{
			name: "exactly 4 bytes — not image",
			buf:  []byte{0x00, 0x00, 0x00, 0x00},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidImageMagicBytes(tt.buf)
			if got != tt.want {
				t.Errorf("isValidImageMagicBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --------------------------------------------------------------------------
// Tests for sanitizeFilename
// --------------------------------------------------------------------------

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "normal filename",
			input: "photo.jpg",
			want:  "photo.jpg",
		},
		{
			name:  "filename with spaces stripped",
			input: "my photo.jpg",
			want:  "myphoto.jpg",
		},
		{
			name:  "path traversal stripped",
			input: "../../etc/passwd",
			want:  "passwd",
		},
		{
			name:  "Windows backslash path",
			input: `C:\Users\uploads\file.png`,
			want:  "file.png",
		},
		{
			name:  "special characters stripped",
			input: "photo (1) [final].jpg",
			want:  "photo1final.jpg",
		},
		{
			name:  "unicode characters stripped",
			input: "café-über.png",
			want:  "caf-ber.png",
		},
		{
			name:  "empty string becomes upload",
			input: "",
			want:  "upload",
		},
		{
			name:  "just a dot becomes upload",
			input: ".",
			want:  "upload",
		},
		{
			name:  "all special chars becomes upload",
			input: "!@#$%^&*()",
			want:  "upload",
		},
		{
			name:  "hyphens and underscores preserved",
			input: "my-file_name.webp",
			want:  "my-file_name.webp",
		},
		{
			name:  "multiple dots preserved",
			input: "file.backup.tar.gz",
			want:  "file.backup.tar.gz",
		},
		{
			name:  "mixed case preserved",
			input: "MyFile.PNG",
			want:  "MyFile.PNG",
		},
		{
			name:  "unix path - only last segment kept",
			input: "/var/tmp/uploads/image.jpg",
			want:  "image.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeFilename(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
