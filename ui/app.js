// Copyright 2026 Kirill Scherba. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// app.js — the screen of the gallery module: a grid of the collection's pictures, previews taken
// from the installation's own thumbnail endpoint, and a viewer that walks the collection.
//
// The module serves no picture bytes: a preview is fetched with the account token the shell puts on
// every request (window.fetch is wrapped by the window), so the same page works at the machine and
// through the relay (docs/MODULES.md in anytext-gui, items 3 and 4).

'use strict'

const LANGS = ['ru', 'en']

function storedLang() {
  try {
    const value = localStorage.getItem('anytext.lang')
    if (LANGS.includes(value)) return value
  } catch (e) { /* private mode */ }
  return (navigator.language || '').toLowerCase().startsWith('ru') ? 'ru' : 'en'
}

let lang = storedLang()

const STRINGS = {
  ru: {
    gallery: 'Галерея', pictures: 'Картинки', articles: 'Статьи',
    searchPlaceholder: 'поиск: имя, описание, теги',
    searchPlaceholderArticles: 'поиск по главам',
    searchingMeaning: 'Ищу по смыслу…',
    byMeaning: 'по смыслу',
    meaningEmpty: 'По смыслу ничего не нашлось',
    found: 'найдено',
    refresh: 'Обновить',
    loading: 'Загрузка…', empty: 'В этой коллекции пока нет картинок',
    nothingFound: 'Ничего не найдено', error: 'Картинки не пришли',
    picturesCount: 'картинок', of: 'из',
    sortTitle: 'порядок', sortNew: 'новые сверху', sortName: 'по имени',
    modeTitle: 'вид сетки', modeCompact: 'плотная', modeCards: 'карточки',
    edit: 'Изменить', metaTitle: 'Описание картинки',
    makePublic: 'Сделать публичной', unpublic: 'Убрать из публичных',
    publicLink: 'публичная ссылка', trash: 'Удалить', trashTitle: 'Убрать из галереи (файл остаётся)',
    trashed: 'Убрано из галереи', publicDone: 'Скопировано в public',
    metaDescription: 'Описание', metaTags: 'Теги (через запятую)',
    metaContext: 'Контекст', metaPrompt: 'Prompt генерации', metaMemoir: 'Рассказ (имя файла)',
    metaDelete: 'Удалить мету', cancel: 'Отмена', save: 'Сохранить',
    backToArticles: 'статьи', story: 'Рассказ', articlesEmpty: 'Статей пока нет', author: 'Автор', contents: 'Оглавление',
    settings: 'Настройки',
    settingsHint: 'Картинки — коллекции, которые показывает сетка. Статьи — коллекция, в которой лежат главы.',
    picturesCollections: 'Коллекции картинок',
    articlesCollection: 'Коллекция статей',
    publicBase: 'Адрес публичных копий',
    settingsSaved: 'Настройки сохранены',
    noArticles: '— без статей —',
    settingsFile: 'Файл настроек',
    editArticle: 'Редактировать',
    describeWithModel: 'Описать моделью',
    describeWaiting: 'Модель смотрит на картинку…',
    describeDone: 'Описание получено — проверьте и сохраните',
    edges: 'Связи',
    original: 'оригинал',
    edgesNone: 'связей нет',
    edgesError: 'связи не получены',
    articleHint: 'Markdown сохраняется в файл главы как есть; картинку достаточно указать именем файла: ![подпись](img_123.png).',
    articleSaved: 'Глава сохранена',
    publish: 'Опубликовать',
    published: 'Опубликовано',
    publishedChanged: 'Опубликовано · изменилось',
    unpublish: 'Снять',
    publishConfirm: 'Опубликовать статью наружу?\n\nТекст уйдёт на страницу, картинки станут публичными по ссылке.',
    unpublishConfirm: 'Снять с публикации? Картинки будут удалены из хранилища, а страница останется с пометкой «снята»: удалить её у Telegraph нельзя.',
    publishedHint: 'Статья опубликована: {url}',
    articlesError: 'Статьи не пришли',
    savedToMemory: 'Сохранено',
    savedPartial: 'Сохранено в память, но не в установку:', articlesSoon: 'Статьи — следующий шаг',
    install: 'установка', collection: 'коллекция',
  },
  en: {
    gallery: 'Gallery', pictures: 'Pictures', articles: 'Articles',
    searchPlaceholder: 'search: name, description, tags',
    searchPlaceholderArticles: 'search the chapters',
    searchingMeaning: 'Searching by meaning…',
    byMeaning: 'by meaning',
    meaningEmpty: 'Nothing found by meaning',
    found: 'found',
    refresh: 'Refresh',
    loading: 'Loading…', empty: 'No pictures in this collection yet',
    nothingFound: 'Nothing found', error: 'The pictures did not arrive',
    picturesCount: 'pictures', of: 'of',
    sortTitle: 'order', sortNew: 'newest first', sortName: 'by name',
    modeTitle: 'grid', modeCompact: 'dense', modeCards: 'cards',
    edit: 'Edit', metaTitle: 'About this picture',
    makePublic: 'Make public', unpublic: 'Remove public',
    publicLink: 'public link', trash: 'Delete', trashTitle: 'Remove from the gallery (the file stays)',
    trashed: 'Removed from the gallery', publicDone: 'Copied to public',
    metaDescription: 'Description', metaTags: 'Tags (comma separated)',
    metaContext: 'Context', metaPrompt: 'Generation prompt', metaMemoir: 'Story (file name)',
    metaDelete: 'Delete meta', cancel: 'Cancel', save: 'Save',
    backToArticles: 'articles', story: 'Story', articlesEmpty: 'No articles yet', author: 'Author', contents: 'Contents',
    settings: 'Settings',
    settingsHint: 'Pictures are the collections the grid shows. Articles are the collection the chapters live in.',
    picturesCollections: 'Collections of pictures',
    articlesCollection: 'Collection of articles',
    publicBase: 'Address of public copies',
    settingsSaved: 'Settings saved',
    noArticles: '— no articles —',
    settingsFile: 'Settings file',
    editArticle: 'Edit',
    describeWithModel: 'Describe with a model',
    describeWaiting: 'The model is looking at the picture…',
    describeDone: 'The description is here — read it and save',
    edges: 'Connections',
    original: 'original',
    edgesNone: 'no connections',
    edgesError: 'connections did not come',
    articleHint: 'The markdown is saved into the chapter file as is; a picture is named by its file: ![alt](img_123.png).',
    articleSaved: 'The chapter is saved',
    publish: 'Publish',
    published: 'Published',
    publishedChanged: 'Published · changed',
    unpublish: 'Remove',
    publishConfirm: 'Publish this article outside?\n\nThe text goes to a page, the pictures become public by their address.',
    unpublishConfirm: 'Take it down? The pictures are removed from storage; the page stays, marked as taken down - Telegraph cannot delete it.',
    publishedHint: 'The article is published: {url}',
    articlesError: 'The articles did not arrive',
    savedToMemory: 'Saved',
    savedPartial: 'Saved in the memory, but not in the installation:', articlesSoon: 'Articles are the next step',
    install: 'installation', collection: 'collection',
  },
}

function t(key) {
  const table = STRINGS[lang] || STRINGS.en
  return table[key] !== undefined ? table[key] : key
}

function locale() {
  return lang === 'ru' ? 'ru-RU' : 'en-GB'
}

