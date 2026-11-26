# Context — anytext-gallery

## Current state

Step 1 of the gallery: a grid of the pictures of one collection, previews and originals from the
installation's own endpoints, a viewer, and a search by name. The module owns no files and no
database (0.1.0, 22.11.2026).

## Decisions

- 22.11.2026: the pictures are **documents of the installation's collection**, not a copy. The old
  Perl gallery keeps its own folder and its own auth; the module reads the collection and shows the
  root folder only — the copies in `pub/` and the pictures in `trash/` are not the gallery.
- 22.11.2026: **no picture bytes through the module.** Previews come from `/api/thumb` and originals
  from `/api/file`, fetched with the account token the window puts on every request. That is the only
  way that works at the machine and through the relay without a second permit system.
- 22.11.2026: one module, two sections («Картинки», «Статьи»). The articles section is the memoirs of
  the old gallery; it is the next step and will read a collection of its own.
- 22.11.2026: descriptions stay for now in `memory/gallery/meta/<file>` (the old gallery reads them
  there) and are **copied** into the installation's standard place (`doc.save_description`, which
  writes `<file>/vision`) — «не переносить, а копировать», because both galleries live side by side
  for a while. When the old gallery is closed, the memory copies are cleaned.

## Lesson of 22.11.2026: the index is not the collection

The first version listed `rag.list` and showed 69 pictures of 200. The reason is a rule of the scan:
a picture without a description has no text, the scan skips it as `no_text`, and it never becomes a
document. The old gallery listed a directory, and that was right. `doc.list` is the same list with
the keys the picture endpoints want — files with their state and kind, not indexed documents.

The same scan result explains why the collection "loses" pictures after a reindex: nothing is lost,
they were never indexed. A gallery must never be built on the full-text index.

## Lesson of 22.11.2026 (second): the counter was honest, the grid was not

The grid showed only the first page: the "load more" sentinel sat outside the scrolling element and
the observer fired once, while the first page was still loading. Now the next page is asked for when
the scroll comes within a screen of the end, and a page that does not fill the window asks for the
next one itself. The counter says "120 из 200" while more remain — the number of pictures drawn and
the number in the collection are two numbers, and both belong on the screen.

## The order of the grid (22.11.2026)

By name the grid looked random: the gallery's files carry three different naming schemes
(`img_<time>.png`, `cf_crop_<what>_<time>.png`, `cat2.png`). The grid now shows the newest file first
(`mod_time`, which the installation gained in the same change) with a switch to "by name". An
installation older than that answers without times, and the module falls back to names.

The viewer shows a spinner while the picture travels, not a torn page: a missing `src` is drawn as a
broken image, and a broken image looks like a failure (22.11.2026).

## Step 2: the words about a picture (0.1.4, 22.11.2026)

- Two grids: the dense one the module started with, and cards the size and shape of the old gallery
  (280px, preview 200px high, name, date, description, «Изменить»). The choice is remembered.
- The editor keeps the old fields — description, tags, context, prompt, story — and keeps them where
  the old gallery keeps them: `memory/gallery/meta/<file>`.
- A saved description is **copied** into the installation (`doc.save_description` → `<file>/vision`),
  which makes it searchable and marks the picture as described for the vision job. «Не переносить, а
  копировать»: the old gallery keeps working until it is switched off.
- The one-time migration copied 120 descriptions of 151 records (31 skipped: their pictures are not in
  the collection root — renamed, moved to pub/ or trash), 0 failures. Search now finds pictures by
  what is written about them.

## The size of a card (22.11.2026)

The cards grid stretched its columns to the window, so a tile on a wide screen was wider than in the
old gallery and `cover` cut more of a square picture than there. The card is now exactly **281 × 199**
as in bmat.uk and the picture fills it (`cover`): the grid grows the number of columns, not the size
of a tile, so the crop is the same as the old gallery's and does not grow with the window. The owner's
rule: the size is what bmat had, and a picture fills its box.

