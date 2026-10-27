import { Plugin } from "@opencode/plugin"
import { readFile } from "node:fs/promises"
import { homedir } from "node:os"
import { join } from "node:path"

/**
 * Telegram → OpenCode steering bridge.
 *
 * The plugin owns a long-polling Telegram connection. When the configured
 * owner sends a message it:
 *   1. interrupts the session that most recently issued a model request;
 *   2. submits the Telegram message as a new steer prompt with a
 *      "continue where you stopped" instruction;
 *   3. sends the owner a short acknowledgement.
 *
 * The bot token is deliberately read from a config file outside the repository.
 * Set OPENCODE_TELEGRAM_BOT_TOKEN / OPENCODE_TELEGRAM_USER_ID to override it.
 *
 * Config file (default):
 *   ~/.config/opencode/telegram-steer.json
 *
 * {
 *   "token": "123456:...",
 *   "userID": 7045145880,
 *   "chatID": 7045145880,
 *   "pollTimeoutSeconds": 25
 * }
 */

type TelegramUpdate = {
  update_id: number
  message?: {
    message_id: number
    text?: string
    from?: { id: number }
    chat?: { id: number }
  }
}

type TelegramResponse<T> = {
  ok: boolean
  result?: T
  description?: string
}

type Config = {
  token: string
  userID: string
  chatID: string
  pollTimeoutSeconds: number
  enabled: boolean
  configPath: string
}

const DEFAULT_CONFIG_PATH = join(homedir(), ".config", "opencode", "telegram-steer.json")