function applyTheme() {
  let theme = 'dark'
  try {
    theme = localStorage.getItem('anytext-theme') || 'dark'
  } catch (e) { /* private mode */ }
  document.documentElement.classList.toggle('dark', theme !== 'light')
}

const $ = (id) => document.getElementById(id)

/**
 * installBase is the installation this module belongs to. The page is served at
 * <installation>/api/module/gallery/ (and through the relay at <site>/s/<id>/api/module/gallery/),
 * so stripping the module's own tail leaves the address the picture endpoints live under.
 */
function installBase() {
  const b = new URL(document.baseURI)
  // Whatever the module is mounted as — /api/module/gallery, another name, or the root of its own
  // port — the pictures live at the installation's /api/..., so the module's tail is cut off and the
  // trailing slash with it: a base that ends in "/" makes every address start with "//" (23.11.2026).
  b.pathname = b.pathname.replace(/api\/module\/[^/]+\/?$/, '').replace(/\/+$/, '')
  return (b.origin + b.pathname).replace(/\/+$/, '')
}

class ApiError extends Error {}

async function api(path, options) {
  const res = await fetch(path, options)
  let data = null
  try {
    data = await res.json()
  } catch (e) {
    throw new ApiError(`${res.status} ${res.statusText}`)
  }
  if (!res.ok) throw new ApiError((data && data.error) || `${res.status} ${res.statusText}`)
  return data
}

const state = {
  bucket: '',
  items: [],
  total: 0,
  offset: 0,
  q: '',
  sort: 'new',
  mode: 'compact',
  tab: 'pictures',
  loading: false,
  done: false,
  error: '',
  // semantic is how many results came from the meaning search: the grid then says so and stops
  // asking for more pages (23.11.2026).
  semantic: 0,
  articleQ: '',
  // media is the relay's permit for picture addresses: {base, token, expires_in}.
  media: null,
}

// One page of the grid: a window taller than sixty cards asks for the next page at once.
const LIMIT = 60

// --- the picture cache of the installation's own window -------------------------------------------
//
// The window keeps a copy of every picture it has shown in IndexedDB «anytext-pictures», keyed by
// collection, document and size (anytext-gui/frontend/src/pictures.js). The module is served from the
// same origin, so it reads and fills that same cache: a picture the window has already seen opens in
// the gallery at once, and what the gallery keeps is there for the window. The rules are the window's
// own — 200 MB, seven days, oldest first — because two caches with two policies would fight
// (23.11.2026).
const PIC_DB = 'anytext-pictures'
const PIC_STORE = 'pictures'
const PIC_META = 'meta'
const PIC_LIMIT = 200 * 1024 * 1024
const PIC_MAX_AGE = 7 * 24 * 60 * 60 * 1000

let picDBPromise = null
const picURLs = new Map() // key -> blob address of this page

/** picKey names one picture the way the window does: the permit is not part of it, the size is. */
function picKey(bucket, doc, max) {
  return `${bucket || ''}/${doc || ''}@${max || 0}`
}

/** picOpen opens the window's database without asking for a version: whatever version the window
 *  keeps, the module takes it as it is. A missing store is not an error — it is a cache turned off. */
function picOpen() {
  if (picDBPromise) return picDBPromise
  picDBPromise = new Promise((resolve) => {
    if (typeof indexedDB === 'undefined') {
      resolve(null)
      return
    }
    const usable = (db) => (db.objectStoreNames.contains(PIC_STORE) && db.objectStoreNames.contains(PIC_META) ? db : null)
    let req
    try {
      // The window's own version and its own two stores: the module may find the database or create it,
      // and both sides must see the same shape. A database the window has already moved past is taken as
      // it is — the stores are checked, not the number.
      req = indexedDB.open(PIC_DB, 2)
    } catch (e) {
      resolve(null)
      return
    }
    req.onupgradeneeded = () => {
      const db = req.result
      if (!db.objectStoreNames.contains(PIC_STORE)) {
        const os = db.createObjectStore(PIC_STORE, { keyPath: 'key' })
        os.createIndex('storedAt', 'storedAt')
      }
      if (!db.objectStoreNames.contains(PIC_META)) {
        db.createObjectStore(PIC_META, { keyPath: 'key' })
      }
    }
    req.onsuccess = () => resolve(usable(req.result))
    req.onerror = () => {
      // A version the window has raised above ours: open whatever it keeps, and use it if the stores are
      // there. Opening without a version never creates anything.
      const fallback = indexedDB.open(PIC_DB)
      fallback.onsuccess = () => resolve(usable(fallback.result))
      fallback.onerror = () => resolve(null)
    }
  })
  return picDBPromise
}

function picRow(db, key) {
  return new Promise((resolve) => {
    try {
      const req = db.transaction(PIC_STORE, 'readonly').objectStore(PIC_STORE).get(key)
      req.onsuccess = () => resolve(req.result || null)
      req.onerror = () => resolve(null)
    } catch (e) {
      resolve(null)
    }
  })
}

function picStore(db, key, blob) {
  return new Promise((resolve) => {
    try {
      const tx = db.transaction([PIC_STORE, PIC_META], 'readwrite')
      tx.objectStore(PIC_META).put({ key, storedAt: Date.now(), bytes: blob.size })
      const req = tx.objectStore(PIC_STORE).put({ key, blob, storedAt: Date.now(), bytes: blob.size })
      req.onsuccess = () => resolve(true)
      req.onerror = () => resolve(false)
    } catch (e) {
      resolve(false)
    }
  })
}

/** picTrim drops the copies the window would drop: too old, or over the limit, oldest first. */
async function picTrim(db) {
  try {
    const rows = await new Promise((resolve) => {
      const req = db.transaction(PIC_META, 'readonly').objectStore(PIC_META).getAll()
      req.onsuccess = () => resolve(req.result || [])
      req.onerror = () => resolve([])
    })
    rows.sort((a, b) => (a.storedAt || 0) - (b.storedAt || 0))
    let total = rows.reduce((sum, r) => sum + (r.bytes || 0), 0)
    const now = Date.now()
    for (const row of rows) {
      const tooOld = now - (row.storedAt || 0) > PIC_MAX_AGE
      if (!tooOld && total <= PIC_LIMIT) break
      const tx = db.transaction([PIC_STORE, PIC_META], 'readwrite')
      tx.objectStore(PIC_STORE).delete(row.key)
      tx.objectStore(PIC_META).delete(row.key)
      total -= row.bytes || 0
      const url = picURLs.get(row.key)
      if (url) {
        URL.revokeObjectURL(url)
        picURLs.delete(row.key)
      }
    }
  } catch (e) {
    // A cache that cannot be trimmed is still a cache; the window trims it on its own writes.
  }
}

/** picFetch goes to the network once for a picture. Through a relay the address is another origin and
 *  the <img> has already loaded it as an opaque response, so the copy has to be asked past the cache;
 *  on this machine the address is the same origin and the browser's own copy is readable. */
async function picFetch(address) {
  try {
    const url = new URL(address, location.href)
    const res = await fetch(address, { cache: url.origin === location.origin ? 'default' : 'reload' })
    if (!res.ok) return null
    const blob = await res.blob()
    if (!blob.size || (blob.type && !blob.type.startsWith('image/'))) return null
    return blob
  } catch (e) {
    return null
  }
}