## As bmat: the grid fills the window, and the card has the buttons (0.1.7)

- The grid no longer keeps a 1400 px page: columns of at least 280 px share the whole width, exactly
  as the dense grid does. The old gallery had no empty margins on the sides, and neither does this one.
- A card carries the old gallery's buttons: **Удалить** (takes the picture out of the gallery — the
  installation excludes the file from the index and keeps it on disk, so it is reversible),
  **Сделать публичной / Убрать из публичных** (copies into `pub/` beside the picture and answers the
  public address), **Изменить** (the meta editor). The public link is shown under the buttons.

## The proportion of a preview (0.1.8, 22.11.2026)

A fixed height made a column that shares the width into a wide strip: 396 × 199 on one screen, 328 × 199
on another — a different crop on every window. The preview now keeps the old gallery's proportion,
**281 : 199** (`aspect-ratio`), so the height follows the width: 326 × 231 measured at a 328 px card,
ratio 1.412 against the 281/199 = 1.412.

## The dense tile (0.1.9, 22.11.2026)

The dense tile is a square, and the picture inside it had no box of its own: a wide picture got its own
height and left the rest of the tile empty. The tile's box now fills the card and the picture covers it,
as in the cards grid — a tile is a picture, not a picture with margins. Measured: tile 161 × 161, image
161 × 161, `cover`.

## The description comes from the installation (0.2.0, 22.11.2026)

The gallery showed `memory/gallery/meta/<file>.description`, so a picture described again in AnyText
("Описать заново") kept its old words here. The installation is where a description lives — the window
writes it there, the vision job writes it there — so the card and the editor now read **the
installation's** text (`doc.list` answers it) and fall back to the memory record only when the
installation has none.

Saving still writes both: the installation (the truth, searchable) and memory-store (the old bmat
gallery, which is still alive). An edit made in AnyText therefore does not reach the old gallery — it is
being retired, and that is the direction of the dependency: gallery → installation, never back.

## Step 3: the articles (0.3.0–0.3.2, 22.11.2026)

- The «Статьи» tab lists the chapters of the `-articles` collection (the memoirs): 40 chapters, newest
  first, with the date from the name and the first paragraph as a line. A chapter opens as a page:
  markdown rendered by goldmark, the header lines (`**Дата:**`, `**Место:**`, `**Автор:**`) read the
  way the old Perl gallery read them.
- Every picture of a chapter is a reference the screen resolves to **the module's own viewer**: the
  markdown is rendered and its `/i/<file>` addresses are replaced with `data-gallery`, then the page
  loads the picture through `/api/file` and opens it in the viewer. bmat.uk is no longer in the way.
- A picture that appears in a chapter gets a **«Рассказ»** button; the picture → chapter index is built
  from the chapters' own texts (a cache for two minutes), so nobody fills the old `memoir` field.
- Two lessons: a container with a `display` rule of its own ignores `hidden` (`.cards[hidden]`), and a
  non-ISO date in `new Date()` is `Invalid Date` — the module now always answers an ISO date when it
  knows one.

## The articles read the file, and the list looks like bmat's (0.3.4, 22.11.2026)

- Articles are read with `doc.text {raw: true}`: the extracted text loses blank lines and rules, and a
  chapter rendered from it glues `---` to a paragraph.
- The table of contents follows the old gallery: a bordered entry with the title, the line
  «Автор: …» · date, and the first paragraph. The author comes from `**Автор:**` or, when the chapter
  says `**Записано:** Барон, …`, from the first word of that line.
- The duplication of pictures was not the renderer's: the chunker repeats two sentences between chunks,
  and `DocText` glued them as they came. Fixed in AnyText (docs/CONTEXT.md there).

## The heading colour of the old gallery (0.3.5, 22.11.2026)

