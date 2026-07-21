//go:build windows

package cli

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

const credentialFullControl windows.ACCESS_MASK = windows.STANDARD_RIGHTS_REQUIRED | windows.SYNCHRONIZE | 0x1ff

func protectCredentialPath(path string, _ bool) error {
	sids, err := credentialSIDs()
	if err != nil {
		return err
	}
	entries := []windows.EXPLICIT_ACCESS{
		fullControlEntry(sids[0], windows.TRUSTEE_IS_USER),
		fullControlEntry(sids[1], windows.TRUSTEE_IS_WELL_KNOWN_GROUP),
		fullControlEntry(sids[2], windows.TRUSTEE_IS_GROUP),
	}
	acl, err := windows.ACLFromEntries(entries, nil)
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		acl,
		nil,
	)
}

func fullControlEntry(sid *windows.SID, trusteeType windows.TRUSTEE_TYPE) windows.EXPLICIT_ACCESS {
	return windows.EXPLICIT_ACCESS{
		AccessPermissions: credentialFullControl,
		AccessMode:        windows.GRANT_ACCESS,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  trusteeType,
			TrusteeValue: windows.TrusteeValueFromSID(sid),
		},
	}
}

func verifyCredentialProtection(path string, _ os.FileInfo) error {
	descriptor, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		return err
	}
	control, _, err := descriptor.Control()
	if err != nil {
		return err
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		return errors.New("credentials DACL inherits permissions")
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return err
	}
	if dacl == nil || dacl.AceCount != 3 {
		return errors.New("credentials DACL must contain exactly three access rules")
	}
	expectedSIDs, err := credentialSIDs()
	if err != nil {
		return err
	}
	found := make(map[string]bool, len(expectedSIDs))
	for index := uint32(0); index < uint32(dacl.AceCount); index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, index, &ace); err != nil {
			return err
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Mask != credentialFullControl {
			return errors.New("credentials DACL contains an unexpected access rule")
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		matched := false
		for _, expected := range expectedSIDs {
			if sid.Equals(expected) {
				key := expected.String()
				if found[key] {
					return fmt.Errorf("credentials DACL contains duplicate rule for %s", key)
				}
				found[key] = true
				matched = true
				break
			}
		}
		if !matched {
			return errors.New("credentials DACL grants access to an unexpected identity")
		}
	}
	return nil
}

func credentialSIDs() ([3]*windows.SID, error) {
	var result [3]*windows.SID
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return result, err
	}
	result[0] = user.User.Sid
	result[1], err = windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return result, err
	}
	result[2], err = windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	return result, err
}
