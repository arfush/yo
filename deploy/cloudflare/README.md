# Совместный просмотр через Cloudflare

Реле — Worker + Durable Object. Одна комната = один Durable Object, которому
клиент отдаёт ключ вида `YOFBQOPGZ`. Сайт `*.workers.dev` работает на бесплатном
плане, отдельный домен не нужен.

## Что понадобится

- аккаунт Cloudflare (бесплатного плана достаточно);
- Node.js 18+ и `npm`;
- около двух минут.

## Деплой

```bash
cd deploy/cloudflare
npm install -g wrangler     # если wrangler ещё не установлен
wrangler login
wrangler deploy
```

`wrangler deploy` напечатает адрес вида:

```text
https://yonime-watchparty.<ваш-аккаунт>.workers.dev
```

Проверка, что реле живо:

```bash
curl https://yonime-watchparty.<ваш-аккаунт>.workers.dev/health
# ok
```

## Подключение клиента

Создайте файл `%APPDATA%\yonime\watchparty.json`:

```json
{
  "cloudflare_url": "https://yonime-watchparty.<ваш-аккаунт>.workers.dev"
}
```

`https://` писать можно: клиент сам переводит адрес в `wss://`. После
перезапуска Yonime в форме «Совместный просмотр» будет доступен провайдер
**Cloudflare**.

Если вы собираете свой exe, адрес можно вшить на этапе сборки:

```bash
go build -ldflags "-X github.com/arfush/yonime/internal/watchparty.CloudflareURL=https://yonime-watchparty.example.workers.dev" ./cmd/yonime
```

## Что внутри

| Файл             | Назначение                                        |
| ---------------- | ------------------------------------------------- |
| `src/worker.js`  | Worker: маршрут `/room` и класс `Room`            |
| `wrangler.toml`  | биндинг `ROOMS` и миграция Durable Object         |

Worker:

1. проверяет ключ (`YO` + 7 латинских букв, 9 символов всего);
2. выбирает Durable Object по ключу через `idFromName`;
3. передаёт апгрейд сокета объекту.

Durable Object:

- держит сокет создателя комнаты и множество гостей;
- запоминает последний `state` и отправляет его новому гостю;
- на `hello` от гостя просит создателя прислать свежий `state`, чтобы гость не
  ждал следующего heartbeat;
- при уходе создателя рассылает гостям `bye`.

## Ограничения бесплатного плана

- 100 000 запросов в день. Комната тратит один запрос на сокет, дальше это
  обычные сообщения WebSocket, а не новые запросы.
- 13 000 ГБ·с в день для Durable Objects: одна комната на 4 часа занимает
  несколько ГБ·с, так что запас огромный.
- Соединения к `workers.dev` из России иногда режутся провайдерами. Клиент
  переподключается сам, но если у вас постоянно рвётся — используйте Supabase.

## Обновление кода реле

```bash
cd deploy/cloudflare
wrangler deploy
```

Ключи комнат не меняются: они живут в клиенте, а не в реле.
