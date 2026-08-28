# Auto WuWa Discord Sign-In

Hi everyone.

I keep forgetting to open Discord every single morning just to click a single button for Wuthering Waves daily rewards. So I made a tiny app that just sits in the Windows tray and does it automatically.

### What it does

- Checks on boot and signs in whenever you turn on your PC (even if you boot up late in the evening), as long as today is not signed in yet.
- Runs quietly in your system tray without popping up console windows, taking almost no RAM.
- Retries on button cooldowns and stops automatically once today is signed in.
- Automatically sets itself to start with Windows so you can set it up once and forget about it.

### How to use

1. Download `wuwa-discord-signin.exe` from the latest Release.
2. Run it once, click the tray icon, and pick **Settings / Discord Token**.
3. Paste your Discord user token into the `config.json` file that opens up, save it, and you are done.

#### Getting your Discord token

- To grab your token easily, use the [discord-get-user-token](https://chromewebstore.google.com/detail/discord-get-user-token/accgjfooejbpdchkfpngkjjdekkcbnfd) Chrome extension (or follow any guide on Google if you prefer).
- **Important**: Logging out of your account on the web immediately invalidates that token. If you switch accounts often, log in from a spare browser profile, copy the token from there, and leave that browser session alone.

### A quick disclaimer

I capped retries to at most 5 times (waiting around 35 to 40 seconds each) per day, so if your Discord account has a normal, clean standing, you should be completely fine. But if your account has already been flagged or warned by Discord before, their systems will watch you a lot closer, and I take zero responsibility if your account catches another strike. Use at your own discretion.
