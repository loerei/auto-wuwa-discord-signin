package discord

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	DiscordAPIBase = "https://discord.com/api/v9"
	UserAgent      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) discord/1.0.9168 Chrome/128.0.6613.186 Electron/32.2.7 Safari/537.36"
)

type Client struct {
	Token      string
	HTTPClient *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		Token: token,
		HTTPClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func getSuperProperties() string {
	sp := map[string]interface{}{
		"os":                  "Windows",
		"browser":             "Discord Client",
		"release_channel":     "stable",
		"client_version":      "1.0.9168",
		"os_version":          "10.0.22631",
		"os_arch":             "x64",
		"app_arch":            "x64",
		"system_locale":       "en-US",
		"client_build_number": 335000,
	}
	b, _ := json.Marshal(sp)
	return base64.StdEncoding.EncodeToString(b)
}

func (c *Client) addHeaders(req *http.Request) {
	req.Header.Set("Authorization", c.Token)
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Origin", "https://discord.com")
	req.Header.Set("X-Super-Properties", getSuperProperties())
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
}

// CheckAuth verifies the token (error3).
func (c *Client) CheckAuth(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", DiscordAPIBase+"/users/@me", nil)
	if err != nil {
		return false, err
	}
	c.addHeaders(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("network error during auth check: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return false, nil
	}
	return false, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
}

type Guild struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CheckGuildMembership checks if user is in target guild (error4).
func (c *Client) CheckGuildMembership(ctx context.Context, guildID string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", DiscordAPIBase+"/users/@me/guilds", nil)
	if err != nil {
		return false, err
	}
	c.addHeaders(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("network error during guilds check: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("failed to fetch guilds, status: %d", resp.StatusCode)
	}

	var guilds []Guild
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&guilds); err != nil {
		return false, fmt.Errorf("failed to parse guilds: %w", err)
	}

	for _, g := range guilds {
		if g.ID == guildID {
			return true, nil
		}
	}
	return false, nil
}

type SigninMessageInfo struct {
	MessageID      string
	SignInCustomID string
	ApplicationID  string
	Found          bool
}

// FindSigninButton scans channel messages for Sign-in button.
func (c *Client) FindSigninButton(ctx context.Context, channelID string) (SigninMessageInfo, error) {
	url := fmt.Sprintf("%s/channels/%s/messages?limit=25", DiscordAPIBase, channelID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return SigninMessageInfo{}, err
	}
	c.addHeaders(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return SigninMessageInfo{}, fmt.Errorf("failed to fetch channel messages: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return SigninMessageInfo{}, fmt.Errorf("fetch messages returned status: %d", resp.StatusCode)
	}

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var messages []map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &messages); err != nil {
		return SigninMessageInfo{}, fmt.Errorf("failed to parse messages: %w", err)
	}

	appID := "1353617365648281630"
	for _, msg := range messages {
		content, _ := msg["content"].(string)
		if strings.Contains(content, "Discord Sign-In Event") || strings.Contains(content, "Sign-in") {
			if author, ok := msg["author"].(map[string]interface{}); ok {
				if aID, ok := author["id"].(string); ok {
					appID = aID
				}
			}
			if comps, ok := msg["components"].([]interface{}); ok {
				for _, cRow := range comps {
					if cMap, ok := cRow.(map[string]interface{}); ok {
						if subComps, ok := cMap["components"].([]interface{}); ok {
							for _, btn := range subComps {
								if btnMap, ok := btn.(map[string]interface{}); ok {
									label, _ := btnMap["label"].(string)
									cID, _ := btnMap["custom_id"].(string)
									if strings.EqualFold(label, "Sign-in") || cID == "sign_in" {
										mID, _ := msg["id"].(string)
										return SigninMessageInfo{
											MessageID:      mID,
											SignInCustomID: cID,
											ApplicationID:  appID,
											Found:          true,
										}, nil
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return SigninMessageInfo{Found: false}, nil
}

type InteractionResponseResult struct {
	StatusCode       int
	EphemeralMessage string
	RateLimitWaitSec int
}

// PerformInteractionWithGateway connects to Gateway, triggers button and intercepts Ephemeral response.
func (c *Client) PerformInteractionWithGateway(ctx context.Context, guildID, channelID, messageID, customID, appID string) (*InteractionResponseResult, error) {
	wsURL := "wss://gateway.discord.gg/?v=9&encoding=json"
	ws, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("gateway dial error: %w", err)
	}
	defer ws.Close()

	var wsWriteMu sync.Mutex
	safeWriteJSON := func(v interface{}) error {
		wsWriteMu.Lock()
		defer wsWriteMu.Unlock()
		_ = ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
		return ws.WriteJSON(v)
	}

	gatewayCtx, cancelGateway := context.WithCancel(ctx)
	defer func() {
		cancelGateway()
		_ = ws.Close()
	}()

	var sessionID string
	readyCh := make(chan bool, 1)
	ephemeralCh := make(chan string, 5)
	var heartbeatStarted int32

	go func() {
		defer cancelGateway()
		for {
			select {
			case <-gatewayCtx.Done():
				return
			default:
			}

			_ = ws.SetReadDeadline(time.Now().Add(60 * time.Second))
			_, message, err := ws.ReadMessage()
			if err != nil {
				return
			}
			var payload map[string]interface{}
			if err := json.Unmarshal(message, &payload); err == nil {
				op, _ := payload["op"].(float64)
				t, _ := payload["t"].(string)

				if op == 10 {
					d, ok := payload["d"].(map[string]interface{})
					if !ok {
						continue
					}
					interval, _ := d["heartbeat_interval"].(float64)
					if interval <= 0 {
						interval = 41250
					}

					if atomic.CompareAndSwapInt32(&heartbeatStarted, 0, 1) {
						go func(hbCtx context.Context, dur time.Duration) {
							ticker := time.NewTicker(dur)
							defer ticker.Stop()

							for {
								select {
								case <-hbCtx.Done():
									return
								case <-ticker.C:
									if err := safeWriteJSON(map[string]interface{}{"op": 1, "d": nil}); err != nil {
										return
									}
								}
							}
						}(gatewayCtx, time.Duration(interval)*time.Millisecond)
					}

					identify := map[string]interface{}{
						"op": 2,
						"d": map[string]interface{}{
							"token": c.Token,
							"properties": map[string]interface{}{
								"os":      "Windows",
								"browser": "Discord Client",
								"device":  "desktop",
							},
						},
					}
					_ = safeWriteJSON(identify)
				}

				if t == "READY" {
					if d, ok := payload["d"].(map[string]interface{}); ok {
						sessionID, _ = d["session_id"].(string)
						select {
						case readyCh <- true:
						default:
						}
					}
				}

				if t == "MESSAGE_CREATE" {
					if d, ok := payload["d"].(map[string]interface{}); ok {
						chID, _ := d["channel_id"].(string)
						flags, _ := d["flags"].(float64)
						content, _ := d["content"].(string)
						if chID == channelID && int(flags)&64 != 0 {
							select {
							case ephemeralCh <- content:
							default:
							}
						}
					}
				}
			}
		}
	}()

	select {
	case <-readyCh:
	case <-time.After(12 * time.Second):
		return nil, fmt.Errorf("gateway connect timed out")
	}

	nonce := fmt.Sprintf("%d", time.Now().UnixNano()/(int64(time.Millisecond))+int64(rand.Intn(1000)))
	payload := map[string]interface{}{
		"type":           3,
		"guild_id":       guildID,
		"channel_id":     channelID,
		"message_id":     messageID,
		"application_id": appID,
		"session_id":     sessionID,
		"data": map[string]interface{}{
			"component_type": 2,
			"custom_id":      customID,
		},
		"nonce": nonce,
	}

	bodyBytes, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(gatewayCtx, "POST", DiscordAPIBase+"/interactions", bytes.NewReader(bodyBytes))
	c.addHeaders(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("interaction request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfterStr := resp.Header.Get("Retry-After")
		waitSec, _ := strconv.Atoi(retryAfterStr)
		if waitSec <= 0 {
			waitSec = 5
		}
		return &InteractionResponseResult{
			StatusCode:       resp.StatusCode,
			RateLimitWaitSec: waitSec,
		}, nil
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, fmt.Errorf("interaction returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var ephemeralMsg string
	select {
	case ephemeralMsg = <-ephemeralCh:
	case <-time.After(8 * time.Second):
		ephemeralMsg = ""
	}

	return &InteractionResponseResult{
		StatusCode:       resp.StatusCode,
		EphemeralMessage: ephemeralMsg,
	}, nil
}