/** paintPicture puts a picture on the screen: the kept copy first, the address when there is none, and
 *  a copy taken for the next visit. The order matters — asking the address first would download every
 *  picture the cache already holds (23.11.2026). */
async function paintPicture(img, bucket, doc, max, address) {
  const key = picKey(bucket, doc, max)
  const kept = picURLs.get(key)
  if (kept) {
    img.src = kept
    return
  }
  const db = await picOpen()
  if (db) {
    const row = await picRow(db, key)
    if (row && row.blob instanceof Blob && Date.now() - (row.storedAt || 0) < PIC_MAX_AGE) {
      const url = URL.createObjectURL(row.blob)
      picURLs.set(key, url)
      img.src = url
      return
    }
  }
  img.src = address
  if (!db) return
  const blob = await picFetch(address)
  if (!blob) return
  if (!(await picStore(db, key, blob))) return
  const url = URL.createObjectURL(blob)
  picURLs.set(key, url)
  if (img.isConnected) img.src = url
  void picTrim(db)
}

/** relayed says the page came through the relay (/s/<id>/api/module/...): its pictures need a media
 *  permit, while a page served on the installation itself takes them from disk (23.11.2026). */
function relayed() {
  return location.pathname.startsWith('/s/')
}

/** mediaBase is where the pictures are served: the relay's address when the installation handed out a
 *  permit, the installation itself otherwise. */
function mediaBase() {
  return state.media && state.media.base ? state.media.base : installBase()
}

/** withPermit adds the relay's media permit. A browser cannot put an Authorization header on an
 *  <img src>, so the permit travels in the query — without it a relayed picture answers 401, which is
 *  the one picture that broke while the rest came from the cache (23.11.2026). */
function withPermit(params) {
  if (state.media && state.media.token) params.set('mt', state.media.token)
  return params
}

/** bucketOfDoc reads the collection out of a document key: a key starts with its collection, and the
 *  grid may show several collections at once (26.11.2026). */
function bucketOfDoc(doc) {
  const i = String(doc || '').indexOf('/')
  return i > 0 ? doc.slice(0, i) : state.bucket
}

/** thumbURL is the installation's preview of one picture. */
function thumbURL(doc, max) {
  const q = withPermit(new URLSearchParams({ bucket: bucketOfDoc(doc), doc, max: String(max) }))
  return `${mediaBase()}/api/thumb?${q.toString()}`
}

let mediaRefresh = null

/** refreshMedia asks the module for a fresh permit. One call at a time: a screen full of pictures that
 *  expired together must not become a screen full of permit requests (23.11.2026). */
async function refreshMedia(doc, max) {
  if (!mediaRefresh) {
    mediaRefresh = api('api/signed?doc=' + encodeURIComponent(doc) + '&max=' + max)
      .then((answer) => {
        if (answer && answer.media) state.media = answer.media
        return answer && answer.media
      })
      .finally(() => { mediaRefresh = null })
  }
  return mediaRefresh
}

/** pictureFallback walks the three answers to a picture that did not come: a fresh permit, then the
 *  picture itself, then the truth. */
function pictureFallback(img, doc, max, final) {
  const step = Number(img.dataset.step || 0)
  img.dataset.step = String(step + 1)
  if (step === 0) {
    refreshMedia(doc, max)
      .then((media) => { if (img.isConnected) img.src = media ? previewURL(doc, max) : fileURL(doc) })
      .catch(() => { if (img.isConnected) img.src = fileURL(doc) })
    return
  }
  if (step === 1) {
    img.src = fileURL(doc)
    return
  }
  final()
}

/** previewURL is the installation's preview — the one it renders once and keeps in its own cache
 *  (memory and on disk). A memoir picture is a 1.5-2.5 MB PNG and a decode of several megapixels; its
 *  1400 px preview is 150-250 KB and comes from that cache. A vector is taken whole: it must stay
 *  sharp (23.11.2026). */
function previewURL(doc, max) {
  return /\.svg$/i.test(doc) ? fileURL(doc) : thumbURL(doc, max)
}

/** fileURL is the picture itself, not compressed. */
function fileURL(doc) {
  const q = withPermit(new URLSearchParams({ bucket: bucketOfDoc(doc), doc }))
  return `${mediaBase()}/api/file?${q.toString()}`
}

async function loadConfig() {
  try {
    const cfg = await api('api/config')
    state.bucket = cfg.bucket || ''
    $('footInfo').textContent = `v${cfg.version} · ${t('collection')} ${state.bucket}`
  } catch (e) {
    state.bucket = 'галерея'
  }
}

async function loadPage(reset, fresh) {
  if (state.loading || (state.done && !reset)) return
  if (reset) {
    state.items = []
    state.offset = 0
    state.done = false
    state.error = ''
    $('cards').innerHTML = ''
    hideEmpty()
  }
  state.loading = true
  try {
    const q = new URLSearchParams({ offset: String(state.offset), limit: String(LIMIT), sort: state.sort })
    if (state.q) q.set('q', state.q)
    if (fresh) q.set('refresh', '1')
    if (relayed()) q.set('media', '1')
    const page = await api('api/images?' + q.toString())
    if (page.media) state.media = page.media
    state.total = page.total
    state.offset = page.offset + page.items.length
    state.done = page.items.length === 0 || state.offset >= page.total
    state.items = state.items.concat(page.items)
    renderCards(page.items)
    updateCounter()
    showEmptyIfNeeded()
    // A window taller than the first page has nothing to scroll: ask for the next page until the
    // grid reaches past the screen or the collection ends.
    const scroller = $('scroller')
    if (!state.done && scroller && scroller.scrollHeight <= scroller.clientHeight + 80) {
      state.loading = false
      return loadPage(false)
    }
  } catch (e) {
    state.error = e.message
    showNotice(`${t('error')}: ${e.message}`)
  } finally {
    state.loading = false
    updateCounter()
    // The window may be taller than the page that just arrived: ask again until the grid fills it.
    setTimeout(maybeMore, 0)
  }
}

function renderCards(items) {
  const grid = $('cards')
  for (const item of items) {
    const card = document.createElement('div')
    card.className = 'card'
    card.dataset.doc = item.doc
    card.title = item.name
    card.innerHTML = `<div class="thumb"><span class="ph">${esc(shortName(item.name))}</span></div>` +
      cardInfoHTML(item)
    card._item = item
    // The pencil on the picture itself: the dense grid hides the caption with its buttons, and a
    // person must not have to switch the grid to rename a picture (23.11.2026).
    const pencil = document.createElement('button')
    pencil.type = 'button'
    pencil.className = 'card-edit'
    pencil.title = t('edit')
    pencil.setAttribute('aria-label', t('edit'))
    pencil.textContent = '✎'
    pencil.addEventListener('click', (event) => { event.stopPropagation(); openMeta(item) })
    card.appendChild(pencil)
    const thumb = card.querySelector('.thumb')
    thumb.addEventListener('click', () => openViewer(item))
    bindCardInfo(card, item)
    grid.appendChild(card)
    observeCard(card, item)
  }
}