The memoirs were pleasant to look at partly because of the heading colour: `#58a6ff` on the dark page,
with `h1` at 2em and `h2` at 1.4em. That colour is now the module's `--heading`: `#58a6ff` in the dark
theme and `#0969da` in the light one (the same pair as GitHub, where the palette comes from) — the old
blue on white is unreadable. The contents page got its own `h1` «Оглавление», and a chapter's `h1`/`h2`
follow the old sizes and margins.

## The article editor (0.4.1, 22.11.2026)

- A chapter page carries «Редактировать» next to «к статьям»: the dialog shows the markdown of the file
  (`doc.text {raw: true}`), not the extracted text, and saves it back with `doc.save_raw` of AnyText.
  The markdown stays markdown — a picture is named by its file, `![подпись](img_123.png)`.
- The right to save comes from the installation: it puts `X-AnyText-Rights` on every proxied request.
  A viewer does not see the button at all; a `POST /api/article` without the right is refused with 403.
- After a save the module forgets its parsed chapters (`articlesReader.Forget`), so the page is built
  from the file again instead of from the two-minute cache.
- The page's base address is the header or `/`: reached on its own port the module serves at the root,
  and a guessed `/api/module/gallery` prefix pointed the screen's relative calls at nothing.
- The empty text is refused by the installation (`doc.save_raw`): a chapter is not erased by clearing
  a field — that lesson cost a memoir chapter on the same day.
- Verified on a copy: a temporary collection in `/tmp` with a copy of a chapter, a second instance of
  the module on its own port, the edit saved and the page rebuilt; the real chapter's md5 unchanged.

## «Описать моделью» (0.4.2, 23.11.2026)

- The meta editor carries the model's button: it runs the installation's own `doc.describe` — the same
  command behind the window's «Описать моделью» — and puts the answer into the description field. It is a
  draft: the person reads it and presses «Сохранить», which is what writes memory and the installation's
  index. A model that answers nothing, or an installation without a vision model, is named in the error
  line instead of leaving the field empty.
- `POST /api/describe?doc=` needs the write right (a model call is not a read), a document, and waits
  150 s — the installation gives a model two minutes, and the module's client now waits three so a slow
  answer cannot look like a broken module.
- Checked on `baron-parrot.png`: 4 s, 508 characters, model `deepseek-v4.1-flash:cloud`; the field filled,
  «Отмена» left memory and the index untouched.

### 0.4.3 — the confirmation is one word (23.11.2026)

«Сохранено в установке (описание сразу в индексе). Копия — в память галереи, для старой bmat» ушло: человеку,
который нажал «Сохранить», нужен один ответ. Осталась строка «Сохранено», а разница между двумя местами
видна только когда что-то пошло не так — тогда «Сохранено в память, но не в установку: <причина>».

## «Связи» in the viewer (0.4.4–0.4.5, 23.11.2026)

- Кирилл: «в старой галерее не все картинки были описаны, и что-то работало; здесь в местах, где у старой
  были связи, нет ничего». Старая галерея показывала под картинкой блок «🔗 Связи» — рёбра графа памяти
  (`graph_get_edges` по имени файла, шаблоны на «иллюстрация», «заказал», «был_в»…). В модуле этого не было.
- Теперь есть: `GET /api/edges?name=` спрашивает memory-store через шлюз установки, ответ кэшируется на две
  минуты (`edgesCache`), блок рисуется под именем в просмотрщике. Спрашивается **по требованию** — по одной
  картинке, которую человек открыл, а не по всем карточкам сетки: у memory-store нет пакетного запроса, и
  шестьдесят вызовов на страницу были бы дороже, чем польза от счётчика на карточке.
- Формат: иконка по словарю графа, имя связи как в графе (подчёркивания → пробелы), стрелка показывает
  направление; имя другого конца — заголовочным цветом. Прежние шаблоны («📖 иллюстрация к Кирилл») давали
  ошибку в падеже, поэтому от готовых фраз отказались в пользу «📖 иллюстрация → Кирилл».
- Описания моделью остаются по требованию: пакетного прогона нет и не будет без просьбы.

