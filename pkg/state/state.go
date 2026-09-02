package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"auto-wuwa-discord-signin/pkg/config"
)

const (
	StateIdle    = "idle"
	StateRunning = "running"
	StateSuccess = "success"
	StateError0  = "error0" // Unbound
	StateError1  = "error1" // Button Cooling down (5 fails)
	StateError2  = "error2" // No Sign-in Event yet
	StateError3  = "error3" // Discord not signed in / invalid token
	StateError4  = "error4" // No Wuthering Waves server found
)

const StateFileName = "state.json"

var stateMu sync.Mutex

type StateData struct {
	State            string `json:"state"`
	Message          string `json:"message"`
	LastCheckTime    string `json:"last_check_time"`     // ISO string in UTC+8
	LastSuccessDate  string `json:"last_success_date"`   // Logical date YYYY-MM-DD (UTC+8 with 4 AM cutoff)
	TotalSuccessDays int    `json:"total_success_days"`
	Stopped          bool   `json:"stopped"`
}

func GetLogicalDateUTC8(t time.Time) string {
	loc := time.FixedZone("UTC+8", 8*3600)
	tUTC8 := t.In(loc)
	logicalTime := tUTC8.Add(-4 * time.Hour)
	return logicalTime.Format("2006-01-02")
}

// NextResetTimeUTC8 computes the next exact 04:00:05 AM (UTC+8) occurrence after the given time.
func NextResetTimeUTC8(now time.Time) time.Time {
	loc := time.FixedZone("UTC+8", 8*3600)
	nowUTC8 := now.In(loc)

	target := time.Date(nowUTC8.Year(), nowUTC8.Month(), nowUTC8.Day(), 4, 0, 5, 0, loc)
	if !nowUTC8.Before(target) {
		target = target.AddDate(0, 0, 1)
	}
	return target
}

func FormatTimeUTC8(t time.Time) string {
	loc := time.FixedZone("UTC+8", 8*3600)
	return t.In(loc).Format("2006-01-02 15:04:05 MST")
}

func GetStatePath() (string, error) {
	dir, err := config.GetAppDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, StateFileName), nil
}

func LoadState() (StateData, error) {
	stateMu.Lock()
	defer stateMu.Unlock()

	statePath, err := GetStatePath()
	if err != nil {
		return StateData{State: StateIdle}, err
	}

	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		initial := StateData{
			State:            StateIdle,
			Message:          "No check performed yet",
			Stopped:          false,
			TotalSuccessDays: 0,
		}
		_ = saveStateUnsafe(initial, statePath)
		return initial, nil
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		return StateData{State: StateIdle}, fmt.Errorf("failed to read state file: %w", err)
	}

	var sd StateData
	if err := json.Unmarshal(data, &sd); err != nil {
		return StateData{State: StateIdle}, fmt.Errorf("failed to parse state file: %w", err)
	}

	return sd, nil
}

func SaveState(sd StateData) error {
	stateMu.Lock()
	defer stateMu.Unlock()

	statePath, err := GetStatePath()
	if err != nil {
		return err
	}

	return saveStateUnsafe(sd, statePath)
}

func saveStateUnsafe(sd StateData, statePath string) error {
	data, err := json.MarshalIndent(sd, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	tmpPath := statePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return os.WriteFile(statePath, data, 0644)
	}

	if err := os.Rename(tmpPath, statePath); err != nil {
		_ = os.Remove(tmpPath)
		return os.WriteFile(statePath, data, 0644)
	}

	return nil
}

func IsAlreadySignedInToday(sd StateData, now time.Time) bool {
	if sd.State != StateSuccess {
		return false
	}
	currentLogicalDate := GetLogicalDateUTC8(now)
	return sd.LastSuccessDate == currentLogicalDate
}
