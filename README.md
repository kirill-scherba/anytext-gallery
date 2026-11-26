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

One script: build, install, check. It can also install on another machine and publish the binary.

```bash
./scripts/install-module.sh build                  # five platforms into dist/, each with its sha256
./scripts/install-module.sh install                # here: binary, registry entry, restart, check
./scripts/install-module.sh install --remote host  # the same on another machine through the ssh-agent
./scripts/install-module.sh publish                # all five into the bucket and the catalogue entry
./scripts/install-module.sh all                    # build + install (what is used most of the time)
```

`build` makes the same five builds the application itself makes — `linux-amd64`, `linux-arm64`,
`windows-amd64` (.exe), `darwin-arm64`, `darwin-amd64` — because a module is a static Go binary.
`publish` uploads them to `gs://cooksy-ea86d-anytext-releases/modules/gallery/<version>/` and writes
the entry into `catalog.json` of `matrica-modules-dist` with the checksum and size of every file: an
installation reads that file and installs the module with one command (`modules.install {id}`).

- The collections and the address are taken **from the entry already installed** in
  `~/.config/teonet/anytext/modules/modules.json`, so updating the module does not change how it is
  configured. With no entry the defaults are used; `BUCKET`, `ARTICLES`, `PUBLIC_BASE`, `ADDR` and
  `INSTALL` override them.
- The version comes from `main.go`, and the script compares it with what
  `/api/module/gallery/api/status` answers after the restart — a binary built after a change but
  without a version bump fails the check instead of looking old.
- `--remote` finds the ssh-agent socket that holds a key itself (`~/.ssh/agent/*`), copies the binary,
  writes the registry on that machine through its own `/api/command` and starts the module.
- `publish` puts the binary and its sha256 into
  `gs://cooksy-ea86d-anytext-releases/modules/gallery/<version>/` — the first step towards the module
  catalogue (the core cannot install from an address yet).

Without the script: `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o ~/.local/bin/anytext-gallery .`
and then `modules.install` with the command line.

## The screen

| Part | What it does |
|---|---|
| grid | the pictures of the collection's **root folder** — not the copies in `pub/`, not the trash, not a subfolder |
| previews | the installation's `/api/thumb` — its own cache, memory and disk — and the **browser cache of the window itself**: the module reads and fills the same IndexedDB (`anytext-pictures`, key `collection/document@size`, 200 MB, seven days) that anytext-gui keeps, so a picture seen in the window opens in the gallery at once and the other way round. A chapter and the viewer ask for a 1400 px preview, not the original; the viewer carries an «оригинал» link |
| viewer | the picture itself (a 1400 px preview from the installation's cache), its name, arrows and `←`/`→`, `Esc` closes; under the name — «оригинал» (the full file), «Рассказ» (the chapter it is in, when it is in one) and **«Связи»**: the memory graph's relations of this picture, as the old gallery showed them |
| the pencil | in the dense grid the caption is hidden, so the picture carries a pencil in its corner: hover on a computer, always on a touch screen |
| settings | «⚙» in the header (a person with the write right): the collections of pictures the grid shows — several at once — the collection of chapters, and the address of the public copies. A person chooses from the installation's own list, with each collection's folders shown under it |
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

## Settings

The flags are the defaults. What a person chooses in the window is kept in
`~/.config/teonet/anytext-gallery/config.json` and overrides them:

```json
{ "pictures": ["галерея", "images"], "articles": "мемуары", "public_base": "https://bmat.uk/i/pub" }
```

- **pictures** — a list: a person may keep generated pictures, screenshots and photographs in different
  collections, and the grid is the one place they belong together. A card of a second collection is
  published, described and thrown away in that collection — the document key starts with its name.
- **articles** — one collection, or empty for no articles tab. The module refuses a collection with no
  `.md` in it before it saves, so an empty tab cannot appear without a word about why.
- the choice is applied at once: the grid reloads, the articles reader starts reading the new collection.

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `-addr` | `127.0.0.1:7860` | where the module listens (loopback only) |
| `-install` | `$ANYTEXT_SELF` | the installation, `http://127.0.0.1:7787` when nothing is set |
| `-bucket` | `галерея` | the collection with the pictures — the default, until the window saves a choice |
| `-articles` | `мемуары` | the collection with the long-form articles (empty: no articles tab) — the default too |
| `-public-base` | — | the address public copies are served at, e.g. `https://bmat.uk/i/pub` |

## Tests

```bash
go vet ./... && go test ./...
```

A fake installation answers `bucket.list` and `rag.list`: the tests check that only pictures of the
root folder are shown, that a page and a filter work, that an unreachable installation is named as
such, and that the page stays self-contained.
