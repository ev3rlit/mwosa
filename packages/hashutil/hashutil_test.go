package hashutil

import "testing"

func TestSHA256ReturnsAlgorithmPrefixedHexDigest(t *testing.T) {
	got := SHA256([]byte("mwosa"))
	want := "sha256:14bd721a6cdebbd2e984e51d5c17e7304d9bbbba653367a52b738521b1b168bc"
	if got != want {
		t.Fatalf("SHA256 = %q, want %q", got, want)
	}
}
