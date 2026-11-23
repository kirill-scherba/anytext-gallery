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
    searchPlaceholder: 'поиск по имени', refresh: 'Обновить',
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
    backToArticles: 'к статьям', story: 'Рассказ', articlesEmpty: 'Статей пока нет', author: 'Автор', contents: 'Оглавление',
    editArticle: 'Редактировать',
    describeWithModel: 'Описать моделью',
    describeWaiting: 'Модель смотрит на картинку…',
    describeDone: 'Описание получено — проверьте и сохраните',
    edges: 'Связи',
    edgesNone: 'связей нет',
    edgesError: 'связи не получены',
    articleHint: 'Markdown сохраняется в файл главы как есть; картинку достаточно указать именем файла: ![подпись](img_123.png).',
    articleSaved: 'Глава сохранена',
    articlesError: 'Статьи не пришли',
    savedToMemory: 'Сохранено',
    savedPartial: 'Сохранено в память, но не в установку:', articlesSoon: 'Статьи — следующий шаг',
    install: 'установка', collection: 'коллекция',
  },
  en: {
    gallery: 'Gallery', pictures: 'Pictures', articles: 'Articles',
    searchPlaceholder: 'search by name', refresh: 'Refresh',
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
    backToArticles: 'back to the articles', story: 'Story', articlesEmpty: 'No articles yet', author: 'Author', contents: 'Contents',
    editArticle: 'Edit',
    describeWithModel: 'Describe with a model',
    describeWaiting: 'The model is looking at the picture…',
    describeDone: 'The description is here — read it and save',
    edges: 'Connections',
    edgesNone: 'no connections',
    edgesError: 'connections did not come',
    articleHint: 'The markdown is saved into the chapter file as is; a picture is named by its file: ![alt](img_123.png).',
    articleSaved: 'The chapter is saved',
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
  b.pathname = b.pathname.replace(/api\/module\/gallery\/?$/, '').replace(/\/+$/, '')
  return b.origin + b.pathname
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
}

const LIMIT = 60
// One preview at a time is a grid that fills slowly; six is a burst a small machine can carry.
const FETCH_SLOTS = 6
let activeFetches = 0
const pendingFetches = []

/** runFetch keeps the number of parallel preview requests bounded and ordered. */
function runFetch(fn) {
  return new Promise((resolve, reject) => {
    const start = () => {
      activeFetches++
      fn().then(resolve, reject).finally(() => {
        activeFetches--
        const next = pendingFetches.shift()
        if (next) next()
      })
    }
    if (activeFetches < FETCH_SLOTS) start()
    else pendingFetches.push(start)
  })
}

/** thumbURL is the installation's preview of one picture. */
function thumbURL(doc, max) {
  const q = new URLSearchParams({ bucket: state.bucket, doc, max: String(max) })
  return `${installBase()}/api/thumb?${q.toString()}`
}

