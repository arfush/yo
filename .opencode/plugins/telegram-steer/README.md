# telegram-steer

OpenCode server plugin that bridges the owner's Telegram chat with the current
session.

## What it does

- long-polls Telegram (`getUpdates`) for messages from one configured user;
- interrupts the session that most recently issued a model request; if no session
  is running, picks the most recently updated session instead of failing;
- sends the Telegram text as a new prompt with an explicit "answer this message
  first, then continue" preamble;
- mirrors final assistant messages back to Telegram on `session.idle`;
- forwards `session.error` and `session.retry.scheduled` with the retry delay.

It does not mirror intermediate tool/progress events.

## Configuration

The plugin reads its token outside the repository. The default file is:

```text
~/.config/opencode/telegram-steer.json
```

```json
{
  "token": "123456:telegram-bot-token",
  "userID": 7045145880,
  "chatID": 7045145880,
  "pollTimeoutSeconds": 25
}
```

Environment overrides:

- `OPENCODE_TELEGRAM_BOT_TOKEN`
- `OPENCODE_TELEGRAM_USER_ID`
- `OPENCODE_TELEGRAM_CHAT_ID`

`configFile` can be passed through plugin options if the file lives elsewhere.
Never put the token into `opencode.json` or this repository.

## Install / reload

The plugin is auto-discovered from `.opencode/plugins/telegram-steer/` when an
OpenCode session is created for this project. Restart the OpenCode service or
start a new session after adding it.

Only one process may poll a Telegram bot token at a time. Stop the standalone
`yo_tg_bot` before enabling this plugin, otherwise the two pollers will split
the updates between them.