/** bindCardInfo attaches the editor of the caption, and rebinds it after the caption is redrawn. */
function bindCardInfo(card, item) {
  const edit = card.querySelector('[data-act="edit"]')
  if (edit) edit.addEventListener('click', (event) => { event.stopPropagation(); openMeta(item) })
  const pub = card.querySelector('[data-act="public"]')
  if (pub) pub.addEventListener('click', (event) => { event.stopPropagation(); togglePublic(item, card) })
  const trash = card.querySelector('[data-act="trash"]')
  if (trash) trash.addEventListener('click', (event) => { event.stopPropagation(); trashPicture(item, card) })
  const edges = card.querySelector('[data-act="edges"]')
  if (edges) edges.addEventListener('click', (event) => { event.stopPropagation(); openViewer(item) })
  const story = card.querySelector('[data-act="story"]')
  if (story) story.addEventListener('click', (event) => { event.stopPropagation(); openArticle(item.story_name) })
}

/** redrawInfo replaces the caption of one card after its meta changed. */
function redrawInfo(card, item) {
  const info = card.querySelector('.info')
  if (!info) return
  info.outerHTML = cardInfoHTML(item)
  bindCardInfo(card, item)
}

/** cardInfoHTML is the caption under a picture: shown in the "cards" grid, hidden in the dense one. */
function cardInfoHTML(item) {
  const when = item.mod_time ? new Date(item.mod_time).toLocaleString(locale(), { day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' }) : ''
  const desc = item.description || (item.meta && item.meta.description) || ''
  const pubUrl = item.public && item.public_url ? item.public_url : ''
  return `<div class="info">
    <div class="info-row"><span class="name">${esc(item.name)}</span>
      <button type="button" class="text del" data-act="trash" title="${esc(t('trashTitle'))}">${esc(t('trash'))}</button></div>
    <div class="date">${esc(when)}</div>
    ${desc ? `<div class="desc">${esc(desc)}</div>` : ''}
    <div class="actions">
      ${item.edges ? `<button type="button" class="text graph-btn" data-act="edges" title="${esc(t('edges'))}">🔗 ${item.edges}</button>` : ''}
      <button type="button" class="text ${item.public ? 'unpub' : 'pub'}" data-act="public">${esc(item.public ? t('unpublic') : t('makePublic'))}</button>
      <button type="button" class="text" data-act="edit">${esc(t('edit'))}</button>
      ${item.story ? `<button type="button" class="text story-btn" data-act="story">${esc(t('story'))}</button>` : ''}
    </div>
    ${item.public ? `<div class="pub-url"><a href="${esc(pubUrl)}" target="_blank" rel="noopener">${esc(t('publicLink'))}</a></div>` : ''}
  </div>`
}

/** applyMode switches the grid between the dense tiles and the cards with words. */
function applyMode() {
  const grid = $('cards')
  if (!grid) return
  grid.classList.toggle('mode-cards', state.mode === 'cards')
  grid.classList.toggle('mode-compact', state.mode !== 'cards')
}

function shortName(name) {
  return name.length > 26 ? name.slice(0, 24) + '…' : name
}

function esc(value) {
  return String(value === undefined || value === null ? '' : value)
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;').replace(/'/g, '&#39;')
}

// The previews are fetched when a card comes near the screen: a collection of a thousand pictures
// must not become a thousand requests at once.
const observer = ('IntersectionObserver' in window)
  ? new IntersectionObserver((entries) => {
    for (const entry of entries) {
      if (!entry.isIntersecting) continue
      observer.unobserve(entry.target)
      loadThumb(entry.target)
    }
  }, { rootMargin: '300px' })
  : null

function observeCard(card, item) {
  card.dataset.loaded = ''
  if (!observer) {
    loadThumb(card)
    return
  }
  observer.observe(card)
}

async function loadThumb(card) {
  if (card.dataset.loaded === '1') return
  card.dataset.loaded = '1'
  const doc = card.dataset.doc
  // The relations come with the preview: both are about the card a person is looking at.
  if (card._item) loadCardEdges(card, card._item)
  // An SVG is not a photograph: the preview endpoint renders pictures, so a vector is taken whole
  // and the browser draws it.
  const thumb = card.querySelector('.thumb') || card
  // The browser is given the address, not a blob: its own image cache, its own decoder off the main
  // thread, and a second look at the same card costs nothing at all (23.11.2026).
  const img = document.createElement('img')
  img.alt = card.title
  img.loading = 'lazy'
  img.decoding = 'async'
  img.addEventListener('error', () => pictureFallback(img, doc, 400, () => card.classList.add('broken')))
  thumb.innerHTML = ''
  thumb.appendChild(img)
  paintPicture(img, state.bucket, doc, 400, previewURL(doc, 400))
}

// --- the viewer --------------------------------------------------------------------------------

/** edgeIcons is the graph's vocabulary as a person scans it: the same pictures the old gallery used,
 *  so the same relation is recognised at a glance. */
const edgeIcons = {
  'иллюстрация': '📖',
  'сгенерировал': '🤖',
  'породил_идею': '💡',
  'заказал': '🍽',
  'был_в': '📍',
  'фото_в': '📸',
  'рассказ_о': '📖',
  'направил': '✉️',
  'упоминает': '🔖',
}

/** edgeLine turns one relation into one readable line. The relation keeps its own name — a graph has
 *  its own words — and the arrow says which way it reads: the picture first when the edge starts at
 *  it, the other end first when the picture is the one being talked about. */
function edgeLine(edge, name) {
  const icon = edgeIcons[edge.relation] || '↔'
  const rel = esc(edge.relation.replaceAll('_', ' '))
  const other = `<span class="who">${esc(edge.from === name ? edge.to : edge.from)}</span>`
  if (edge.from === name) return `${icon} <span class="rel">${rel}</span> → ${other}`
  return `${icon} ${other} → <span class="rel">${rel}</span>`
}

// edgeCache keeps what memory said about a picture while the viewer walks back and forth.
const edgeCache = new Map()

// The grid asks the memory about the relations of the cards it shows — four at a time, and once per
// card: a collection of two hundred pictures must not become two hundred requests at once.
const edgeQueue = []
let edgeRunning = 0

function edgeJob(fn) {
  return new Promise((resolve, reject) => {
    edgeQueue.push({ fn, resolve, reject })
    pumpEdges()
  })
}

function pumpEdges() {
  while (edgeRunning < 4 && edgeQueue.length > 0) {
    const job = edgeQueue.shift()
    edgeRunning++
    job.fn()
      .then(job.resolve, job.reject)
      .finally(() => { edgeRunning--; pumpEdges() })
  }
}

/** getEdges answers the relations of one picture, from the cache or from the module. */
async function getEdges(name) {
  // The whole answer is kept, not only the list: an empty list and a memory that did not answer are two
  // different things, and only the second one is worth a word (23.11.2026).
  if (edgeCache.has(name)) return edgeCache.get(name)
  const answer = await edgeJob(() => api('api/edges?name=' + encodeURIComponent(name)))
  edgeCache.set(name, answer)
  return answer
}

/** loadCardEdges puts the old gallery's «Связи» mark on a card: a picture the graph knows something
 *  about says so at a glance, instead of only after opening it. */
async function loadCardEdges(card, item) {
  if (!item || item.edges !== undefined) return
  try {
    const answer = await getEdges(item.name)
    const edges = (answer && answer.edges) || []
    item.edges = edges.length
    if (edges.length > 0 && card.isConnected) redrawInfo(card, item)
  } catch (e) {
    // A graph that did not answer is not a picture without relations; the card stays as it is.
    item.edges = 0
  }
}

/** showEdges draws the relations of the picture on screen. No relations is the normal case for a
 *  picture nobody has linked yet: the block does not appear at all, and nothing is red anywhere. Only a
 *  memory that did not answer says so, quietly (23.11.2026). */
async function showEdges(name) {
  const box = $('viewerEdges')
  box.hidden = true
  box.innerHTML = ''
  const draw = (edges) => {
    if (!edges || edges.length === 0) return false
    box.innerHTML = `<div class="head">🔗 ${esc(t('edges'))}</div>` +
      edges.map((e) => `<div class="edge">${edgeLine(e, name)}</div>`).join('')
    box.hidden = false
    return true
  }
  try {
    const answer = await getEdges(name)
    if (draw((answer && answer.edges) || [])) return
    if (answer && answer.unavailable) {
      box.innerHTML = `<div class="head">🔗 ${esc(t('edges'))}</div><div class="muted">${esc(t('edgesError'))}</div>`
      box.hidden = false
    }
  } catch (e) {
    box.innerHTML = `<div class="head">🔗 ${esc(t('edges'))}</div><div class="muted">${esc(t('edgesError'))}</div>`
    box.hidden = false
  }
}

let viewerIndex = -1

async function openViewer(item) {
  viewerIndex = state.items.findIndex((x) => x.doc === item.doc)
  await showViewer()
}

async function showViewer() {
  const item = state.items[viewerIndex]
  if (!item) return
  const box = $('viewer')
  box.hidden = false
  $('viewerName').textContent = item.name
  const img = $('viewerImage')
  const spinner = $('viewerSpinner')
  // While the picture is on its way the viewer shows a turnstile, not a broken image: a missing src
  // is drawn as a torn page and looks like a failure (22.11.2026).
  img.hidden = true
  img.removeAttribute('src')
  img.alt = item.name
  spinner.hidden = false
  delete img.dataset.fallback
  delete img.dataset.step
  img.onload = () => { spinner.hidden = true; img.hidden = false }
  img.onerror = () => pictureFallback(img, item.doc, 1400, () => {
    spinner.hidden = true
    img.hidden = false
    img.alt = `${item.name} — ${t('error')}`
  })
  paintPicture(img, state.bucket, item.doc, 1400, previewURL(item.doc, 1400))
  const original = $('viewerOriginal')
  original.href = fileURL(item.doc)
  original.hidden = false
  // The chapter this picture is in: a person who opened a picture to look at it often wants the story
  // it came from, and before this the only way back was to close the viewer and find the card
  // (23.11.2026).
  const story = $('viewerStory')
  story.hidden = !item.story_name
  if (item.story_name) {
    story.textContent = t('story')
    story.title = item.story || item.story_name
    story.dataset.name = item.story_name
  }
  showEdges(item.name)
  $('viewerPrev').disabled = viewerIndex <= 0
  $('viewerNext').disabled = viewerIndex >= state.items.length - 1 && state.done
}

function closeViewer() {
  const box = $('viewer')
  box.hidden = true
  const img = $('viewerImage')
  if (img.src.startsWith('blob:')) URL.revokeObjectURL(img.src)
  img.removeAttribute('src')
}

function stepViewer(delta) {
  const next = viewerIndex + delta
  if (next < 0) return
  if (next >= state.items.length) {
    if (!state.done) loadPage(false).then(() => { viewerIndex = next; showViewer() })
    return
  }
  viewerIndex = next
  showViewer()
}

/** togglePublic copies the picture into the public folder, or removes that copy. */
async function togglePublic(item, card) {
  try {
    const answer = item.public
      ? await api('api/public?doc=' + encodeURIComponent(item.doc), { method: 'DELETE' })
      : await api('api/public?doc=' + encodeURIComponent(item.doc), { method: 'POST' })
    item.public = !!answer.public
    item.public_url = answer.url || ''
    redrawInfo(card, item)
    hideNotice()
  } catch (e) {
    showNotice(e.message)
  }
}

/** trashPicture takes the picture out of the gallery; the file stays on disk. */
async function trashPicture(item, card) {
  if (!window.confirm(`${t('trash')}: ${item.name}?`)) return
  try {
    await api('api/trash?doc=' + encodeURIComponent(item.doc), { method: 'POST' })
    state.items = state.items.filter((x) => x.doc !== item.doc)
    card.remove()
    state.total = Math.max(0, state.total - 1)
    updateCounter()
    showEmptyIfNeeded()
    hideNotice()
  } catch (e) {
    showNotice(e.message)
  }
}

// --- what a person writes about a picture ------------------------------------------------------

let editingArticle = null

let editingMeta = null

async function openMeta(item) {
  editingMeta = item
  $('metaError').hidden = true
  $('metaSaved').hidden = true
  $('metaFile').textContent = item.name
  $('fDescription').value = ''
  $('fTags').value = ''
  $('fContext').value = ''
  $('fPrompt').value = ''
  $('fMemoir').value = ''
  $('meta').showModal()
  $('fDescription').focus()
  try {
    const answer = await api('api/meta?doc=' + encodeURIComponent(item.doc))
    const meta = (answer && answer.meta) || {}
    $('fDescription').value = item.description || meta.description || ''
    $('fTags').value = meta.tags || ''
    $('fContext').value = meta.context || ''
    $('fPrompt').value = meta.prompt || ''
    $('fMemoir').value = meta.memoir || ''
  } catch (e) {
    $('metaError').textContent = e.message
    $('metaError').hidden = false
  }
}

/** describeMeta asks the installation's vision model about the picture and puts its words into the
 *  description field: the same command the window's own «Описать моделью» button runs. */
async function describeMeta() {
  if (!editingMeta) return
  const error = $('metaError')
  const saved = $('metaSaved')
  const button = $('metaDescribe')
  error.hidden = true
  saved.hidden = true
  button.disabled = true
  button.classList.add('busy')
  button.textContent = t('describeWaiting')
  try {
    const answer = await api('api/describe?doc=' + encodeURIComponent(editingMeta.doc), { method: 'POST' })
    $('fDescription').value = answer.text || ''
    saved.textContent = `${t('describeDone')}${answer.model ? ' · ' + answer.model : ''}`
    saved.hidden = false
  } catch (e) {
    error.textContent = e.message
    error.hidden = false
  } finally {
    button.disabled = false
    button.classList.remove('busy')
    button.textContent = t('describeWithModel')
  }
}

async function saveMeta(event) {
  event.preventDefault()
  if (!editingMeta) return
  const error = $('metaError')
  const saved = $('metaSaved')
  error.hidden = true
  saved.hidden = true
  const body = {
    description: $('fDescription').value.trim(),
    tags: $('fTags').value.trim(),
    context: $('fContext').value.trim(),
    prompt: $('fPrompt').value.trim(),
    memoir: $('fMemoir').value.trim(),
  }
  const button = $('metaSave')
  button.disabled = true
  try {
    const answer = await api('api/meta?doc=' + encodeURIComponent(editingMeta.doc), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
    editingMeta.meta = body
    // The picture's description in the installation is what the card shows.
    editingMeta.description = body.description
    const card = [...document.querySelectorAll('.card')].find((c) => c.dataset.doc === editingMeta.doc)
    if (card) redrawInfo(card, editingMeta)
    // The copy in the installation is a separate result: a person must see when it did not happen.
    if (answer.index && answer.index !== 'ok' && answer.index !== 'нет описания') {
      saved.textContent = `${t('savedPartial')} ${answer.index}`
    } else {
      saved.textContent = t('savedToMemory')
    }
    saved.hidden = false
  } catch (e) {
    error.textContent = e.message
    error.hidden = false
  } finally {
    button.disabled = false
  }
}

async function deleteMeta() {
  if (!editingMeta) return
  if (!window.confirm(t('metaDelete') + '?')) return
  try {
    await api('api/meta?doc=' + encodeURIComponent(editingMeta.doc), { method: 'DELETE' })
    editingMeta.meta = {}
    const card = [...document.querySelectorAll('.card')].find((c) => c.dataset.doc === editingMeta.doc)
    if (card) redrawInfo(card, editingMeta)
    $('meta').close()
  } catch (e) {
    $('metaError').textContent = e.message
    $('metaError').hidden = false
  }
}

/** publication asks the installation where this article stands and paints the buttons:
 *  "Опубликовать" when it is not published, "Опубликовано · изменилось" when the file was edited
 *  after publishing, and "снять" while it is up. The state is the document's, not ours. */
async function refreshPublication(doc) {
  const publish = $('storyPublish')
  const published = $('storyPublished')
  const unpublish = $('storyUnpublish')
  published.hidden = true
  unpublish.hidden = true
  if (!doc) {
    publish.hidden = true
    return
  }
  publish.hidden = false
  publish.textContent = t('publish')
  try {
    const pub = await api('api/publish?doc=' + encodeURIComponent(doc))
    if (pub && pub.published && !pub.removed) {
      publish.hidden = true
      published.hidden = false
      published.href = pub.url
      published.title = t('publishedHint', { url: pub.url })
      published.querySelector('span').textContent = pub.changed ? t('publishedChanged') : t('published')
      unpublish.hidden = false
    }
  } catch {
    // A collection that is not publishable, or an installation without the command: no buttons
    // is a better answer than a button that cannot work.
    publish.hidden = true
  }
}

async function publishArticle() {
  if (!editingArticle || !editingArticle.doc) return
  if (!window.confirm(t('publishConfirm'))) return
  const publish = $('storyPublish')
  publish.disabled = true
  publish.textContent = '…'
  try {
    await api('api/publish?doc=' + encodeURIComponent(editingArticle.doc), { method: 'POST' })
    await refreshPublication(editingArticle.doc)
  } catch (e) {
    showNotice(e.message || String(e))
  } finally {
    publish.disabled = false
  }
}

async function unpublishArticle() {
  if (!editingArticle || !editingArticle.doc) return
  if (!window.confirm(t('unpublishConfirm'))) return
  try {
    await api('api/publish?doc=' + encodeURIComponent(editingArticle.doc), { method: 'DELETE' })
    await refreshPublication(editingArticle.doc)
  } catch (e) {
    showNotice(e.message || String(e))
  }
}

/** openArticleEditor shows the markdown of the chapter, as the file holds it. */
function openArticleEditor() {
  if (!editingArticle) return
  $('articleFile').textContent = editingArticle.name
  const text = $('articleText')
  text.value = editingArticle.source || ''
  $('articleError').hidden = true
  $('articleEditor').showModal()
  // A long chapter must open at its first line, not at the caret's end.
  text.setSelectionRange(0, 0)
  text.scrollTop = 0
  text.focus()
}

async function saveArticleEditor(event) {
  event.preventDefault()
  if (!editingArticle) return
  const error = $('articleError')
  error.hidden = true
  const button = $('articleSave')
  button.disabled = true
  try {
    await api('api/article', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: editingArticle.name, text: $('articleText').value }),
    })
    $('articleEditor').close()
    await openArticle(editingArticle.name)
  } catch (e) {
    error.textContent = e.message
    error.hidden = false
  } finally {
    button.disabled = false
  }
}

