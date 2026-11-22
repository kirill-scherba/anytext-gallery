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

## Next steps

1. Metadata: description, tags, prompt, links — read/write in memory-store, and the copy into the
   installation's index described above.
2. Articles: a list of a collection, markdown rendered, pictures open the viewer.
3. Trash and public on the installation's own mechanism (`doc.delete` without deleting the file,
   `doc.restore`), then the Perl service and its basic auth are switched off.
4. Several collections per section — the setting becomes a list, the grid merges with the collection
   in the picture's key, so the data model does not change.
