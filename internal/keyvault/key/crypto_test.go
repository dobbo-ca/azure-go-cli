package key

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
)

func TestDecodeValue(t *testing.T) {
	got, err := decodeValue("aGVsbG8=", "base64")
	if err != nil || string(got) != "hello" {
		t.Errorf("base64 form gave %q, %v", got, err)
	}
	got, err = decodeValue("hello", "plaintext")
	if err != nil || string(got) != "hello" {
		t.Errorf("plaintext form gave %q, %v", got, err)
	}
	if _, err := decodeValue("not base64!", "base64"); err == nil {
		t.Error("expected an error for a non-base64 value")
	}
}

func TestDecodeHex(t *testing.T) {
	got, err := decodeHex("00ff", "iv")
	if err != nil || len(got) != 2 || got[1] != 0xff {
		t.Errorf("got %v, %v", got, err)
	}
	if got, err := decodeHex("", "iv"); got != nil || err != nil {
		t.Errorf("an empty value should give nil, nil; got %v, %v", got, err)
	}
	if _, err := decodeHex("zz", "iv"); err == nil {
		t.Error("expected an error for a non-hex value")
	}
}

// TestPemImportRoundTrip imports a PEM private key into a JWK, then rebuilds
// the public key from that JWK, as download would. The two must match.
func TestPemImportRoundTrip(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ecDER, err := x509.MarshalECPrivateKey(ecKey)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		block *pem.Block
		want  any
	}{
		{"pkcs1 rsa", &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rsaKey)}, &rsaKey.PublicKey},
		{"sec1 ec", &pem.Block{Type: "EC PRIVATE KEY", Bytes: ecDER}, &ecKey.PublicKey},
	}
	for _, c := range cases {
		jwk, err := pemToJWK(pem.EncodeToMemory(c.block), "")
		if err != nil {
			t.Fatalf("%s: pemToJWK failed: %v", c.name, err)
		}
		pub, err := publicKeyFromJWK(jwk)
		if err != nil {
			t.Fatalf("%s: publicKeyFromJWK failed: %v", c.name, err)
		}
		wantDER, err := x509.MarshalPKIXPublicKey(c.want)
		if err != nil {
			t.Fatal(err)
		}
		gotDER, err := x509.MarshalPKIXPublicKey(pub)
		if err != nil {
			t.Fatal(err)
		}
		if string(gotDER) != string(wantDER) {
			t.Errorf("%s: the round trip changed the public key", c.name)
		}
	}
}

// TestPemToJWKRSAExponent pins the exponent encoding: 65537 is three bytes,
// big-endian, with no leading zero.
func TestPemToJWKRSAExponent(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rsaKey)}
	jwk, err := pemToJWK(pem.EncodeToMemory(block), "")
	if err != nil {
		t.Fatal(err)
	}
	if got := new(big.Int).SetBytes(jwk.E).Int64(); got != int64(rsaKey.E) {
		t.Errorf("exponent round-tripped as %d, want %d", got, rsaKey.E)
	}
	if got := new(big.Int).SetBytes(jwk.N); got.Cmp(rsaKey.N) != 0 {
		t.Error("the modulus did not round-trip")
	}
}

func TestEncodePublicKey(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes, err := encodePublicKey(&rsaKey.PublicKey, "PEM")
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "PUBLIC KEY" {
		t.Fatalf("PEM form gave %q", pemBytes)
	}
	derBytes, err := encodePublicKey(&rsaKey.PublicKey, "DER")
	if err != nil {
		t.Fatal(err)
	}
	if string(derBytes) != string(block.Bytes) {
		t.Error("the DER form and the PEM body differ")
	}
	if _, err := encodePublicKey(&rsaKey.PublicKey, "JWK"); err == nil {
		t.Error("expected an error for an unsupported encoding")
	}
}

func TestPublicKeyFromJWKRejectsUnsupported(t *testing.T) {
	if _, err := publicKeyFromJWK(&azkeys.JSONWebKey{Kty: to.Ptr(azkeys.KeyType("oct"))}); err == nil {
		t.Error("expected an error for an oct key")
	}
	if _, err := publicKeyFromJWK(&azkeys.JSONWebKey{
		Kty: to.Ptr(azkeys.KeyType("EC")),
		Crv: to.Ptr(azkeys.CurveName("SECP256K1")),
		X:   []byte{1}, Y: []byte{2},
	}); err == nil {
		t.Error("expected an error for a curve Go does not implement")
	}
}