export default Plugin.define({
  id: "telegram-steer",
  async setup(ctx) {
    const options = (ctx.options && typeof ctx.options === "object" ? ctx.options : {}) as Record<string, unknown>
    const config = await loadConfig(options)
    if (!config.enabled) {
      console.info("telegram-steer: disabled")
      return
    }
    if (!config.token || !config.userID) {
      console.warn(`telegram-steer: no token/userID (looked at ${config.configPath} and environment)`)
      return
    }

    let stopped = false
    let activeSessionID: string | undefined
    let activeSessionSeenAt = 0
    let offset = await loadOffset(ctx)
    const controller = new AbortController()
    const eventController = new AbortController()
    const lastMessage = new Map<string, string>()

    let hookRegistration: { dispose: () => Promise<void> } | undefined
    try {
      hookRegistration = await ctx.session.hook("context", (event: any) => {
        if (event && typeof event.sessionID === "string" && event.sessionID) {
          activeSessionID = event.sessionID
          activeSessionSeenAt = Date.now()
        }
      })
    } catch (error) {
      console.warn("telegram-steer: context hook unavailable, will use recent sessions")
    }

    const report = (error: unknown) => {
      const message = error instanceof Error ? error.message : String(error)
      console.warn("telegram-steer:", redact(message, config.token))
    }

    const handle = async (update: TelegramUpdate) => {
      const message = update.message
      if (!message || !message.text) return
      if (!message.from || String(message.from.id) !== config.userID) return
      if (config.chatID && message.chat && String(message.chat.id) !== config.chatID) return
      const text = message.text.trim()
      if (!text) return

      const sessionID = await resolveSession(ctx, activeSessionID, activeSessionSeenAt)
      if (!sessionID) {
        await sendTelegram(config, `Принял сообщение, но не нашёл активную сессию OpenCode.\n\n${preview(text)}`)
        return
      }

      await sendTelegram(
        config,
        `Принял: «${preview(text)}»\nСначала отвечу на новое сообщение, затем продолжу текущую задачу.`,
      )

      try {
        const result = await ctx.session.interrupt({ sessionID, resume: false })
        if (result && result.interrupted) {
          // Give the loop a moment to unwind before admitting the next prompt.
          await Promise.race([ctx.session.wait({ sessionID }), sleep(1500)])
        }
      } catch (error) {
        // A session that is already idle is fine: prompt admission still works.
        report(error)
      }

      const prompt = [
        "[Telegram, владелец]",
        text,
        "",
        "Сначала ответь на это новое сообщение из Telegram: коротко, по-русски, по существу. Ответ на него должен быть первым.",
        "После ответа сразу продолжай предыдущую задачу с того места, где остановился. Это новое указание владельца; не начинай задачу заново.",
      ].join("\n")

      let submitted = false
      for (let attempt = 0; attempt < 2 && !submitted; attempt++) {
        try {
          await ctx.session.prompt({ sessionID, text: prompt, delivery: "steer" })
          submitted = true
        } catch (error) {
          report(error)
          if (attempt === 0) await sleep(800)
        }
      }

      if (submitted) {
        await sendTelegram(config, "Передал в сессию. Сначала отвечу, затем продолжу с места остановки.")
      } else {
        await sendTelegram(config, "Не смог передать сообщение в сессию. Попробуй ещё раз.")
      }
    }

    const poll = async () => {
      while (!stopped) {
        try {
          const updates = await telegramCall<TelegramUpdate[]>(
            config,
            "getUpdates",
            {
              offset,
              timeout: config.pollTimeoutSeconds,
              allowed_updates: JSON.stringify(["message"]),
            },
            controller.signal,
            (config.pollTimeoutSeconds + 10) * 1000,
          )
          for (const update of updates || []) {
            const next = Number(update.update_id) + 1
            try {
              await handle(update)
            } catch (error) {
              report(error)
              // Do not advance past a message that failed to reach the session:
              // the next poll will retry it instead of silently losing it.
              break
            }
            offset = next
            await ctx.storage.set("telegram-offset", offset)
          }
        } catch (error) {
          if (stopped) break
          report(error)
          await sleep(2000)
        }
      }
    }

    const eventLoop = async () => {
      for await (const event of ctx.event.subscribe({
        signal: eventController.signal,
        types: ["session.idle", "session.error", "session.retry.scheduled"] as any,
      })) {
        const type = String((event as any)?.type || "")
        const sessionID = eventSessionID(event)
        if (!sessionID) continue
        // Mirror only the session that most recently talked to the model. This
        // keeps background/child sessions from spamming Telegram.
        if (activeSessionID && sessionID !== activeSessionID) continue

        if (type === "session.idle") {
          // The idle event is emitted after the assistant message is persisted.
          // Give the store a short moment, then mirror the final assistant text.
          await sleep(250)
          try {
            const messages = await ctx.session.context({ sessionID })
            const last = lastAssistantMessage(messages)
            if (!last || !last.text) continue
            if (lastMessage.get(sessionID) === last.id) continue
            lastMessage.set(sessionID, last.id)
            await sendTelegram(config, last.text)
          } catch (error) {
            report(error)
          }
          continue
        }

        if (type === "session.error") {
          const error = (event as any)?.error || (event as any)?.properties?.error || event
          const text = errorText(error)
          const delay = retryAfterMs((event as any)?.properties || event)
          await sendTelegram(config, `Ошибка OpenCode: ${text}${retryLine(delay)}`)
          continue
        }

        if (type === "session.retry.scheduled") {
          const delay = retryAfterMs((event as any)?.properties || event)
          if (delay) await sendTelegram(config, `Повтор запроса через ${formatDelay(delay)}.`)
        }
      }
    }
    void eventLoop().catch((error) => {
      if (!stopped) report(error)
    })

    void poll()
    console.info("telegram-steer: polling enabled")

    return async () => {
      stopped = true
      controller.abort()
      eventController.abort()
      if (hookRegistration) {
        await hookRegistration.dispose().catch(() => undefined)
      }
    }
  },
})

function eventSessionID(event: any): string {
  const value = event?.sessionID ?? event?.properties?.sessionID ?? event?.session?.id ?? event?.info?.sessionID
  return typeof value === "string" ? value : ""
}