### 0.4.6–0.4.7 — the old links on the cards (23.11.2026)

- Старая галерея показывала связность **на карточке**: кнопка «Связи» с числом рёбер (`graph_count`), которую она
  получала вызовом `graph_get_edges` на каждую картинку выдачи. Модуль теперь делает то же, но по требованию:
  связи спрашиваются вместе с превью, когда карточка подходит к экрану (`IntersectionObserver`), не больше
  четырёх запросов разом, ответ запоминается в браузере и в модуле на две минуты. На карточке — «🔗 N», при
  нажатии открывается просмотрщик с полным списком.
- Проверили совпадение со старой галереей: `http://127.0.0.1:8900/gallery/api/?action=graph_edges&file=cat2.png`
  и `/api/edges?name=cat2.png` отвечают одним и тем же JSON.
- Вторая «старая связь» — поле `memoir` в мете (19 записей от bmat: «Иллюстрация к главе»). Модуль искал главу
  только по тексту, поэтому две записи не показывались (`cat2.png` → «Глава 8», `img_1784999640.png` →
  «Граф памяти»). Теперь `memoir` — запасной путь: кнопка «Рассказ» есть, если глава с таким именем
  существует; несуществующая глава кнопки не даёт. Карточек с «Рассказ» стало 47 (45 по тексту + 2 по мете).

## Search: words first, meaning on request (0.5.0–0.5.3, 23.11.2026)

- Локальный поиск картинок смотрит теперь не только в имя: имя, папка, описание установки и теги
  memory-записи. Каждое слово запроса должно найтись — «верблюд балкон» находит описание, где есть оба
  (на живой коллекции: «попугай» → 1, «верблюд балкон» → 2, «хинкали» → 5, «мемуары» → 1 по тегам).
  Меты грузятся одним пакетным вызовом на всю коллекцию (`decorate`), поэтому фильтр по тегам ничего не
  стоит сверх уже загруженного.
- Статьи получили поиск: заголовок, автор, дата и весь текст главы; результат несёт `snippet` — окно
  вокруг находки, вычищенное от markdown (картинка → подпись, ссылка → слова, `**`, `#`, `---` убраны).
- Семантика — `rag.search` установки, по требованию: Enter в поле или пустой локальный результат.
  Замеры: 0.4–1.3 с, картинки 0.53 лучший score, главы 0.375 — абсолютного порога нет, поэтому обрезка
  относительная (`trimHits`: ≥0.82 от лучшего, максимум 24, минимум 5). Без обрезки установка отдавала
  всю коллекцию: 60 картинок и 40 глав на любой запрос.
- Одно поле поиска на обе вкладки, подсказка меняется; счётчик говорит «N · по смыслу», когда результат
  пришёл оттуда; «ещё» в таком режиме не догружается.

## Cache: whose cache is used, and what was not (0.5.4–0.5.6, 23.11.2026)

- Кирилл: «в AnyText есть кэш для картинок, но модуль им не пользуется, и превьюхи вообще не кешируются».
  Замеры: установка отдаёт превью за 1.1 мс (память + диск: 1028 файлов, 50 МБ в `cache/thumbs`), с
  `ETag` и `Cache-Control: private, max-age=86400`; браузер на повторной загрузке сетки получает
  `transferSize = 0` по всем превью. То есть кэш установки модуль использовал — **для сетки**.
- Что действительно не использовалось:
  1. **Картинки в главе и просмотрщик брали оригинал** (`/api/file`): открытие главы «День, когда я поехал
     в метро» тянуло 1.46 МБ + 1.53 МБ PNG и декодировало оба в полный размер. Теперь — превью 1400/1600 px
     из того же кэша: 137 КБ + 135 КБ, в 11 раз меньше, и декод мелкий.
  2. **Сетка забирала превью через `fetch` → blob → objectURL → revoke**: HTTP-кэш работал, но картинка
     каждый раз шла через JS и декодировалась заново. Теперь `<img src>` прямо на `/api/thumb`: кэш и
     декодер браузера, `loading=lazy`, `decoding=async`; blob-путь остался только как запасной при ошибке.