/** fileURL is the picture itself, not compressed. */
function fileURL(doc) {
  const q = new URLSearchParams({ bucket: state.bucket, doc })
  return `${installBase()}/api/file?${q.toString()}`
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

async function loadPage(reset) {
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
    const page = await api('api/images?' + q.toString())
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
  const vector = /\.svg$/i.test(doc)
  try {
    const res = await runFetch(() => fetch(vector ? fileURL(doc) : thumbURL(doc, 400)))
    if (!res.ok) throw new Error(res.status)
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const img = document.createElement('img')
    img.src = url
    img.alt = card.title
    img.loading = 'lazy'
    img.addEventListener('load', () => URL.revokeObjectURL(url))
    const thumb = card.querySelector('.thumb') || card
    thumb.innerHTML = ''
    thumb.appendChild(img)
  } catch (e) {
    card.classList.add('broken')
  }
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
  if (edgeCache.has(name)) return edgeCache.get(name)
  const answer = await edgeJob(() => api('api/edges?name=' + encodeURIComponent(name)))
  const edges = answer.edges || []
  edgeCache.set(name, edges)
  return edges
}

/** loadCardEdges puts the old gallery's «Связи» mark on a card: a picture the graph knows something
 *  about says so at a glance, instead of only after opening it. */
async function loadCardEdges(card, item) {
  if (!item || item.edges !== undefined) return
  try {
    const edges = await getEdges(item.name)
    item.edges = edges.length
    if (edges.length > 0 && card.isConnected) redrawInfo(card, item)
  } catch (e) {
    // A graph that did not answer is not a picture without relations; the card stays as it is.
    item.edges = 0
  }
}

/** showEdges asks the module for the relations of the picture on screen and draws them under it. */
async function showEdges(name) {
  const box = $('viewerEdges')
  box.hidden = true
  box.innerHTML = ''
  const draw = (edges) => {
    if (!edges || edges.length === 0) return
    box.innerHTML = `<div class="head">🔗 ${esc(t('edges'))}</div>` +
      edges.map((e) => `<div class="edge">${edgeLine(e, name)}</div>`).join('')
    box.hidden = false
  }
  try {
    draw(await getEdges(name))
  } catch (e) {
    // A graph that did not answer is not a picture without relations: say so, quietly, and let the
    // picture stay the thing on screen.
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
  try {
    const res = await runFetch(() => fetch(fileURL(item.doc)))
    if (!res.ok) throw new Error(res.status)
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    img.onload = () => { spinner.hidden = true; img.hidden = false }
    img.src = url
  } catch (e) {
    spinner.hidden = true
    img.hidden = false
    img.alt = `${item.name} — ${t('error')}`
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

// --- the bar -----------------------------------------------------------------------------------

function updateCounter() {
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
  $('refresh').addEventListener('click', () => { hideNotice(); search('') })

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
    timer = setTimeout(() => search(value), 250)
  })

  $('metaForm').addEventListener('submit', saveMeta)
  $('metaDescribe').addEventListener('click', describeMeta)
  if (document.body.dataset.write === 'false') $('metaDescribe').hidden = true
  $('metaCancel').addEventListener('click', () => $('meta').close())
  $('metaDelete').addEventListener('click', deleteMeta)
  $('viewerClose').addEventListener('click', closeViewer)
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
  $('articleForm').addEventListener('submit', saveArticleEditor)
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

function search(value) {
  const q = (value || '').trim()
  if (q === state.q) {
    if (!q) loadPage(true)
    return
  }
  state.q = q
  loadPage(true)
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
  $('search').hidden = state.tab === 'articles'
  $('sort').hidden = state.tab === 'articles'
  $('mode').hidden = state.tab === 'articles'
  if (state.tab === 'articles') loadArticles()
}

/** loadArticles draws the table of contents: newest chapter first. */
async function loadArticles() {
  const toc = $('toc')
  toc.innerHTML = `<div class="empty">${esc(t('loading'))}</div>`
  try {
    const answer = await api('api/articles')
    const list = answer.articles || []
    if (!list.length) {
      toc.innerHTML = `<div class="empty">${esc(t('articlesEmpty'))}</div>`
      return
    }
    toc.innerHTML = `<h1>${esc(t('contents'))}</h1>`
    for (const article of list) {
      const row = document.createElement('button')
      row.type = 'button'
      row.className = 'toc-row'
      const date = formatDate(article.date)
      const meta = []
      if (article.author) meta.push(`<span><b>${esc(t('author'))}:</b> ${esc(article.author)}</span>`)
      if (date) meta.push(`<span><b>${esc(date)}</b></span>`)
      row.innerHTML = `<span class="toc-title">${esc(article.title)}</span>` +
        (meta.length ? `<span class="toc-meta">${meta.join(' · ')}</span>` : '') +
        (article.description ? `<span class="toc-desc">${esc(article.description)}</span>` : '')
      row.addEventListener('click', () => openArticle(article.name))
      toc.appendChild(row)
    }
  } catch (e) {
    toc.innerHTML = `<div class="empty">${esc(t('articlesError'))}: ${esc(e.message)}</div>`
  }
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
    const page = await api('api/article?name=' + encodeURIComponent(name))
    $('storyTitle').textContent = page.title || name
    const date = formatDate(page.date)
    $('storyMeta').textContent = [date, page.place].filter(Boolean).join(' · ')
    editingArticle = page
    $('storyBody').innerHTML = page.html || ''
    const docs = page.docs || {}
    for (const img of $('storyBody').querySelectorAll('img[data-gallery]')) {
      const picture = img.dataset.gallery
      const doc = docs[picture]
      if (!doc) continue
      img.dataset.doc = doc
      img.classList.add('story-picture')
      img.addEventListener('click', () => {
        const item = { doc, name: picture }
        state.items = [item]
        viewerIndex = 0
        showViewer()
      })
      try {
        const res = await runFetch(() => fetch(fileURL(doc)))
        if (res.ok) img.src = URL.createObjectURL(await res.blob())
      } catch (e) { /* a picture that did not come is a picture without a box */ }
    }
    for (const link of $('storyBody').querySelectorAll('a[data-gallery]')) {
      const doc = docs[link.dataset.gallery]
      if (!doc) continue
      link.addEventListener('click', (event) => {
        event.preventDefault()
        const item = { doc, name: link.dataset.gallery }
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
