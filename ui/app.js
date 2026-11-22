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
    picturesCount: 'картинок', articlesSoon: 'Статьи — следующий шаг',
    install: 'установка', collection: 'коллекция',
  },
  en: {
    gallery: 'Gallery', pictures: 'Pictures', articles: 'Articles',
    searchPlaceholder: 'search by name', refresh: 'Refresh',
    loading: 'Loading…', empty: 'No pictures in this collection yet',
    nothingFound: 'Nothing found', error: 'The pictures did not arrive',
    picturesCount: 'pictures', articlesSoon: 'Articles are the next step',
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
    const q = new URLSearchParams({ offset: String(state.offset), limit: String(LIMIT) })
    if (state.q) q.set('q', state.q)
    const page = await api('api/images?' + q.toString())
    state.total = page.total
    state.offset = page.offset + page.items.length
    state.done = page.items.length === 0 || state.offset >= page.total
    state.items = state.items.concat(page.items)
    renderCards(page.items)
    updateCounter()
    showEmptyIfNeeded()
  } catch (e) {
    state.error = e.message
    showNotice(`${t('error')}: ${e.message}`)
  } finally {
    state.loading = false
  }
}

function renderCards(items) {
  const grid = $('cards')
  for (const item of items) {
    const card = document.createElement('button')
    card.type = 'button'
    card.className = 'card'
    card.dataset.doc = item.doc
    card.title = item.name
    card.innerHTML = `<span class="ph">${esc(shortName(item.name))}</span>`
    card.addEventListener('click', () => openViewer(item))
    grid.appendChild(card)
    observeCard(card, item)
  }
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
    card.innerHTML = ''
    card.appendChild(img)
  } catch (e) {
    card.classList.add('broken')
  }
}

// --- the viewer --------------------------------------------------------------------------------

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
  img.removeAttribute('src')
  img.alt = item.name
  try {
    const res = await runFetch(() => fetch(fileURL(item.doc)))
    if (!res.ok) throw new Error(res.status)
    const blob = await res.blob()
    img.src = URL.createObjectURL(blob)
  } catch (e) {
    img.alt = `${item.name} — ${t('error')}`
  }
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

// --- the bar -----------------------------------------------------------------------------------

function updateCounter() {
  const total = state.total
  $('counter').textContent = total ? `${total} ${t('picturesCount')}` : ''
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

  let timer = null
  $('search').addEventListener('input', (event) => {
    clearTimeout(timer)
    const value = event.target.value
    timer = setTimeout(() => search(value), 250)
  })

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
    tab.addEventListener('click', () => {
      if (tab.dataset.tab === 'articles') {
        showNotice(t('articlesSoon'))
        return
      }
      for (const other of document.querySelectorAll('[data-tab]')) other.classList.toggle('active', other === tab)
    })
  }

  window.addEventListener('storage', (event) => {
    if (event.key === 'anytext-theme') applyTheme()
    if (event.key === 'anytext.lang') {
      lang = storedLang()
      applyTexts()
      updateCounter()
    }
  })

  // The end of the grid asks for the next page; nothing else scrolls it.
  const sentinel = $('sentinel')
  if ('IntersectionObserver' in window) {
    new IntersectionObserver((entries) => {
      if (entries.some((e) => e.isIntersecting)) loadPage(false)
    }, { rootMargin: '600px' }).observe(sentinel)
  }
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
