package runner

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestValidSignature(t *testing.T) {
	secret := "s3cr3t"
	body := []byte(`{"ref":"refs/heads/main"}`)

	tests := []struct {
		name   string
		secret string
		header string
		body   []byte
		want   bool
	}{
		{"válida", secret, sign(secret, body), body, true},
		{"segredo errado", secret, sign("outro", body), body, false},
		{"corpo adulterado", secret, sign(secret, body), []byte(`{"ref":"x"}`), false},
		{"sem prefixo sha256", secret, hex.EncodeToString([]byte("abc")), body, false},
		{"header vazio", secret, "", body, false},
		{"segredo vazio", "", sign(secret, body), body, false},
		{"hex inválido", secret, "sha256=zzzz", body, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validSignature(tt.secret, tt.header, tt.body); got != tt.want {
				t.Errorf("validSignature() = %v, quer %v", got, tt.want)
			}
		})
	}
}
