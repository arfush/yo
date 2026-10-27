# Архитектура yo

Документ фиксирует целевую структуру пакетов и направления зависимостей.
Правила, которые уже можно проверить, закреплены тестом
`tools/architecture/architecture_test.go`; он входит в обычный `go test ./...`.

## Принципы

- **Фича важнее слоя.** Пакет называется по задаче (`library`, `playback`,
  `party`), а не по техническому слою (`domain`, `infrastructure`).
- **`internal/` не используется.** Внутренние пакеты и так недоступны снаружи
  модуля, а лишний уровень в пути только удлиняет импорты. Ограничения
  задаются правилами зависимостей, а не расположением каталога.
- **`cmd/` — только точки входа.** В `cmd` нет логики, которую можно
  переиспользовать или протестировать отдельно.
- **`app` — композиционный корень.** Он знает про все сервисы и экраны,
  собирает их и передаёт экранам данные и команды. Экраны не создают сервисы
  сами.
- **Пакет-фича не знает про UI.** `library`, `playback`, `storage` и остальные
  возвращают данные и ошибки; показывать их — задача `ui`.
- **`platform` — только механизм ОС.** Пути `%APPDATA%`, реестр, мьютекс
  одного экземпляра, трей и Win32-заголовок окна. Никакой продуктовой
  политики.
- **Внедрение называется по роли, как `tls.Config`.** Экран получает
  `title.Config`, `settings.Config` и т. п. — структуру с зависимостями и
  командами именно этого экрана. Не заводи общие `Actions` или `Callbacks`:
  по такому имени нельзя понять, что оно делает и кто его владелец.

## Целевое дерево

```text
cmd/
  yo/                 единственная точка входа: app.Run
app/                  композиционный корень: пути, сервисы, single instance, запуск ui
  app.go              запуск процесса, сборка ui.Services, именованные логгеры
  ratelimit.go        адаптер состояния лимита AnimeLIB для заголовка
catalog/              чистые типы: Title, Voice, Status, Segment, ReadingChapter
  errors.go           общие сентинелы (ErrRateLimited, ErrTokenMissing и т. п.)
libsocial/             API AnimeLIB и LibSocial: клиент, episodes, reading, rate limit
  reading.go          ссылки и главы MangaLib/SlashLIB/RanobeLib/HentaiLib
  errors.go           сентинелы API-клиента
browser/              Playwright: Chromium, авторизация, headless Kodik/Filmlib
kodik/                резолв Kodik embed -> HLS
filmlib/              второй источник: метаданные, каталог Alloha, резолв потоков
library/              загрузка тайтла, выбор озвучки/главы, обложки, reader-мост
  reading.go          чтение глав и страниц, source-роутинг
metadata/             Shikimori/MAL, AniList, IMDb, introdb
mpv/                  поиск и установка mpv
playback/             сессия mpv, очередь, прогресс, главы, автопропуск
party/                протокол совместного просмотра, часы, конфигурация реле
  deploy/             Cloudflare Worker и Supabase-реле
  party.example.json  образец пользовательского party.json
storage/               SQLite: схема, история, настройки, кэши
  db.go                открытие базы, проверка схемы, Close
  queries.go           go:embed SQL и загрузчик именованных запросов
  sql/                 schema.sql и запросы по доменам
  errors.go            ErrCorruptDatabase: старая/битая база не поддерживается
token/                файл токена AnimeLIB и проверка срока
update/               всё об обновлении: GitHub Releases, SHA-256, замена exe
log/                  журнал на log/slog: файл, ротация, кольцевой буфер, source
version/              версия по умолчанию и форматирование
platform/
  appdata/           пути в %APPDATA%\yoanime
  install/           проверка установки через реестр Windows
  singleinstance/    один запущенный экземпляр на пользователя
  tray/              значок и меню в системном трее
  window/            Win32: кастомный заголовок, DWM, DPI
ui/
  assets/            шрифты, изображения и встроенные IconVG-иконки
  draw/              низкоуровневые примитивы рисования
  theme/             палитра и hover-анимация
  widgets/           переиспользуемые кнопки, панели, пилюли, прогресс
  errors.go          UI-ошибки с русским текстом и технической причиной
  app.go, shell.go,  контроллер приложения, каркас окна, секции библиотеки
  sections.go        yoanime/yomanga/yoranobe: настройка, цвета, фильтр истории
  main_screen.go,    экраны: главный, тайтл, ридер, настройки, журнал, F1, party
  title_screen.go,
  reader_screen.go,
  *_screen.go
docs/
installer/
third_party/
tools/
```

