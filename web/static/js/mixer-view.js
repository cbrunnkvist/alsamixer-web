;(function () {
  function toArray(list) {
    return Array.prototype.slice.call(list || [])
  }

  function visibleControls(card) {
    var controls = toArray(card.querySelectorAll('.mixer-control'))
    return controls.filter(function (control) {
      return !control.classList.contains('is-filtered')
    })
  }

  function updateEmptyState(card, view) {
    var empty = card.querySelector('.mixer-card__empty')
    if (!empty) return

    var controls = visibleControls(card)
    if (controls.length === 0) {
      var label = 'No controls in this view.'
      if (view === 'playback') {
        label = 'No playback controls in this view.'
      } else if (view === 'capture') {
        label = 'No capture controls in this view.'
      }
      empty.textContent = label
      empty.classList.add('is-visible')
      return
    }

    empty.textContent = ''
    empty.classList.remove('is-visible')
  }

  function setActiveIndex(card, index) {
    var controls = visibleControls(card)
    if (controls.length === 0) {
      card.removeAttribute('data-active-index')
      return
    }

    var next = index
    if (next < 0) next = controls.length - 1
    if (next >= controls.length) next = 0
    card.setAttribute('data-active-index', String(next))

    for (var i = 0; i < controls.length; i++) {
      controls[i].classList.toggle('is-active', i === next)
    }

    var label = card.querySelector('[data-active-label]')
    if (label) {
      label.textContent = controls[next].getAttribute('data-control-name') || ''
    }
  }

  function setView(card, view) {
    card.setAttribute('data-current-view', view)
    var buttons = toArray(card.querySelectorAll('[data-view]'))
    for (var i = 0; i < buttons.length; i++) {
      var match = buttons[i].getAttribute('data-view') === view
      buttons[i].setAttribute('aria-pressed', match ? 'true' : 'false')
    }

    var controls = toArray(card.querySelectorAll('.mixer-control'))
    for (var j = 0; j < controls.length; j++) {
      var controlView = controls[j].getAttribute('data-control-view') || 'playback'
      var show = view === 'all' || controlView === view
      controls[j].classList.toggle('is-filtered', !show)
    }

    updateEmptyState(card, view)
    setActiveIndex(card, 0)
  }

  function applyCompact(card) {
    var compact = window.matchMedia('(max-width: 720px)').matches
    card.classList.toggle('is-compact', compact)
  }

  function initCard(card) {
    var defaultView = card.getAttribute('data-current-view') || 'playback'
    setView(card, defaultView)

    var viewButtons = toArray(card.querySelectorAll('[data-view]'))
    for (var i = 0; i < viewButtons.length; i++) {
      viewButtons[i].addEventListener('click', function (event) {
        var nextView = event.currentTarget.getAttribute('data-view')
        if (nextView) {
          setView(card, nextView)
        }
      })
    }

    var navButtons = toArray(card.querySelectorAll('[data-nav]'))
    for (var j = 0; j < navButtons.length; j++) {
      navButtons[j].addEventListener('click', function (event) {
        var direction = event.currentTarget.getAttribute('data-nav')
        var current = parseInt(card.getAttribute('data-active-index') || '0', 10)
        var next = direction === 'prev' ? current - 1 : current + 1
        setActiveIndex(card, next)
      })
    }

    applyCompact(card)
  }

  function initAll(root) {
    var cards = toArray((root || document).querySelectorAll('.mixer-card'))
    for (var i = 0; i < cards.length; i++) {
      initCard(cards[i])
    }
  }

  document.addEventListener('DOMContentLoaded', function () {
    initAll(document)
    window.addEventListener('resize', function () {
      var cards = toArray(document.querySelectorAll('.mixer-card'))
      for (var i = 0; i < cards.length; i++) {
        applyCompact(cards[i])
      }
    })
  })

  if (document.body) {
    document.body.addEventListener('htmx:afterSwap', function (event) {
      if (event && event.target) {
        initAll(event.target)
      }
    })
  }
})()
