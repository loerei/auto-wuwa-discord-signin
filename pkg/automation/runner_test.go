package automation

import (
	"testing"
	"time"

	"auto-wuwa-discord-signin/pkg/config"
	"auto-wuwa-discord-signin/pkg/state"
)

type MockConfigProvider struct {
	Token string
}

func (m MockConfigProvider) GetConfig() (config.Config, error) {
	cfg := config.DefaultConfig()
	cfg.DiscordToken = m.Token
	return cfg, nil
}

func TestRunnerNoToken(t *testing.T) {
	runner := &Runner{
		cfg: MockConfigProvider{Token: ""},
	}

	st, err := runner.ExecuteSigninCycle(true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if st.State != state.StateError3 {
		t.Errorf("expected state %s, got %s", state.StateError3, st.State)
	}
}

func TestAlreadySignedInSkip(t *testing.T) {
	now := time.Now()
	logicalDate := state.GetLogicalDateUTC8(now)

	st := state.StateData{
		State:           state.StateSuccess,
		LastSuccessDate: logicalDate,
	}
	_ = state.SaveState(st)

	runner := &Runner{
		cfg: MockConfigProvider{Token: "test_token"},
	}

	res, err := runner.ExecuteSigninCycle(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.State != state.StateSuccess {
		t.Errorf("expected state to remain %s, got %s", state.StateSuccess, res.State)
	}
}
