# anytext-gallery — the gallery module of AnyText

A grid of the pictures of one collection, previews taken from the installation's own thumbnail
endpoint, and a viewer that walks the collection. The module owns no pictures and no database: the
collection belongs to the installation, and the screen is a window over it.

```
person ──► AnyText (proxy /api/module/gallery/) ──► anytext-gallery :7860
                                                        │
                                                        ├── rag.list / bucket.list  (what is in it)
                                                        └── /api/thumb, /api/file    (the pictures)
```

## What it needs

- AnyText 0.4.16 or newer: a module process is started with `ANYTEXT_SELF`, and the picture endpoints
  answer with the account token the window puts on every request — so previews work at the machine and
  through the relay alike.
- **AnyText 0.4.18 for the article editor**: it needs `doc.text {raw: true}` (the file, not the extracted
  text), `doc.save_raw` (write a chapter back) and `doc.list` with `mod_time`/`description`. An older
  installation still shows the grid and the chapters, but «Редактировать» cannot save.
- Nothing else. No database, no second copy of the files.

## Build and install

Один скрипт: собрать, поставить, проверить. Он же умеет ставить на другую машину и публиковать бинарь.

```bash
./scripts/install-module.sh build                  # dist/anytext-gallery-linux-amd64 + sha256
./scripts/install-module.sh install                # на этой машине: бинарь, реестр, перезапуск, проверка
./scripts/install-module.sh install --remote host  # то же на другой машине через ssh-agent
./scripts/install-module.sh publish                # в бакет modules/gallery/<version>/ с sha256
./scripts/install-module.sh all                    # build + install (чаще всего это)
```

- Коллекции и адрес скрипт берёт **из уже установленной записи** в
  `~/.config/teonet/anytext/modules/modules.json`: обновление не меняет настройки модуля. Если записи
  нет — значения по умолчанию, их можно переопределить переменными `BUCKET`, `ARTICLES`,
  `PUBLIC_BASE`, `ADDR`, `INSTALL`.
- Версия — из `main.go`; её же скрипт сверяет с той, что отвечает `/api/module/gallery/api/status`
  после перезапуска, и падает, если они разошлись.
- `--remote` находит сокет ssh-агента с ключом сам (`~/.ssh/agent/*`), копирует бинарь, пишет реестр
  на той машине её же `/api/command` и запускает модуль.
- `publish` кладёт в `gs://cooksy-ea86d-anytext-releases/modules/gallery/<version>/` бинарь и его
  sha256 — это первый шаг к каталогу модулей (в ядре установки по адресу пока нет).

Без скрипта: `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o ~/.local/bin/anytext-gallery .`,
затем `modules.install` с командной строкой.

## The screen

