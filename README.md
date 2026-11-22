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

```bash
CGO_ENABLED=0 go build -o ~/.local/bin/anytext-gallery .

curl -s -X POST http://127.0.0.1:7787/api/command -H 'Content-Type: application/json' -d '{
  "command":"modules.install",
  "args":{"id":"gallery","name":"Галерея","names":{"ru":"Галерея","en":"Gallery"},"version":"0.1.0",
          "addr":"127.0.0.1:7860",
          "command":["'"$HOME"'/.local/bin/anytext-gallery","-addr","127.0.0.1:7860","-bucket","галерея"],
          "auto_start":true}}'

curl -s http://127.0.0.1:7787/api/module/gallery/health   # {"status":"ok"}
```

## The screen

| Part | What it does |
|---|---|
| grid | the pictures of the collection's **root folder** — not the copies in `pub/`, not the trash, not a subfolder |
| previews | taken from `/api/thumb` with the account token, six at a time, when a card comes near the screen |
| viewer | the picture itself (`/api/file`), its name, arrows and `←`/`→`, `Esc` closes |
| two grids | **dense** — pictures only; **cards** — the size and shape of the old gallery: preview, name, date, description, the editor button. The choice is remembered |
| search | by name of the file and its folder |
| meta | description, tags, context, generation prompt, the story a picture belongs to: read from `memory/gallery/meta/<file>`, written there and **copied** into the installation as the picture's description |
| migrate | `POST /api/migrate` copies every known description into the installation once |
| articles | the tab shows the chapters of a collection (the memoirs): newest first, with a date and the first line; a chapter opens as a page, its pictures open the viewer |
| story | a picture that appears in a chapter gets a «Рассказ» button — the link is computed from the chapter's text, nobody fills it by hand |
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
| `GET`/`POST`/`DELETE /api/public` | the public copy of a picture (`https://bmat.uk/i/pub/<file>`) |
| `POST /api/trash` | a picture goes to the trash of the installation, the file stays |
| `POST /api/migrate` | every known description is copied into the installation once |
| `GET /api/articles` | the chapters of the `-articles` collection: title, author, date, first line, pictures |
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
