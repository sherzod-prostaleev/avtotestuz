package b2b

import (
	"bytes"
	"encoding/base64"
	"os/exec"
	"strings"
	"testing"
)

func TestMobilePromoURLValidation(t *testing.T) {
	for _, raw := range []string{"https://drivergo.uz/r/REF-C62LC2", "https://example.com/A%2fb?x=1&x=2+y#Case", "http://example.com"} {
		if err := ValidateMobilePromo(true, raw); err != nil {
			t.Errorf("valid URL rejected: %v", err)
		}
	}
	for _, raw := range []string{"", " https://example.com", "https://example.com\n", "javascript:alert(1)", "//example.com", "https://u:p@example.com", "https://", "https://example.com\\bad", "https://example.com/" + strings.Repeat("x", 512)} {
		if ValidateMobilePromo(true, raw) == nil {
			t.Errorf("accepted invalid URL %q", raw)
		}
	}
	if err := ValidateMobilePromo(false, ""); err != nil {
		t.Fatal(err)
	}
}

// Decode with an independent implementation, not the encoder under test.
func TestMobilePromoQRExactRoundTrip(t *testing.T) {
	decoder, err := exec.LookPath("zbarimg")
	if err != nil {
		t.Skip("install zbarimg for independent QR decoding")
	}
	for _, raw := range []string{"https://drivergo.uz/r/REF-C62LC2", "https://example.com/A%2fb?x=1&x=2+y&next=%2FUz#Case", "https://example.com/" + strings.Repeat("a", 490)} {
		p := MobilePromo{Enabled: true, URL: raw}
		if err := p.GenerateQR(); err != nil {
			t.Fatal(err)
		}
		png, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(p.QRDataURL, "data:image/png;base64,"))
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(decoder, "--quiet", "--raw", "-")
		cmd.Stdin = bytes.NewReader(png)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if string(bytes.TrimSuffix(out, []byte("\n"))) != raw {
			t.Fatalf("QR changed URL: %q", out)
		}
	}
}