- Побочно: `installBase()` больше не оставляет хвостовой слэш и не знает имени модуля наизусть — адреса
  были вида `http://host//api/thumb`.
- Удалена очередь `runFetch`: она ограничивала параллельные blob-загрузки, а теперь параллельность держат
  браузер (6 соединений на хост) и сама установка (очередь декодирования, 10 с на слот).

### 0.5.7–0.5.9 — the window's own browser cache, shared (23.11.2026)

- Кирилл уточнил: речь про **кэш браузера в AnyText, который пишет в IndexedDB**. До этой правки модуль его не
  касался вовсе: превью шли как `<img src>` на адрес установки, и работал только HTTP-кэш.
- Кэш окна (`anytext-gui/frontend/src/pictures.js`): IndexedDB `anytext-pictures`, хранилища `pictures`
  (blob) и `meta`, ключ `stableKey(bucket, doc, max)` = `collection/document@size`, 200 МБ, 7 дней, вытеснение
  по `storedAt`, плюс карта blob-адресов на страницу. Адрес не годится как ключ: медиа-пропуск меняется
  каждые десять минут.
- Модуль теперь открывает **тот же** IndexedDB (та же схема, версия 2; если окно ушло вперёд — берём его
  версию как есть) и работает по тем же правилам: `bucket/doc@max`, те же поля записи, тот же лимит и срок,
  та же вытеснялка. Страница модуля отдаётся с того же origin, что и окно, поэтому база общая в обе стороны.
- Порядок отрисовки: сперва спрашиваем кэш (5–10 мс), и только если копии нет — ставим адрес и забираем
  картинку один раз, чтобы положить в базу. Первая версия ставила адрес сразу — и скачивала всё заново
  даже при тёплом кэше (12 запросов, 182 КБ на загрузку, из них 6 лишних).
- Замеры после правки: тёплый IndexedDB — **0 запросов, 0 байт**, 6 из 6 карточек показываются из blob;
  прокрутка — новые картинки качаются по разу и ложатся в базу (18 записей на 18 карточек); глава — обе
  картинки из blob.
- Ограничения: кэш общий только на одном origin — модуль на своём порту (7860) имеет свою базу, окно на
  127.0.0.1:7787 свою; в веб-клиенте origin общий (сайт), в приложении тоже (127.0.0.1:7787). Нет
  IndexedDB (webview без хранилища) — модуль молча показывает адреса, ничего не ломается.

### 0.5.10 — «нет связей» это не ошибка (23.11.2026)

- Кирилл: «Network в DevTools кишит ошибками — это пугает». Причина: memory-store отвечает на картинку без
  рёбер фразой `No edges found for "x.png"`, модуль не мог разобрать её как JSON и отдавал **502**. То есть
  нормальный ответ «связей нет» выглядел как сломанный шлюз, и на каждой такой картинке в консоли появлялось
  красное.
- Теперь: `No edges found` — это `{"edges": []}` со статусом 200; блок «Связи» просто не рисуется. Настоящая
  недоступность памяти (MCP не ответил, память не настроена) — тоже 200, но с `"unavailable": true` и
  `note`; просмотрщик в этом случае пишет тихую строку «связи не получены». 400 остаётся только на пустое имя.
- Проверка: картинка из жалобы (`img_1793392506.png`) — 200 и пустой список; прогон по галерее, просмотрщику и
  главе не дал **ни одного** ответа ≥400 в Network.

### 0.5.11 — the media permit: 401 на одной картинке в удалённом клиенте (23.11.2026)

- Симптом Кирилла: в удалённом клиенте при просмотре больших картинок одна картинка отдаёт 401 на
  `https://relay-ru…/s/<id>/api/file?...`, остальные работают; перезагрузка страницы лечит.
