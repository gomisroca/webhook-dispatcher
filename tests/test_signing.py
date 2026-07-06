
from app.signing import sign, verify


def test_sign_produces_sha256_prefix():
    sig = sign(b"hello", "secret")
    assert sig.startswith("sha256=")
    assert len(sig) == len("sha256=") + 64 # 32 bytes hex-encoded

def test_sign_is_deterministic():
    assert sign(b"hello", "secret") == sign(b"hello", "secret")

def test_sign_differs_with_different_secret():
    assert sign(b"hello", "secret") != sign(b"hello", "different")

def test_sign_differs_with_different_payload():
    assert sign(b"hello", "secret") != sign(b"world", "secret")

def test_verify_correct():
    sig  = sign(b"payload", "mysecret")
    assert verify(b"payload", "mysecret", sig) is True

def test_verify_wrong_secret():
    sig  = sign(b"payload", "mysecret")
    assert verify(b"payload", "wrongsecret", sig) is False

def test_verify_tampered_payload():
    sig = sign(b"original", "mysecret")
    assert verify(b"tampered", "mysecret", sig) is False