// --- settings ----------------------------------------------------------------------------------

/** openSettings draws the form from the module: the values in force and every collection of the
 *  installation with the folders behind it, so a person chooses instead of typing a name. */
async function openSettings() {
  const box = $('settings')
  $('settingsError').hidden = true
  $('settingsSave').disabled = true
  box.showModal()
  try {
    const answer = await api('api/settings')
    const chosen = new Set(answer.pictures || [])
    const collections = answer.collections || []
    $('settingsPictures').innerHTML = collections.map((c) => `
      <label class="collection-option">
        <input type="checkbox" value="${esc(c.id)}" ${chosen.has(c.id) ? 'checked' : ''} />
        <span class="collection-name">${esc(c.name || c.id)}</span>
        <span class="collection-path">${esc((c.folders || []).join(', '))}</span>
      </label>`).join('') || `<div class="empty">${esc(t('nothingFound'))}</div>`
    const options = [`<option value="">${esc(t('noArticles'))}</option>`].concat(
      collections.map((c) => `<option value="${esc(c.id)}" ${answer.articles === c.id ? 'selected' : ''}>${esc(c.name || c.id)}</option>`))
    $('settingsArticles').innerHTML = options.join('')
    $('settingsPublicBase').value = answer.public_base || ''
    $('settingsFile').textContent = answer.settings_file ? `${t('settingsFile')}: ${answer.settings_file}` : ''
  } catch (e) {
    $('settingsError').textContent = e.message
    $('settingsError').hidden = false
  } finally {
    $('settingsSave').disabled = false
  }
}

