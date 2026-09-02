package tray

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/getlantern/systray"

	"auto-wuwa-discord-signin/pkg/automation"
	"auto-wuwa-discord-signin/pkg/autostart"
	"auto-wuwa-discord-signin/pkg/config"
	"auto-wuwa-discord-signin/pkg/state"
)

type TrayApp struct {
	runner      *automation.Runner
	mStatus     *systray.MenuItem
	mDays       *systray.MenuItem
	mToggleAuto *systray.MenuItem
	mCheckNow   *systray.MenuItem
	mOpenConfig *systray.MenuItem
	mAutostart  *systray.MenuItem
	mUninstall  *systray.MenuItem
	mQuit       *systray.MenuItem
	mu          sync.Mutex
	isExecuting bool
}

func NewTrayApp() *TrayApp {
	return &TrayApp{
		runner: automation.NewRunner(),
	}
}

func (app *TrayApp) Run() {
	systray.Run(app.onReady, app.onExit)
}

func (app *TrayApp) onReady() {
	// Enable Start with Windows by default on launch
	if !autostart.IsEnabled() {
		_ = autostart.Enable()
	}

	systray.SetIcon(GenerateIconICO(state.StateIdle))
	systray.SetTitle("WuWa Discord Sign-In")
	systray.SetTooltip(fmt.Sprintf("Wuthering Waves Discord Auto Sign-in (%s)", config.AppVersion))

	app.mStatus = systray.AddMenuItem("Status: Initializing...", "Current automation status")
	app.mStatus.Disable()

	app.mDays = systray.AddMenuItem("Signed In: 0 days", "Total days successfully signed in")
	app.mDays.Disable()

	systray.AddSeparator()

	app.mCheckNow = systray.AddMenuItem("Check & Sign In Now", "Trigger immediate sign-in cycle")
	app.mToggleAuto = systray.AddMenuItem("Pause Automation", "Toggle automated sign-in on/off")
	app.mOpenConfig = systray.AddMenuItem("Settings / Discord Token", "Open configuration folder to edit token")
	app.mAutostart = systray.AddMenuItemCheckbox("Start with Windows", "Automatically launch on system startup", autostart.IsEnabled())

	systray.AddSeparator()
	app.mUninstall = systray.AddMenuItem("Uninstall", "Remove autostart and delete app data")
	app.mQuit = systray.AddMenuItem(fmt.Sprintf("Exit (%s)", config.AppVersion), "Exit application")

	app.RefreshUI()

	go app.eventLoop()
	go app.schedulerLoop()

	go app.triggerAutomation(false)
}

func (app *TrayApp) onExit() {
}

func (app *TrayApp) RefreshUI() {
	st, err := state.LoadState()
	if err != nil {
		st = state.StateData{State: state.StateIdle}
	}

	app.mu.Lock()
	defer app.mu.Unlock()

	systray.SetIcon(GenerateIconICO(st.State))

	statusText := fmt.Sprintf("Status: [%s] %s", st.State, st.Message)
	app.mStatus.SetTitle(statusText)

	daysText := fmt.Sprintf("Signed In: %d days (Last: %s)", st.TotalSuccessDays, st.LastSuccessDate)
	app.mDays.SetTitle(daysText)

	if st.Stopped {
		app.mToggleAuto.SetTitle("Resume Automation")
	} else {
		app.mToggleAuto.SetTitle("Pause Automation")
	}

	if autostart.IsEnabled() {
		app.mAutostart.Check()
	} else {
		app.mAutostart.Uncheck()
	}
}

func (app *TrayApp) triggerAutomation(force bool) {
	app.mu.Lock()
	if app.isExecuting {
		app.mu.Unlock()
		return
	}
	app.isExecuting = true
	app.mu.Unlock()

	defer func() {
		app.mu.Lock()
		app.isExecuting = false
		app.mu.Unlock()
		app.RefreshUI()
	}()

	systray.SetIcon(GenerateIconICO(state.StateRunning))
	st, err := app.runner.ExecuteSigninCycle(force)
	if err != nil {
		st.State = state.StateError3
		st.Message = fmt.Sprintf("Operational failure: %v", err)
		_ = state.SaveState(st)
	}
}

func (app *TrayApp) eventLoop() {
	for {
		select {
		case <-app.mCheckNow.ClickedCh:
			app.mu.Lock()
			busy := app.isExecuting
			app.mu.Unlock()
			if !busy {
				go app.triggerAutomation(true)
			}

		case <-app.mToggleAuto.ClickedCh:
			st, err := state.LoadState()
			if err == nil {
				st.Stopped = !st.Stopped
				if st.Stopped {
					st.State = state.StateIdle
					st.Message = "Automation paused by user"
				} else {
					st.Message = "Automation resumed"
				}
				_ = state.SaveState(st)
			}
			app.RefreshUI()

		case <-app.mOpenConfig.ClickedCh:
			dir, err := config.GetAppDataDir()
			if err == nil {
				_ = os.MkdirAll(dir, 0755)
				_ = exec.Command("explorer", dir).Start()
			}

		case <-app.mAutostart.ClickedCh:
			if autostart.IsEnabled() {
				_ = autostart.Disable()
			} else {
				_ = autostart.Enable()
			}
			app.RefreshUI()

		case <-app.mUninstall.ClickedCh:
			_ = autostart.Disable()
			dir, err := config.GetAppDataDir()
			if err == nil {
				_ = os.RemoveAll(dir)
			}
			systray.Quit()
			return

		case <-app.mQuit.ClickedCh:
			systray.Quit()
			return
		}
	}
}

func (app *TrayApp) schedulerLoop() {
	getInterval := func() time.Duration {
		cfg, err := config.LoadConfig()
		if err == nil && cfg.CheckIntervalHr > 0 {
			return time.Duration(cfg.CheckIntervalHr) * time.Hour
		}
		return 30 * time.Minute
	}

	interval := getInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	resetTimer := time.NewTimer(time.Until(state.NextResetTimeUTC8(time.Now())))
	defer resetTimer.Stop()

	for {
		select {
		case <-resetTimer.C:
			st, err := state.LoadState()
			if err == nil && !st.Stopped {
				app.mu.Lock()
				busy := app.isExecuting
				app.mu.Unlock()
				if !busy {
					go app.triggerAutomation(false)
				}
			}
			resetTimer.Reset(time.Until(state.NextResetTimeUTC8(time.Now())))

		case <-ticker.C:
			st, err := state.LoadState()
			if err == nil && !st.Stopped {
				now := time.Now()
				if !state.IsAlreadySignedInToday(st, now) {
					app.mu.Lock()
					busy := app.isExecuting
					app.mu.Unlock()
					if !busy {
						go app.triggerAutomation(false)
					}
				}
			}

			newInterval := getInterval()
			if newInterval != interval {
				interval = newInterval
				ticker.Reset(interval)
			}
		}
	}
}