- Причина: **релей отдаёт картинки только по медиа-пропуску** (`mt=`), потому что браузер не может положить
  заголовок на `<img src>`. Окно AnyText так и делает (`signed()` в `frontend/src/api.js`), а модуль собирал
  адреса сам, без пропуска. Работали те картинки, что уже лежали в IndexedDB-кэше, — сеть спрашивалась только
  для новых, и падала именно она. Перезагрузка «лечила», потому что после неё всё нужное уже было в кэше.
- Теперь: страница, пришедшая через релей (`/s/<id>/…`), просит `media=1`; модуль берёт пропуск через
  `media.link` установки, кэширует его до истечения (10 минут минус 45 с) и отдаёт экрану `{base, token,
  expires_in}`. Адреса картинок собираются на базе релея с `mt=`. Локально (страница на самой установке)
  пропуск не запрашивается — картинки берутся с диска, как раньше.
- Если пропуск всё же истёк (страница открыта час), картинка, не загрузившаяся первой, просит свежий пропуск
  (`GET /api/signed?doc=&max=`, один запрос на всех) и пробует снова; дальше — оригинал, потом честная ошибка.
- Проверено: с пропуском релей отдаёт превью 200 (39 КБ), картинка в браузере 400×400; без `media=1` (локально)
  ответ не содержит пропуска, поведение прежнее.

### 0.5.12 — карандаш в плотной сетке и телефон (23.11.2026)

- В плотной сетке подписи с кнопками нет вовсе, поэтому до редактора меты было не добраться. Теперь на самой
  картинке в углу — круглая кнопка «✎»: на компьютере появляется при наведении (и при фокусе с клавиатуры), на
  сенсорном экране (`hover: none`) видна всегда. Нажатие открывает тот же редактор.
- «На мобильном не работают стрелки в просмотрщике». Причина оказалась не в кнопках: тулбар не переносился и
  делал всю страницу **544 px шириной на экране 390 px** — layout viewport вырастал, а правая стрелка уезжала
  за край экрана. Теперь `.actions` переносится, поиск тянется, ширина страницы равна экрану (проверено:
  `innerWidth = 390`), стрелки на телефоне лежат поверх картинки по краям (`position: fixed`), кадр —
  `max-height: 72vh`. Проверено касаниями: тап вправо → следующая, тап влево → предыдущая.
- Фон просмотрщика на телефоне почти непрозрачный: подпись под картинкой не должна спорить с тем, что
  просвечивает из-за неё.

### 0.5.14 — таблицы в главах и «Рассказ» в просмотрщике (23.11.2026)

- Markdown-таблицы рендерились как стена палок: `goldmark.New()` без расширений не включает GFM-таблицы.
  Теперь включены `extension.Table`, `Strikethrough`, `TaskList`, `Linkify`, а таблицы в главе получили рамки,
  тихую шапку и чередование строк; на узком экране таблица прокручивается сама, не растягивая страницу.
  Проверено на «Глава. История AnyText, часть первая» — 1 таблица, 7 строк, 2 колонки.
- В просмотрщике рядом с «оригиалом» появилась кнопка **«Рассказ»**: она есть, когда картинка входит в главу, и
  открывает эту главу (из сетки — по `story_name` карточки, из главы — по самой главе). У картинки без главы
  кнопка скрыта.

### 0.5.15–0.5.18 — телефонная шапка (23.11.2026)

- Кирилл: «в мобильном режиме теряем минимум две строки вверху на переключатели и кнопки». Так и было: имя
  галереи, вкладки, два селекта, поиск и «Обновить» переносились на три строки и съедали ~150 px экрана.
- Теперь на телефоне шапка — **одна строка 41 px**: счётчик, вкладки, «⋯». Вся панель (вид, порядок, поиск,
  обновить) живёт под кнопкой и раскрывается по нажатию (148 px), сворачивается сама после выбора сортировки
  или вида и после «Обновить». Открытая шапка — две строки, но только пока человек в ней работает.
