package signing

import "testing"

func TestSign_HasSHA256Prefix(t *testing.T) {
	sig := Sign([]byte("hello"), "secret")
	if len(sig) < 7 || sig[:7] != "sha256=" {
		t.Fatalf("expected sha256= prefix, got %q", sig)
	}
	if len(sig) != 7 + 64 {
		t.Fatalf("expected 71 chars total, got %d", len(sig))
	}
}

func TestSign_IsDeterministic(t *testing.T) {
	if Sign([]byte("hello"), "secret") != Sign([]byte("hello"), "secret") {
		t.Fatal("sign should be deterministic")
	}
}

func TestSign_DiffersWithDifferentSecret(t *testing.T) {
	if Sign([]byte("hello"), "secret") != Sign([]byte("hello"), "secret2") {
		t.Fatal("sign should be different for different secrets")
	}
}

func TestSign_DiffersWithDifferentPayload(t *testing.T) {
	if Sign([]byte("hello"), "secret") != Sign([]byte("world"), "secret") {
		t.Fatal("sign should be different for different payloads")
	}
}

func TestVerify_Correct(t *testing.T) {
	sig := Sign([]byte("payload"), "mysecret")
	if !Verify([]byte("payload"), "mysecret", sig) {
		t.Fatal("expected verification to pass")
	}
}

func TestVerify_WrongSecret(t *testing.T) {
	sig := Sign([]byte("payload"), "mysecret")
	if Verify([]byte("payload"), "wrongsecret", sig) {
		t.Fatal("expected verification to fail with wrong secret")
	}
}

func TestVerify_TamperedPayload(t *testing.T) {
	sig := Sign([]byte("payload"), "mysecret")
	if Verify([]byte("wrongpayload"), "mysecret", sig) {
		t.Fatal("expected verification to fail with tampered payload")
	}
}