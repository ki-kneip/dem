//go:build windows

package cli

import (
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"

	"github.com/ki-kneip/dem/internal/core"
)

// persistPath writes paths.Bin and paths.Shims into the current
// user's persistent PATH (HKCU\Environment) and notifies running
// processes (Explorer, in particular) so freshly opened terminals
// pick it up without a logoff/logon. Reports whether PATH is
// configured after this call (true even when nothing needed
// changing).
func persistPath(paths core.Paths) (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()

	current, _, err := k.GetStringValue("Path")
	if err != nil && err != registry.ErrNotExist {
		return false, err
	}

	existing := map[string]bool{}
	for _, p := range strings.Split(current, ";") {
		if p != "" {
			existing[strings.ToLower(p)] = true
		}
	}

	var toAdd []string
	for _, p := range []string{paths.Bin, paths.Shims} {
		if !existing[strings.ToLower(p)] {
			toAdd = append(toAdd, p)
		}
	}
	if len(toAdd) == 0 {
		return true, nil
	}

	newPath := current
	if newPath != "" && !strings.HasSuffix(newPath, ";") {
		newPath += ";"
	}
	newPath += strings.Join(toAdd, ";")

	if err := k.SetStringValue("Path", newPath); err != nil {
		return false, err
	}

	broadcastEnvChange()
	return true, nil
}

// broadcastEnvChange tells running processes (Explorer, mainly) that
// the environment changed, the same signal setx and MSI installers
// send, so new processes they spawn see the updated PATH.
func broadcastEnvChange() {
	user32 := syscall.NewLazyDLL("user32.dll")
	sendMessageTimeout := user32.NewProc("SendMessageTimeoutW")
	env, err := syscall.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}
	const (
		hwndBroadcast   = 0xffff
		wmSettingChange = 0x001A
		smtoAbortIfHung = 0x0002
		timeoutMillis   = 5000
	)
	sendMessageTimeout.Call(
		uintptr(hwndBroadcast),
		uintptr(wmSettingChange),
		0,
		uintptr(unsafe.Pointer(env)),
		uintptr(smtoAbortIfHung),
		uintptr(timeoutMillis),
		0,
	)
}