`ui` остаётся владельцем экранов и состояния между ними; в подпакеты
вынесены только слои, которые не знают про конкретный экран. `ui/widgets`
не импортирует корневой `ui`, а получает типографику через узкий интерфейс и
параметры стиля.

## Направления зависимостей

Стрелка `A -> B` означает «A может импортировать B».

```text
cmd -> app
app -> ui, все фичи, platform, log, version, catalog
ui  -> catalog, log, version, party, platform (экраны)
фичи -> catalog, log, version, token, platform
platform -> catalog, log, version
log, version, catalog -> только стандартная библиотека (log использует log/slog)
token -> catalog и стандартная библиотека
```

Сейчас в `ui` ещё остаётся переходный контроллер (`app.go`, `party.go`,
`services.go`), поэтому он временно импортирует фичи и платформенные пути.
Новые экраны уже не должны этого делать: они получают зависимости через
`Config`/`Services`, а не создают клиентов и не читают `%APPDATA%`.

Запрещено:

- импортировать `cmd/...` — это точки входа;
- импортировать `ui` из чего угодно, кроме `app` и `cmd`;
- импортировать `app` из `ui` и фич;
- импортировать `storage` из `ui`: экран получает данные через `app`;
- импортировать фичи (`library`, `playback`, `party`, ...) из `platform`;
- замыкать импорты в цикл.

Тест проверяет эти правила для production-файлов; тестовые файлы в граф не
входят, чтобы тест одного пакета мог свободно собирать чужие пакеты.

## Почему так

### `updater` и `updates` — один пакет `update`

Всё про обновление живёт в одном модуле: `update/` проверяет релизы на
GitHub, скачивает и проверяет SHA-256, а затем заменяет запущенный exe через
скрытый `update-helper.cmd`. Разделять эти две части по пакетам было неудобно:
`ui` и так импортировал их обе, а общая политика (цепочка проверок, временные
файлы, ldflags `Repository`) у них одна. `platform` остаётся тонкими обёртками
над ОС без продуктовой политики — замена exe продуктовая, поэтому она не во
`platform`, а рядом с остальным ходом обновления.

### Логирование: `log` на `log/slog` с именованными логгерами

`log/` не угадывает источник записи по стеку: каждый модуль собирает свой
логгер константой из `log` (`var logger = log.For(log.SourceLibrary)`) и
пишет через него. Атрибут `source` попадает во все записи и в кольцевой
буфер окна журнала. Пакет отвечает только за файл, ротацию, редактирование
секретов и окно журнала; формат — стандартные `log/slog`-вызовы.

### `token` — это файл токена AnimeLIB

Пакет читает и пишет `%APPDATA%\yoanime\libsocial.token`: JSON-объект
OAuth-токена (как в localStorage сайта), запечатанный пользовательским Windows
DPAPI, плюс запасной разбор простой строки. Старый открытый файл читается и
переписывается в DPAPI-контейнер при первом `Load`. Пакет проверяет срок с
запасом `ExpiryMargin` в 2 минуты. Имя файла и энтропия DPAPI относятся к
LibSocial; старый `animelib_token.txt` намеренно не мигрируется.

Пакет нужен двум независимым сторонам: `browser` сохраняет токен после входа,
`library` читает его для API. Если положить токен в `browser`, то `library`
начнёт зависеть от Playwright; если в `libsocial`, то `browser` начнёт зависеть
от API-клиента и его rate limiter. Отдельный маленький пакет — самый дешёвый
способ разорвать эту связь.

