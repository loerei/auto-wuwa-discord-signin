package autostart

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const (
	AppName     = "WuWaDiscordAutoSignin"
	RegistryKey = `Software\Microsoft\Windows\CurrentVersion\Run`
)

// Enable adds the current executable to Windows Startup.
func Enable() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	exePath = filepath.Clean(exePath)

	k, err := registry.OpenKey(registry.CURRENT_USER, RegistryKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %w", err)
	}
	defer k.Close()

	if err := k.SetStringValue(AppName, fmt.Sprintf(`"%s"`, exePath)); err != nil {
		return fmt.Errorf("failed to set startup registry value: %w", err)
	}

	return nil
}

// Disable removes the current executable from Windows Startup.
func Disable() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, RegistryKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %w", err)
	}
	defer k.Close()

	if err := k.DeleteValue(AppName); err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("failed to delete startup registry value: %w", err)
	}

	return nil
}

// IsEnabled checks if the app is currently set to launch on Windows Startup.
func IsEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, RegistryKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	val, _, err := k.GetStringValue(AppName)
	if err != nil || val == "" {
		return false
	}
	return true
}