function lastAssistantMessage(messages: any): { id: string; text: string } | undefined {
  const list = Array.isArray(messages) ? messages : messages?.messages || messages?.data || []
  for (let index = list.length - 1; index >= 0; index--) {
    const message = list[index]
    const role = message?.info?.role || message?.role
    if (role !== "assistant") continue
    const text = assistantText(message)
    if (!text) continue
    const id = String(message?.info?.id || message?.id || message?.messageID || index)
    return { id, text }
  }
  return undefined
}

function assistantText(message: any): string {
  const parts = message?.parts || message?.info?.parts || message?.content
  if (Array.isArray(parts)) {
    const text = parts
      .filter((part: any) => part?.type === "text" && typeof part.text === "string")
      .map((part: any) => part.text.trim())
      .filter(Boolean)
      .join("\n\n")
      .trim()
    if (text) return text
  }
  if (typeof message?.text === "string") return message.text.trim()
  if (typeof message?.content === "string") return message.content.trim()
  return ""
}

function errorText(error: any): string {
  const value =
    error?.message ||
    error?.data?.message ||
    error?.error?.message ||
    error?.properties?.message ||
    error?.type ||
    "неизвестная ошибка"
  const text = String(value).replace(/\s+/g, " ").trim()
  return text.length > 600 ? text.slice(0, 597) + "..." : text
}

function retryAfterMs(event: any): number {
  const value =
    event?.retryAfterMs ??
    event?.retry_after_ms ??
    event?.delayMs ??
    event?.properties?.retryAfterMs ??
    event?.properties?.retry_after_ms ??
    event?.properties?.delayMs ??
    event?.error?.retryAfterMs
  const parsed = Number(value)
  if (Number.isFinite(parsed) && parsed > 0) return parsed
  const seconds = Number(event?.retryAfter ?? event?.retry_after ?? event?.properties?.retryAfter)
  if (Number.isFinite(seconds) && seconds > 0) return seconds * 1000
  return 0
}

function retryLine(delay: number): string {
  return delay > 0 ? `\nПовтор через ${formatDelay(delay)}.` : ""
}

function formatDelay(ms: number): string {
  if (ms < 60_000) return `${Math.max(1, Math.round(ms / 1000))} сек`
  return `${Math.max(1, Math.round(ms / 60_000))} мин`
}

async function loadConfig(options: Record<string, unknown>): Promise<Config> {
  const configPath = stringValue(options.configFile) || DEFAULT_CONFIG_PATH
  let fileConfig: Record<string, unknown> = {}
  try {
    fileConfig = JSON.parse(await readFile(configPath, "utf8")) as Record<string, unknown>
  } catch (error) {
    const code = (error as { code?: string } | undefined)?.code
    if (code !== "ENOENT") {
      console.warn(`telegram-steer: cannot read ${configPath}: ${error instanceof Error ? error.message : error}`)
    }
  }

  return {
    token: stringValue(process.env.OPENCODE_TELEGRAM_BOT_TOKEN) || stringValue(fileConfig.token),
    userID: stringValue(process.env.OPENCODE_TELEGRAM_USER_ID) || stringValue(fileConfig.userID),
    chatID: stringValue(process.env.OPENCODE_TELEGRAM_CHAT_ID) || stringValue(fileConfig.chatID),
    pollTimeoutSeconds: numberValue(fileConfig.pollTimeoutSeconds, 25),
    enabled: fileConfig.enabled !== false,
    configPath,
  }
}

async function loadOffset(ctx: any): Promise<number> {
  try {
    const stored = await ctx.storage.get("telegram-offset")
    const value = typeof stored === "number" ? stored : Number(stored)
    return Number.isFinite(value) && value > 0 ? value : 0
  } catch {
    return 0
  }
}

