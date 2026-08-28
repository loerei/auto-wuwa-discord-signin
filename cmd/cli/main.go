package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"auto-wuwa-discord-signin/pkg/automation"
	"auto-wuwa-discord-signin/pkg/config"
	"auto-wuwa-discord-signin/pkg/state"
)

type CLIConfigProvider struct {
	token     string
	guildID   string
	channelID string
}

func (c CLIConfigProvider) GetConfig() (config.Config, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		cfg = config.DefaultConfig()
	}
	if c.token != "" {
		cfg.DiscordToken = c.token
	}
	if c.guildID != "" {
		cfg.GuildID = c.guildID
	}
	if c.channelID != "" {
		cfg.ChannelID = c.channelID
	}
	return cfg, nil
}

func main() {
	var tokenFlag = flag.String("token", "", "Discord User Token (leave empty to use config.json)")
	var guildFlag = flag.String("guild", "", "Wuthering Waves Guild ID (leave empty to use config.json)")
	var channelFlag = flag.String("channel", "", "Sign-in Channel ID (leave empty to use config.json)")
	var saveFlag = flag.Bool("save", false, "Persist provided flags into %APPDATA% config.json")
	flag.Parse()

	fmt.Println("==========================================================")
	fmt.Println("  AUTO WUTHERING WAVES DISCORD SIGN-IN (DIAGNOSTIC CLI)")
	fmt.Println("==========================================================")

	baseCfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("⚠️ Warning: Failed to load config: %v\n", err)
		baseCfg = config.DefaultConfig()
	}

	activeToken := *tokenFlag
	if strings.TrimSpace(activeToken) == "" {
		activeToken = os.Getenv("DISCORD_TOKEN")
		if strings.TrimSpace(activeToken) == "" {
			activeToken = baseCfg.DiscordToken
		}
	}

	activeGuild := *guildFlag
	if strings.TrimSpace(activeGuild) == "" {
		activeGuild = baseCfg.GuildID
	}

	activeChannel := *channelFlag
	if strings.TrimSpace(activeChannel) == "" {
		activeChannel = baseCfg.ChannelID
	}

	if *saveFlag {
		baseCfg.DiscordToken = activeToken
		baseCfg.GuildID = activeGuild
		baseCfg.ChannelID = activeChannel
		if err := config.SaveConfig(baseCfg); err != nil {
			fmt.Printf("⚠️ Warning: Failed to save config: %v\n", err)
		} else {
			fmt.Println("💾 Configuration saved to disk.")
		}
	}

	if strings.TrimSpace(activeToken) == "" {
		fmt.Println("❌ ERROR: Discord token is missing.")
		fmt.Println("👉 Please specify --token <token> or set it in %APPDATA%\\WuWaDiscordAuto\\config.json")
		os.Exit(1)
	}

	provider := CLIConfigProvider{
		token:     activeToken,
		guildID:   activeGuild,
		channelID: activeChannel,
	}

	runner := automation.NewRunnerWithProvider(provider)

	fmt.Println("\n>>> Triggering automated sign-in check...")
	st, err := runner.ExecuteSigninCycle(true)
	if err != nil {
		fmt.Printf("❌ EXECUTION ERROR: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n--- Final Status: [%s] ---\n", st.State)
	fmt.Printf("Message: %s\n", st.Message)
	fmt.Printf("Last Check Time (UTC+8): %s\n", st.LastCheckTime)
	fmt.Printf("Total Successful Days: %d\n", st.TotalSuccessDays)

	if st.State == state.StateSuccess {
		fmt.Println("🎉 SUCCESS!")
	} else if st.State == state.StateError0 {
		fmt.Println("⚠️ Game account unbound. Please bind at: https://wutheringwaves-dc.kurogames-global.com/")
	} else {
		fmt.Printf("❌ Execution ended with state: %s\n", st.State)
	}
}