### `version` и `update`

`version` — форматирование и версия по умолчанию, без сети. `update` —
проверка GitHub Releases, скачивание и замена exe, то есть фича. Не смешивай
их: `platform/install` и `ui` могут знать про `version`, но не про `update`.

### Секции библиотеки и ридер

Приложение показывает три библиотеки: `yoanime` (аниме), `yomanga`
(MangaLib/SlashLIB/HentaiLib) и `yoranobe` (RanobeLib). Секция хранится в
настройках (`library_section`), переключается меню в шапке и фильтрует
историю на главном экране. Манга и ранобэ открываются не страницей тайтла, а
отдельным ридером (`ui/reader_screen.go`), который ходит в `library` за
главами и страницами; mpv в чтении не участвует.

## Совместимость имён

Приложение переименовано в yo. Старые версии не поддерживаются, поэтому
данные не мигрируются: прежний каталог `%APPDATA%\yonime` и `watchparty.json`
игнорируются, а новый файл конфигурации реле лежит в
`%APPDATA%\yoanime\party.json`. Настройки `party_provider` и `party_name` в
SQLite сохранили прежние имена.

Переменные сборки переименованы в `YO_CLOUDFLARE_ENDPOINT`,
`YO_SUPABASE_ENDPOINT` и `YO_SUPABASE_PUBKEY`; одноимённые
repository variables нужно обновить. Вшитые значения по умолчанию лежат в
корневом `party.json`.

Остальные имена переведены в пространство `party`: адрес комнаты
`sha256("yoanime/party/room\x00" + ключ)`, метка карточки `yoanime/party/card` и
тема Supabase `realtime:party:`. Это протокол `3.0`: комната, открытая старой
версией, новому клиенту не видна, поэтому обновлять нужно обе стороны. Адрес
развёрнутого реле при этом не меняется.

## Статус переезда

- [x] `internal/watchparty` -> `party/`
- [x] `internal/logging` -> `log/` (теперь на `log/slog` с именованными source)
- [x] `internal/appdata` -> `platform/appdata`
- [x] `internal/install` -> `platform/install`
- [x] `internal/singleinstance` -> `platform/singleinstance`
- [x] `ui/tray_*` -> `platform/tray`
- [x] Win32-код из `ui/platform_*` -> `platform/window`
- [x] остальные `internal/*` -> корень
- [x] `catalog/` и `app/` как отдельные пакеты
- [x] модуль и репозиторий переименованы: `github.com/arfush/yo`
- [x] экраны `*_window` -> `*_screen` (main/title/reader/settings/log/…)
- [x] `updates/` + `updater/` -> единый пакет `update/`
- [x] `storage`: старая схема без миграций; ad-hoc ALTER-миграции удалены,
      `prepareDatabase` штампует `PRAGMA user_version = 2` и нормализует
      известные старые source/slug напрямую; неполные базы открываются как
      повреждённые (`ErrCorruptDatabase`). Отдельный файл миграций сейчас не
      создаётся.
- [x] сентинелы ошибок собраны в `errors.go` каждого модуля
- [x] `animelib` -> `libsocial`, `store` -> `storage`; SQL вынесен в
      `storage/sql/*.sql` и подключён через `go:embed`; `titles.source`
      нормализуется в `libsocial` при открытии базы
- [x] `libsocial.WebOrigin` — единственный сайтовый origin для referer
      (`anilib.me`), API остаётся на `api.cdnlibs.org`
- [x] LibSocial: `libsocial/reading.go`, `library/reading.go`, секции
      yoanime/yomanga/yoranobe, `ui/reader_screen.go`
- [ ] `ui` разложен на `kit`, `controls`, `chrome`, `screen`
- [ ] контроллер окна и комнаты вынесен из `ui` в `app`; до этого
  `ui/app.go`, `ui/party.go` и `ui/services.go` временно импортируют фичи
