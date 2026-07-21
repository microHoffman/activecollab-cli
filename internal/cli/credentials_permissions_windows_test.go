//go:build windows

package cli

import (
	"os"
	"testing"

	"golang.org/x/sys/windows"
)

func TestCredentialFileUsesProtectedWindowsDACL(t *testing.T) {
	options := authTestOptions(t)
	if err := options.saveLoginCredentials(storedConfiguration{URL: "https://activecollab.example.com/api/v1", Token: "secret-token"}); err != nil {
		t.Fatal(err)
	}
	descriptor, err := windows.GetNamedSecurityInfo(options.configPath, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := descriptor.Control()
	if err != nil {
		t.Fatal(err)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatal("credentials DACL is not protected from inherited access rules")
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if dacl == nil {
		t.Fatal("credentials DACL is missing")
	}
	if dacl.AceCount != 3 {
		t.Fatalf("credentials DACL has %d rules, want 3", dacl.AceCount)
	}
	info, err := os.Stat(options.configPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyCredentialProtection(options.configPath, info); err != nil {
		t.Fatal(err)
	}
}

func TestCredentialFileRejectsPermissiveWindowsDACL(t *testing.T) {
	options := authTestOptions(t)
	if err := options.saveLoginCredentials(storedConfiguration{URL: "https://activecollab.example.com/api/v1", Token: "secret-token"}); err != nil {
		t.Fatal(err)
	}
	everyone, err := windows.CreateWellKnownSid(windows.WinWorldSid)
	if err != nil {
		t.Fatal(err)
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{fullControlEntry(everyone, windows.TRUSTEE_IS_WELL_KNOWN_GROUP)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(
		options.configPath,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		acl,
		nil,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := options.loadConfiguration(); err == nil {
		t.Fatal("loadConfiguration() accepted a credentials file readable by Everyone")
	}
}
