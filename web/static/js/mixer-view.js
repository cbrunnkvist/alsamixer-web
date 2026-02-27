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

    // Update nav button visibility
    var prevBtn = card.querySelector('[data-nav="prev"]')
    var nextBtn = card.querySelector('[data-nav="next"]')
    if (prevBtn && nextBtn) {
      if (controls.length <= 1) {
        prevBtn.style.visibility = 'hidden'
        nextBtn.style.visibility = 'hidden'
      } else {
        prevBtn.style.visibility = next === 0 ? 'hidden' : 'visible'
        nextBtn.style.visibility = next === controls.length - 1 ? 'hidden' : 'visible'
      }
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
    // Compact mode (carousel) for viewports narrower than desktop
    var compact = window.matchMedia('(max-width: 959px)').matches
    card.classList.toggle('is-compact', compact)

    // Lock body scroll if any card is in compact mode (carousel view)
    var anyCompact = document.querySelector('.mixer-card.is-compact') !== null
    document.documentElement.classList.toggle('is-locked', anyCompact)
  }

  function debounce(func, wait) {
    var timeout
    return function () {
      var context = this
      var args = arguments
      var later = function () {
        timeout = null
        func.apply(context, args)
      }
      clearTimeout(timeout)
      timeout = setTimeout(later, wait)
    }
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

    // New navigation logic for scrolling container
    var controlsContainer = card.querySelector('.mixer-card__controls')
    var navButtons = toArray(card.querySelectorAll('[data-nav]'))

    for (var j = 0; j < navButtons.length; j++) {
      navButtons[j].addEventListener('click', function (event) {
        var direction = event.currentTarget.getAttribute('data-nav')
        var controls = visibleControls(card)
        var currentIdx = parseInt(card.getAttribute('data-active-index') || '0', 10)
        var nextIdx = currentIdx + (direction === 'next' ? 1 : -1)

        if (nextIdx >= 0 && nextIdx < controls.length) {
          var targetControl = controls[nextIdx]
          // Calculate scroll position based on index (each control is 100% width in carousel)
          var controlWidth = controlsContainer.offsetWidth
          var scrollLeft = nextIdx * controlWidth
          
          // Directly scroll the container without affecting the page
          controlsContainer.scrollTo({
            left: scrollLeft,
            behavior: 'smooth'
          })

          // Immediately update state. This is essential for themes that don't use 
          // a horizontal scroll track (like Linux Console) where no scroll event will fire.
          // For scrolling themes, this provides immediate feedback while the scroll animates.
          setActiveIndex(card, nextIdx)
        }
      })
    }

    var onScroll = debounce(function () {
      var containerCenter = controlsContainer.scrollLeft + (controlsContainer.offsetWidth / 2)
      var controls = visibleControls(card)
      var closest = { index: -1, distance: Infinity }

      for (var i = 0; i < controls.length; i++) {
        var controlCenter = controls[i].offsetLeft + (controls[i].offsetWidth / 2)
        var distance = Math.abs(containerCenter - controlCenter)
        if (distance < closest.distance) {
          closest.index = i
          closest.distance = distance
        }
      }

      if (closest.index !== -1) {
        var currentActive = parseInt(card.getAttribute('data-active-index') || '-1', 10)
        if (currentActive !== closest.index) {
          setActiveIndex(card, closest.index)
        }
      }
    }, 150)

    controlsContainer.addEventListener('scroll', onScroll)


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