- Побочно найден и исправлен старый баг: «Обновить» звал функцию `search()`, переименованную в `runSearch()`
  ещё в 0.5.x, — кнопка падала в консоль и ничего не делала.

## 0.6.x — настройки коллекций в окне (26.11.2026)

- Кирилл: «у картинок есть своя папка по умолчанию, у статей нет — нужен выбор папки для статей», и
  вопрос про картинки (у пользователя их может быть несколько коллекций).
- Модуль теперь читает выбор из `~/.config/teonet/anytext-gallery/config.json`; флаги остаются
  значениями по умолчанию (`-bucket`, `-articles`, `-public-base`). Файл перекрывает флаги, запись
  атомарная (tmp + rename).
- **Картинки — список коллекций**: сетка собирает их все, у картинки появилось поле `bucket`, и
  публикация/описание/корзина идут в коллекцию ключа документа (`bucketOf`). Одинаковые имена в двух
  коллекциях — два разных файла, оба видны. Одна недоступная коллекция не прячет остальные: в ответе
  остаётся всё, что прочиталось.
- **Статьи — одна коллекция**, и она проверяется: если в коллекции нет ни одного `.md`, сохранение
  отклоняется с 400, чтобы вкладка не оказалась пустой без объяснения. Читалка статей берёт имя
  коллекции функцией (`newArticlesReaderDynamic`) и сбрасывает кэш при смене — перезапуск не нужен.
- UI: «⚙» в шапке (только с правом записи), диалог со списком коллекций установки и папками под каждым
  именем, выпадающий список статей, адрес публичных копий, путь файла настроек. После сохранения сетка
  перезагружается — и `state.loading` сбрасывается, иначе ответ, ушедший до сохранения, оставлял старый
  счётчик.
- Побочно: `install-module.sh install` пересобирает всегда — раньше он брал готовый `dist/`, и установка
  получала вчерашний бинарь с сегодняшней записью в реестре (поймано проверкой версии).
- Проверено на живой установке: 212 → 215 картинок при добавлении «Изображений» и обратно 212; отказ
  `«images» нет ни одной статьи (.md)`; файл настроек с одной коллекцией.

### 0.6.2 — «Статей пока нет» после появления конфига (26.11.2026)

- Симптом: коллекция «Мемуары» выбрана, а вкладка говорит «Статей пока нет».
- Причина: в 0.6.0 поле `articlesReader.bucket` стало **функцией** (чтобы коллекцию можно было менять
  без перезапуска), а `text()` продолжал класть в аргументы `doc.text` само поле — то есть `map[string]any`
  с функцией. `json.Marshal` такого не умеет, вызов падал на каждой главе, и `List` честно возвращала
  пустой список без единой ошибки наружу. Компилятор молчал: в `map[string]any` кладётся что угодно.
- Исправлено: `"bucket": r.bucketNow()`.
- Тесты, которых не хватало: `TestArticlesReadTheirCollection` (фальшивая установка отдаёт главу, список
  обязан её вернуть; `readme.md` — не глава) и `TestArticlesFollowTheCollection` (смена коллекции на лету,
  пустая коллекция — пустой список без ошибки). Фальшивая установка теперь умеет `doc.text` и проверяет,
  что коллекция в аргументах **названа**.
- Проверено на живой установке: `41 глава`, первая — «Глава. Доска, ночь и пустой конверт».

## Next steps

1. Metadata: description, tags, prompt, links — read/write in memory-store, and the copy into the
   installation's index described above.
2. Articles: a list of a collection, markdown rendered, pictures open the viewer.
3. Trash and public on the installation's own mechanism (`doc.delete` without deleting the file,
   `doc.restore`), then the Perl service and its basic auth are switched off.
4. Several collections per section — the setting becomes a list, the grid merges with the collection
   in the picture's key, so the data model does not change.