async function resolveSession(ctx: any, activeSessionID: string | undefined, seenAt: number): Promise<string | undefined> {
  if (activeSessionID && Date.now() - seenAt < 2 * 60 * 60 * 1000) return activeSessionID
  let lastKnown: string | undefined
  try {
    const page = await ctx.session.list({ directory: ctx.location?.directory, limit: 50 })
    const items = Array.isArray(page) ? page : page?.items || page?.sessions || page?.data || []
    // A session list is not guaranteed to be ordered. Pick the session with
    // the newest update time so an idle session still receives the new prompt.
    const ordered = items
      .filter((item: any) => {
        const id = item?.id || item?.sessionID || item?.session?.id
        return typeof id === "string" && id && item?.archived !== true
      })
      .sort((a: any, b: any) => sessionUpdatedAt(b) - sessionUpdatedAt(a))
    const newest = ordered[0]
    lastKnown = newest?.id || newest?.sessionID || newest?.session?.id
    if (typeof lastKnown === "string" && lastKnown) return lastKnown
  } catch (error) {
    console.warn("telegram-steer: cannot list sessions:", error instanceof Error ? error.message : error)
  }
  // Listing can fail on a fresh service. Keep a previously observed session
  // rather than creating a second one, because it is still the latest active.
  if (activeSessionID) return activeSessionID
  try {
    const created = await ctx.session.create({ title: "Telegram" })
    return created?.id || created?.sessionID
  } catch (error) {
    console.warn("telegram-steer: cannot create session:", error instanceof Error ? error.message : error)
    return undefined
  }
}

function sessionUpdatedAt(session: any): number {
  const value =
    session?.time?.updated ??
    session?.time?.created ??
    session?.updatedAt ??
    session?.updated_at ??
    session?.time
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 0
}

async function sendTelegram(config: Config, text: string): Promise<void> {
  try {
    const chunks = splitMessage(text.trim(), 3900)
    for (const chunk of chunks) {
      await telegramCall(config, "sendMessage", {
        chat_id: config.chatID || config.userID,
        text: chunk,
        disable_web_page_preview: true,
      })
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error)
    console.warn("telegram-steer: send failed:", redact(message, config.token))
  }
}

function splitMessage(text: string, limit: number): string[] {
  if (!text) return []
  if (text.length <= limit) return [text]
  const chunks: string[] = []
  let rest = text
  while (rest.length > limit) {
    let cut = rest.lastIndexOf("\n", limit)
    if (cut < limit / 2) cut = rest.lastIndexOf(" ", limit)
    if (cut < limit / 2) cut = limit
    chunks.push(rest.slice(0, cut).trim())
    rest = rest.slice(cut).trim()
  }
  if (rest) chunks.push(rest)
  return chunks
}

async function telegramCall<T>(
  config: Config,
  method: string,
  fields: Record<string, unknown>,
  signal?: AbortSignal,
  timeoutMs = 30_000,
): Promise<T> {
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), timeoutMs)
  const abort = () => controller.abort()
  if (signal) {
    if (signal.aborted) controller.abort()
    else signal.addEventListener("abort", abort, { once: true })
  }
  try {
    const response = await fetch(`https://api.telegram.org/bot${config.token}/${method}`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(fields),
      signal: controller.signal,
    })
    const body = (await response.json()) as TelegramResponse<T>
    if (!response.ok || !body.ok) {
      throw new Error(body.description || `Telegram HTTP ${response.status}`)
    }
    return body.result as T
  } finally {
    clearTimeout(timeout)
    if (signal) signal.removeEventListener("abort", abort)
  }
}

function redact(value: string, token: string): string {
  return token ? value.split(token).join("***") : value
}

function stringValue(value: unknown): string {
  if (typeof value === "string") return value.trim()
  if (typeof value === "number" && Number.isFinite(value)) return String(value)
  return ""
}

function numberValue(value: unknown, fallback: number): number {
  const parsed = typeof value === "number" ? value : Number(value)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback
}

function preview(text: string): string {
  const oneLine = text.replace(/\s+/g, " ").trim()
  return oneLine.length > 120 ? oneLine.slice(0, 117) + "..." : oneLine
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}
