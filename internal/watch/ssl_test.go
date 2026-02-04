package watch

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/datasance/router/internal/qdr"
)

func TestScanSSLProfileDir(t *testing.T) {
	dir := t.TempDir()

	// Empty dir returns empty map
	profiles, err := ScanSSLProfileDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 0 {
		t.Errorf("empty dir: got %d profiles, want 0", len(profiles))
	}

	// Subdir with ca.crt only
	profile1 := filepath.Join(dir, "profile1")
	if err := os.MkdirAll(profile1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profile1, "ca.crt"), []byte("ca"), 0644); err != nil {
		t.Fatal(err)
	}
	profiles, err = ScanSSLProfileDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 {
		t.Fatalf("got %d profiles, want 1", len(profiles))
	}
	p, ok := profiles["profile1"]
	if !ok {
		t.Fatal("profile1 not found")
	}
	if p.Name != "profile1" {
		t.Errorf("profile name = %q, want profile1", p.Name)
	}
	absCa := filepath.Join(dir, "profile1", "ca.crt")
	if p.CaCertFile != absCa {
		t.Errorf("CaCertFile = %q, want %q", p.CaCertFile, absCa)
	}
	if p.CertFile != "" || p.PrivateKeyFile != "" {
		t.Errorf("expected no cert/key when only ca.crt present")
	}

	// Subdir with ca.crt, tls.crt, tls.key
	profile2 := filepath.Join(dir, "profile2")
	if err := os.MkdirAll(profile2, 0755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"ca.crt", "tls.crt", "tls.key"} {
		if err := os.WriteFile(filepath.Join(profile2, f), []byte(f), 0644); err != nil {
			t.Fatal(err)
		}
	}
	profiles, err = ScanSSLProfileDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 {
		t.Fatalf("got %d profiles, want 2", len(profiles))
	}
	p2, ok := profiles["profile2"]
	if !ok {
		t.Fatal("profile2 not found")
	}
	if p2.CaCertFile != filepath.Join(dir, "profile2", "ca.crt") {
		t.Errorf("profile2 CaCertFile = %q", p2.CaCertFile)
	}
	if p2.CertFile != filepath.Join(dir, "profile2", "tls.crt") {
		t.Errorf("profile2 CertFile = %q", p2.CertFile)
	}
	if p2.PrivateKeyFile != filepath.Join(dir, "profile2", "tls.key") {
		t.Errorf("profile2 PrivateKeyFile = %q", p2.PrivateKeyFile)
	}
}

func TestScanSSLProfileDir_Nonexistent(t *testing.T) {
	profiles, err := ScanSSLProfileDir(filepath.Join(t.TempDir(), "nonexistent"))
	if err != nil {
		t.Fatal(err)
	}
	if profiles != nil {
		t.Errorf("nonexistent dir: got %v, want nil", profiles)
	}
}

func TestScanSSLProfileDir_ProfilesAreQdrSslProfile(t *testing.T) {
	dir := t.TempDir()
	profileDir := filepath.Join(dir, "myprofile")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "ca.crt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	profiles, err := ScanSSLProfileDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var _ map[string]qdr.SslProfile = profiles
	if len(profiles) != 1 {
		t.Fatalf("got %d profiles", len(profiles))
	}
}