| Part | What it does |
|---|---|
| grid | the pictures of the collection's **root folder** — not the copies in `pub/`, not the trash, not a subfolder |
| previews | the installation's `/api/thumb` — its own cache, memory and disk — and the **browser cache of the window itself**: the module reads and fills the same IndexedDB (`anytext-pictures`, key `collection/document@size`, 200 MB, seven days) that anytext-gui keeps, so a picture seen in the window opens in the gallery at once and the other way round. A chapter and the viewer ask for a 1400 px preview, not the original; the viewer carries an «оригинал» link |
| viewer | the picture itself (a 1400 px preview from the installation's cache), its name, arrows and `←`/`→`, `Esc` closes; under the name — «оригинал» (the full file), «Рассказ» (the chapter it is in, when it is in one) and **«Связи»**: the memory graph's relations of this picture, as the old gallery showed them |
| the pencil | in the dense grid the caption is hidden, so the picture carries a pencil in its corner: hover on a computer, always on a touch screen |
| two grids | **dense** — pictures only; **cards** — the size and shape of the old gallery: preview, name, date, description, the editor button. The choice is remembered |
| search (pictures) | by file name, folder, description and tags — every word of the query must be found; instant, no model involved |
| search (chapters) | the title, the author, the date and the whole text of the chapter; the result carries the line the query was found in, without markdown |
| search by meaning | Enter, or an empty result, asks the installation's own meaning search (`rag.search`): pictures by what is drawn on them, chapters by what they are about. The counter says «по смыслу», the cut is relative to the best hit (24 at most) |
| graph mark | a card the memory graph knows something about carries «🔗 N» — the old gallery's «Связи» button; the relations themselves are under the picture in the viewer |
| meta | description, tags, context, generation prompt, the story a picture belongs to: read from `memory/gallery/meta/<file>`, written there and **copied** into the installation as the picture's description. «Описать моделью» asks the installation's vision model and puts its words into the field as a draft |
| migrate | `POST /api/migrate` copies every known description into the installation once |
| articles | the tab shows the chapters of a collection (the memoirs): newest first, with a date and the first line; a chapter opens as a page — markdown with GFM tables, strikethrough, task lists and linkified addresses — and its pictures open the viewer |
| story | a picture that appears in a chapter gets a «Рассказ» button — the link is computed from the chapter's text; a picture whose meta names a chapter (the 19 records bmat left) gets it too, even when the chapter does not spell the file name out |
| article editor | «Редактировать» on a chapter opens its markdown — the file as it is, not the extracted text — and saves it back with `doc.save_raw`; the button is offered only to a person who holds the write right |

The screen is **one self-contained document**: styles and script inside the page. A module split into
files shows bare HTML through the relay (docs/MODULES.md in anytext-gui, item 4).

## API

| Request | Answer |
|---|---|
| `GET /health` | `{"status":"ok"}` |
| `GET /api/status` | module version, installation address, collection, how many pictures, why not |
| `GET /api/config` | the collection the module works with |
| `GET /api/images?offset=&limit=&q=&sort=` | one page of the grid: `{bucket, total, offset, limit, items}` |
| `GET /api/meta?name=` · `POST` · `DELETE` | the description of a picture in `memory/gallery/meta/<file>`, copied into the installation |
| `GET /api/signed?doc=&max=` | a fresh relay media permit (`{media: {base, token, expires_in}}`); asked when a picture answers 401 |
| `GET /api/edges?name=` | the memory graph's relations of one picture (`graph_get_edges`), asked when a picture is opened and remembered for two minutes. A picture nobody has linked answers `{"edges": []}` with 200 — «No edges found» is an answer, not an error; a memory that did not answer answers 200 with `unavailable: true` |
| `POST /api/describe?doc=` | the installation's vision model looks at the picture (`doc.describe` — the same command the window's «Описать моделью» runs); the answer fills the editor, it is not saved |
| `GET`/`POST`/`DELETE /api/public` | the public copy of a picture (`https://bmat.uk/i/pub/<file>`) |
| `POST /api/trash` | a picture goes to the trash of the installation, the file stays |
| `POST /api/migrate` | every known description is copied into the installation once |
| `GET /api/articles?q=` | the chapters of the `-articles` collection: title, author, date, first line, pictures; `q` searches the title, the author, the date and the text and adds `snippet` |
| `GET /api/search?in=pictures\|articles&q=` | the installation's meaning search, turned into card items (pictures) or chapter rows (articles) |
| `GET /api/article?name=` | one chapter: `{title, html, source, docs}` — `source` is the markdown the editor shows |
| `POST /api/article` `{name, text}` | the chapter is written back with `doc.save_raw`; needs the write right, refused for a picture or an empty text |

A picture is `{doc, name, folder, vision, has_description}` — `doc` is the document key the picture
endpoints expect.

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `-addr` | `127.0.0.1:7860` | where the module listens (loopback only) |
| `-install` | `$ANYTEXT_SELF` | the installation, `http://127.0.0.1:7787` when nothing is set |
| `-bucket` | `галерея` | the collection with the pictures |
| `-articles` | `мемуары` | the collection with the long-form articles (empty: no articles tab) |
| `-public-base` | — | the address public copies are served at, e.g. `https://bmat.uk/i/pub` |

## Tests

```bash
go vet ./... && go test ./...
```

A fake installation answers `bucket.list` and `rag.list`: the tests check that only pictures of the
root folder are shown, that a page and a filter work, that an unreachable installation is named as
such, and that the page stays self-contained.
