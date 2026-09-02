package automation

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"auto-wuwa-discord-signin/pkg/config"
	"auto-wuwa-discord-signin/pkg/discord"
	"auto-wuwa-discord-signin/pkg/state"
)

type Runner struct {
	cfg ConfigProvider
}

type ConfigProvider interface {
	GetConfig() (config.Config, error)
}

type DefaultConfigProvider struct{}

func (d DefaultConfigProvider) GetConfig() (config.Config, error) {
	return config.LoadConfig()
}

func NewRunner() *Runner {
	return &Runner{
		cfg: DefaultConfigProvider{},
	}
}

func NewRunnerWithProvider(provider ConfigProvider) *Runner {
	return &Runner{
		cfg: provider,
	}
}

// ExecuteSigninCycle runs a single complete check and signin attempt.
func (r *Runner) ExecuteSigninCycle(force bool) (state.StateData, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	now := time.Now()
	currentState, err := state.LoadState()
	if err != nil {
		currentState = state.StateData{State: state.StateIdle}
	}

	if currentState.Stopped && !force {
		currentState.Message = "Automation is currently STOPPED"
		_ = state.SaveState(currentState)
		return currentState, nil
	}

	if !force && state.IsAlreadySignedInToday(currentState, now) {
		currentState.Message = fmt.Sprintf("Already signed in for today (%s)", currentState.LastSuccessDate)
		_ = state.SaveState(currentState)
		return currentState, nil
	}

	cfg, err := r.cfg.GetConfig()
	if err != nil {
		return currentState, fmt.Errorf("failed to read configuration: %w", err)
	}

	if strings.TrimSpace(cfg.DiscordToken) == "" {
		currentState.State = state.StateError3
		currentState.Message = "Discord Token not configured in config.json"
		currentState.LastCheckTime = state.FormatTimeUTC8(now)
		_ = state.SaveState(currentState)
		return currentState, nil
	}

	currentState.State = state.StateRunning
	currentState.Message = "Checking and performing sign-in..."
	currentState.LastCheckTime = state.FormatTimeUTC8(now)
	_ = state.SaveState(currentState)

	client := discord.NewClient(cfg.DiscordToken)

	// Step 1: Verify token auth (error3)
	validAuth, err := client.CheckAuth(ctx)
	if err != nil {
		currentState.State = state.StateError3
		currentState.Message = fmt.Sprintf("Discord connection error: %v", err)
		_ = state.SaveState(currentState)
		return currentState, nil
	}
	if !validAuth {
		currentState.State = state.StateError3
		currentState.Message = "Discord token is invalid or expired (error3)"
		_ = state.SaveState(currentState)
		return currentState, nil
	}

	// Step 2: Check server membership (error4)
	inGuild, err := client.CheckGuildMembership(ctx, cfg.GuildID)
	if err != nil {
		currentState.State = state.StateError4
		currentState.Message = fmt.Sprintf("Failed to check Wuthering Waves server: %v", err)
		_ = state.SaveState(currentState)
		return currentState, nil
	}
	if !inGuild {
		currentState.State = state.StateError4
		currentState.Message = "Account is not a member of Wuthering Waves server (error4)"
		_ = state.SaveState(currentState)
		return currentState, nil
	}

	// Step 3: Find sign-in button (error2)
	signinInfo, err := client.FindSigninButton(ctx, cfg.ChannelID)
	if err != nil {
		currentState.State = state.StateError2
		currentState.Message = fmt.Sprintf("Failed to scan channel messages: %v", err)
		_ = state.SaveState(currentState)
		return currentState, nil
	}
	if !signinInfo.Found {
		currentState.State = state.StateError2
		currentState.Message = "Sign-in event message or button not found (No Sign-in Event yet - error2)"
		_ = state.SaveState(currentState)
		return currentState, nil
	}

	// Step 4: Trigger interaction with Cooling-Down and 429 retry logic (error1, error0, success)
	const maxRetries = 5
	for attempt := 1; attempt <= maxRetries; attempt++ {
		res, err := client.PerformInteractionWithGateway(ctx, cfg.GuildID, cfg.ChannelID, signinInfo.MessageID, signinInfo.SignInCustomID, signinInfo.ApplicationID)
		if err != nil {
			currentState.State = state.StateRunning
			currentState.Message = fmt.Sprintf("Attempt %d failed: %v", attempt, err)
			time.Sleep(5 * time.Second)
			continue
		}

		if res.StatusCode == http.StatusTooManyRequests {
			waitDur := time.Duration(res.RateLimitWaitSec) * time.Second
			if waitDur < 5*time.Second {
				waitDur = 5 * time.Second
			}
			time.Sleep(waitDur)
			continue
		}

		body := res.EphemeralMessage
		if strings.Contains(body, "Please bind your game account first") || strings.Contains(body, "wutheringwaves-dc.kurogames-global.com") {
			currentState.State = state.StateError0
			currentState.Message = "Game account is unbound. Please bind at: https://wutheringwaves-dc.kurogames-global.com/ (error0)"
			currentState.LastCheckTime = state.FormatTimeUTC8(time.Now())
			_ = state.SaveState(currentState)
			return currentState, nil
		}

		if strings.Contains(body, "You must wait 30s between button presses") || strings.Contains(body, "Button Cooling down") {
			if attempt < maxRetries {
				time.Sleep(35 * time.Second)
				continue
			} else {
				currentState.State = state.StateError1
				currentState.Message = "Button cooling down exceeded 5 retries (error1)"
				currentState.LastCheckTime = state.FormatTimeUTC8(time.Now())
				_ = state.SaveState(currentState)
				return currentState, nil
			}
		}

		// Explicit verification of success confirmation strings
		if strings.Contains(body, "Sign-in Successful") ||
			strings.Contains(body, "You've already signed in today") ||
			strings.Contains(body, "All event rewards have been claimed") {
			today := state.GetLogicalDateUTC8(time.Now())
			if currentState.LastSuccessDate != today {
				currentState.TotalSuccessDays++
			}
			currentState.State = state.StateSuccess
			currentState.LastSuccessDate = today
			currentState.Message = "Sign-in successful for today!"
			currentState.LastCheckTime = state.FormatTimeUTC8(time.Now())
			_ = state.SaveState(currentState)
			return currentState, nil
		}

		// If response is empty or unrecognized, retry
		if attempt < maxRetries {
			time.Sleep(5 * time.Second)
			continue
		}
	}

	currentState.State = state.StateError1
	currentState.Message = "Did not receive a valid sign-in confirmation after 5 attempts (error1)"
	currentState.LastCheckTime = state.FormatTimeUTC8(time.Now())
	_ = state.SaveState(currentState)
	return currentState, nil
}