/** saveSettings sends the choice and applies it: the grid and the articles start reading the new
 *  collections at once, without a restart. */
async function saveSettings(event) {
  event.preventDefault()
  const error = $('settingsError')
  error.hidden = true
  const pictures = [...$('settingsPictures').querySelectorAll('input[type=checkbox]')]
    .filter((box) => box.checked).map((box) => box.value)
  const body = {
    pictures,
    articles: $('settingsArticles').value,
    public_base: $('settingsPublicBase').value.trim(),
  }
  const button = $('settingsSave')
  button.disabled = true
  try {
    await api('api/settings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
    $('settings').close()
    await loadConfig()
    state.q = ''
    $('search').value = ''
    // A page may be on its way while the settings change: the grid must be rebuilt from the new
    // collections, not left with the answer that was already travelling (26.11.2026).
    state.loading = false
    state.done = false
    showTab('pictures')
    await loadPage(true)
  } catch (e) {
    error.textContent = e.message
    error.hidden = false
  } finally {
    button.disabled = false
  }
}

// --- the bar -----------------------------------------------------------------------------------

function updateCounter() {
  if (state.tab === 'articles') {
    if (state.semantic) {
      $('counter').textContent = `${state.semantic} · ${t('byMeaning')}`
      return
    }
    $('counter').textContent = state.articleQ ? `${state.articleCount || 0} ${t('found')}` : ''
    return
  }
  if (state.semantic) {
    $('counter').textContent = `${state.semantic} · ${t('byMeaning')}`
    return
  }
  const total = state.total
  if (!total) {
    $('counter').textContent = ''
    return
  }
  // What is on the screen and what is in the collection are two numbers: "60 из 200" is the honest
  // answer, and it also says that scrolling brings more (22.11.2026).
  const loaded = state.items.length
  $('counter').textContent = loaded < total
    ? `${loaded} ${t('of')} ${total}`
    : `${total} ${t('picturesCount')}`
}

function showNotice(text) {
  const notice = $('notice')
  notice.textContent = text
  notice.hidden = false
}

function hideNotice() {
  $('notice').hidden = true
}

function showEmptyIfNeeded() {
  const empty = $('empty')
  if (state.items.length) {
    empty.hidden = true
    return
  }
  if (state.loading) return
  empty.hidden = false
  empty.textContent = state.q ? t('nothingFound') : (state.error ? `${t('error')}: ${state.error}` : t('empty'))
}

/** maybeMore asks for the next page when the end of the grid is near. */
function maybeMore() {
  if (state.semantic) return
  const s = $('scroller')
  if (!s || state.loading || state.done) return
  if (s.scrollTop + s.clientHeight >= s.scrollHeight - 600) loadPage(false)
}

function hideEmpty() {
  $('empty').hidden = true
}

function applyTexts() {
  document.documentElement.lang = lang
  document.title = t('gallery')
  for (const el of document.querySelectorAll('[data-i18n]')) el.textContent = t(el.dataset.i18n)
  for (const el of document.querySelectorAll('[data-i18n-placeholder]')) el.placeholder = t(el.dataset.i18nPlaceholder)
}

function bind() {
  $('refresh').addEventListener('click', () => {
    hideNotice()
    // «Обновить» clears the search and reloads the grid; the module drops its picture list with
    // refresh=1, which is how a file added outside the module appears without a restart.
    $('search').value = ''
    state.q = ''
    state.semantic = 0
    if (state.tab === 'articles') {
      loadArticles('')
      return
    }
    state.loading = false
    state.done = false
    loadPage(true, true)
  })
  // The toolbar behind one button on a phone; it folds back as soon as a choice is made.
  const topbar = document.querySelector('.topbar')
  const foldToolbar = () => topbar.classList.remove('more-open')
  $('moreToggle').addEventListener('click', () => topbar.classList.toggle('more-open'))
  $('mode').addEventListener('change', foldToolbar)
  $('sort').addEventListener('change', foldToolbar)
  $('refresh').addEventListener('click', () => setTimeout(foldToolbar, 300))
  document.addEventListener('keydown', (event) => {
    if (event.key === 'Escape' && topbar.classList.contains('more-open')) foldToolbar()
  })

  try {
    const storedMode = localStorage.getItem('anytextGallery.mode')
    if (storedMode === 'cards' || storedMode === 'compact') state.mode = storedMode
  } catch (e) { /* private mode */ }
  $('mode').value = state.mode
  applyMode()
  $('mode').addEventListener('change', (event) => {
    state.mode = event.target.value === 'cards' ? 'cards' : 'compact'
    try { localStorage.setItem('anytextGallery.mode', state.mode) } catch (e) { /* private mode */ }
    applyMode()
  })

  try {
    const stored = localStorage.getItem('anytextGallery.sort')
    if (stored === 'name' || stored === 'new') state.sort = stored
  } catch (e) { /* private mode */ }
  $('sort').value = state.sort
  $('sort').addEventListener('change', (event) => {
    state.sort = event.target.value === 'name' ? 'name' : 'new'
    try { localStorage.setItem('anytextGallery.sort', state.sort) } catch (e) { /* private mode */ }
    loadPage(true)
  })

  let timer = null
  $('search').addEventListener('input', (event) => {
    clearTimeout(timer)
    const value = event.target.value
    timer = setTimeout(() => runSearch(value), 250)
  })
  // Enter asks the meaning search: the words in a description are one thing, «верблюд на балконе» is
  // another (23.11.2026).
  $('search').addEventListener('keydown', (event) => {
    if (event.key !== 'Enter') return
    event.preventDefault()
    searchByMeaning()
  })

  $('metaForm').addEventListener('submit', saveMeta)
  $('metaDescribe').addEventListener('click', describeMeta)
  if (document.body.dataset.write === 'false') $('metaDescribe').hidden = true
  $('metaCancel').addEventListener('click', () => $('meta').close())
  $('metaDelete').addEventListener('click', deleteMeta)
  $('viewerClose').addEventListener('click', closeViewer)
  $('viewerStory').addEventListener('click', () => {
    const name = $('viewerStory').dataset.name
    if (!name) return
    closeViewer()
    showTab('articles')
    openArticle(name)
  })
  $('viewerPrev').addEventListener('click', () => stepViewer(-1))
  $('viewerNext').addEventListener('click', () => stepViewer(1))
  $('viewer').addEventListener('click', (event) => {
    if (event.target === $('viewer')) closeViewer()
  })
  window.addEventListener('keydown', (event) => {
    if ($('viewer').hidden) return
    if (event.key === 'Escape') closeViewer()
    if (event.key === 'ArrowLeft') stepViewer(-1)
    if (event.key === 'ArrowRight') stepViewer(1)
  })

  for (const tab of document.querySelectorAll('[data-tab]')) {
    tab.addEventListener('click', () => showTab(tab.dataset.tab))
  }
  $('storyBack').addEventListener('click', closeArticle)
  if (document.body.dataset.write === 'false') $('storyEdit').hidden = true
  $('storyEdit').addEventListener('click', openArticleEditor)
  $('storyPublish').addEventListener('click', publishArticle)
  $('storyUnpublish').addEventListener('click', unpublishArticle)
  $('articleForm').addEventListener('submit', saveArticleEditor)
  $('settingsBtn').addEventListener('click', openSettings)
  $('settingsForm').addEventListener('submit', saveSettings)
  $('settingsCancel').addEventListener('click', () => $('settings').close())
  if (document.body.dataset.write !== 'false') $('settingsBtn').hidden = false
  $('articleCancel').addEventListener('click', () => $('articleEditor').close())

  window.addEventListener('storage', (event) => {
    if (event.key === 'anytext-theme') applyTheme()
    if (event.key === 'anytext.lang') {
      lang = storedLang()
      applyTexts()
      updateCounter()
    }
  })

  // The next page is asked for when the grid is within a screen of its end. A scroll handler, not an
  // intersection observer: the observer fires once, while the first page is still loading, and never
  // again - the counter said 200 while the grid held 60 (22.11.2026).
  $('scroller').addEventListener('scroll', maybeMore, { passive: true })
  window.addEventListener('resize', maybeMore)
}

/** searchByMeaning asks the installation's meaning search for the tab on screen: pictures by what is
 *  drawn on them, chapters by what they are about. It costs about a second and a half — that is why it
 *  is asked on Enter, or when the words found nothing (23.11.2026). */
async function searchByMeaning() {
  const q = ($('search').value || '').trim()
  if (!q) return
  if (state.tab === 'articles') {
    const toc = $('toc')
    toc.innerHTML = `<div class="empty">${esc(t('searchingMeaning'))}</div>`
    try {
      const answer = await api('api/search?in=articles&q=' + encodeURIComponent(q))
      const list = answer.articles || []
      state.semantic = list.length
      state.articleCount = list.length
      toc.innerHTML = list.length
        ? `<h1>${esc(t('contents'))} · ${esc(t('byMeaning'))}</h1>`
        : `<div class="empty">${esc(t('meaningEmpty'))}</div>`
      for (const article of list) toc.appendChild(articleRow(article))
      updateCounter()
    } catch (e) {
      toc.innerHTML = `<div class="empty">${esc(t('articlesError'))}: ${esc(e.message)}</div>`
    }
    return
  }
  const grid = $('cards')
  grid.innerHTML = `<div class="empty">${esc(t('searchingMeaning'))}</div>`
  try {
    const answer = await api('api/search?in=pictures&q=' + encodeURIComponent(q))
    const items = answer.items || []
    state.items = items
    state.total = items.length
    state.semantic = items.length
    state.done = true
    grid.innerHTML = ''
    if (items.length === 0) {
      grid.innerHTML = `<div class="empty">${esc(t('meaningEmpty'))}</div>`
    } else {
      renderCards(items)
    }
    updateCounter()
  } catch (e) {
    grid.innerHTML = `<div class="empty">${esc(t('error'))}: ${esc(e.message)}</div>`
  }
}

/** runSearch is the local search of the tab on screen: the grid for pictures, the contents for the
 *  chapters. It is instant — no model, no installation call. */
function runSearch(value) {
  const q = (value || '').trim()
  state.semantic = 0
  if (state.tab === 'articles') {
    state.articleQ = q
    loadArticles(q)
    return
  }
  if (q === state.q) {
    if (!q) loadPage(true)
    return
  }
  state.q = q
  loadPage(true).then(() => {
    // Nothing by the words — try the meaning: the person typed what the picture is, not its name.
    if (q && state.items.length === 0) searchByMeaning()
  })
}

async function start() {
  const inFrame = window.self !== window.top
  if (!inFrame) {
    const params = new URLSearchParams(window.location.search)
    const qLang = params.get('lang')
    if (LANGS.includes(qLang)) lang = qLang
  }
  applyTheme()
  applyTexts()
  bind()
  await loadConfig()
  await loadPage(true)
}

start()

// --- the articles ------------------------------------------------------------------------------

/** formatDate draws an ISO date in the person's language; anything else is shown as written. */
function formatDate(value) {
  if (!value) return ''
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return value
  const d = new Date(value + 'T00:00:00')
  if (isNaN(d.getTime())) return value
  return d.toLocaleDateString(locale(), { day: 'numeric', month: 'long', year: 'numeric' })
}

/** showTab switches between the pictures and the articles of the memoirs. */
function showTab(tab) {
  state.tab = tab === 'articles' ? 'articles' : 'pictures'
  for (const button of document.querySelectorAll('[data-tab]')) {
    button.classList.toggle('active', button.dataset.tab === state.tab)
  }
  $('cards').hidden = state.tab !== 'pictures'
  $('toc').hidden = state.tab !== 'articles'
  $('story').hidden = true
  $('empty').hidden = true
  // The counter and the "by meaning" note belong to the tab: a new tab starts from its own search.
  state.semantic = 0
  updateCounter()
  $('search').placeholder = t(state.tab === 'articles' ? 'searchPlaceholderArticles' : 'searchPlaceholder')
  $('search').hidden = false
  $('sort').hidden = state.tab === 'articles'
  $('mode').hidden = state.tab === 'articles'
  if (state.tab === 'articles') loadArticles()
}

/** loadArticles draws the table of contents: newest chapter first, or what the query found in them. */
async function loadArticles(query) {
  const q = query === undefined ? state.articleQ : (query || '').trim()
  state.articleQ = q
  const toc = $('toc')
  toc.innerHTML = `<div class="empty">${esc(q ? t('searchingMeaning') : t('loading'))}</div>`
  try {
    const answer = await api('api/articles' + (q ? '?q=' + encodeURIComponent(q) : ''))
    const list = answer.articles || []
    state.articleCount = list.length
    if (!list.length) {
      toc.innerHTML = `<div class="empty">${esc(q ? t('meaningEmpty') : t('articlesEmpty'))}</div>`
      updateCounter()
      return
    }
    toc.innerHTML = `<h1>${esc(q ? t('found') : t('contents'))}</h1>`
    for (const article of list) {
      toc.appendChild(articleRow(article))
    }
    updateCounter()
    // Nothing by the words — ask the meaning search, the same way the grid does.
    if (q && list.length === 0) searchByMeaning()
  } catch (e) {
    toc.innerHTML = `<div class="empty">${esc(t('articlesError'))}: ${esc(e.message)}</div>`
  }
}

/** articleRow is one chapter in the contents: the title, «Автор · дата», the first line or the line a
 *  search found it in. */
function articleRow(article) {
  const row = document.createElement('button')
  row.type = 'button'
  row.className = 'toc-row'
  const date = formatDate(article.date)
  const meta = []
  if (article.author) meta.push(`<span><b>${esc(t('author'))}:</b> ${esc(article.author)}</span>`)
  if (date) meta.push(`<span><b>${esc(date)}</b></span>`)
  row.innerHTML = `<span class="toc-title">${esc(article.title)}</span>` +
    (meta.length ? `<span class="toc-meta">${meta.join(' · ')}</span>` : '') +
    (article.snippet ? `<span class="toc-snippet">${esc(article.snippet)}</span>`
      : (article.description ? `<span class="toc-desc">${esc(article.description)}</span>` : ''))
  row.addEventListener('click', () => openArticle(article.name))
  return row
}

/** openArticle shows one chapter; its pictures are loaded through the same viewer as the grid. */
async function openArticle(name) {
  const story = $('story')
  $('toc').hidden = true
  $('cards').hidden = true
  story.hidden = false
  $('storyTitle').textContent = ''
  $('storyMeta').textContent = ''
  $('storyBody').innerHTML = `<div class="empty">${esc(t('loading'))}</div>`
  $('scroller').scrollTop = 0
  try {
    const page = await api('api/article?name=' + encodeURIComponent(name) + (relayed() ? '&media=1' : ''))
    if (page.media) state.media = page.media
    $('storyTitle').textContent = page.title || name
    const date = formatDate(page.date)
    $('storyMeta').textContent = [date, page.place].filter(Boolean).join(' · ')
    editingArticle = page
    refreshPublication(page.doc)
    $('storyBody').innerHTML = page.html || ''
    const docs = page.docs || {}
    for (const img of $('storyBody').querySelectorAll('img[data-gallery]')) {
      const picture = img.dataset.gallery
      const doc = docs[picture]
      if (!doc) continue
      img.dataset.doc = doc
      img.classList.add('story-picture')
      img.addEventListener('click', () => {
        const item = { doc, name: picture, story_name: name, story: $('storyTitle').textContent }
        state.items = [item]
        viewerIndex = 0
        showViewer()
      })
      // A chapter shows previews, not originals: two pictures used to pull three megabytes of PNG and
      // decode both at full size (23.11.2026).
      img.loading = 'lazy'
      paintPicture(img, state.bucket, doc, 1400, previewURL(doc, 1400))
      img.addEventListener('error', () => pictureFallback(img, doc, 1400, () => {
        img.alt = `${picture} — ${t('error')}`
      }))
    }
    for (const link of $('storyBody').querySelectorAll('a[data-gallery]')) {
      const doc = docs[link.dataset.gallery]
      if (!doc) continue
      link.addEventListener('click', (event) => {
        event.preventDefault()
        const item = { doc, name: link.dataset.gallery, story_name: name, story: $('storyTitle').textContent }
        state.items = [item]
        viewerIndex = 0
        showViewer()
      })
    }
  } catch (e) {
    $('storyBody').innerHTML = `<div class="empty">${esc(e.message)}</div>`
  }
}

function closeArticle() {
  $('story').hidden = true
  $('toc').hidden = false
}
