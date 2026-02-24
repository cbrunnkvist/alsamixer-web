;(function () {
  // Initialize window.app for debug toggle (preserve existing value if set)
  window.app = window.app || {}
  if (window.app.debugLogging === undefined) {
    window.app.debugLogging = false
  }

  // Debug logging - toggle with window.app.debugLogging = true
  var debug = {
    log: function() {
      if (!(window.app && window.app.debugLogging)) return
      var args = Array.prototype.slice.call(arguments)
      // Build copyable string version first (for easy copy)
      var copyable = args.map(function(a) {
        if (a && typeof a === 'object' && !(a instanceof String)) {
          try { return JSON.stringify(a) } catch(e) { return String(a) }
        }
        return String(a)
      }).join(' ')
      // Log: copyable string first (visible in log preview), then interactive objects
      console.debug.apply(console, [copyable].concat(args))
    }
  }

  // Track recently modified controls to prevent SSE from overwriting user's active interaction
  var recentlyModifiedControl = null
  var MODIFIED_COOLDOWN_MS = 1000

  function getControlId(cardId, controlName) {
    return cardId + '|' + controlName
  }

  function shouldSkipUpdate(controlName) {
    if (!recentlyModifiedControl) return false
    // Check if this control matches (handle "Master" vs "Master Volume" variations)
    var normalized = function(n) { return n ? n.replace(' Volume', '').toLowerCase() : '' }
    var incoming = normalized(controlName)
    var modified = normalized(recentlyModifiedControl.split('|')[1])
    return incoming === modified
  }

  // Expose for mixer-volume.js to call when user interacts
  window.app.setRecentlyModified = function(cardId, controlName) {
    recentlyModifiedControl = getControlId(cardId, controlName)
    debug.log('[SSE] set recentlyModifiedControl:', recentlyModifiedControl)
    setTimeout(function() {
      recentlyModifiedControl = null
    }, MODIFIED_COOLDOWN_MS)
  }

  window.app.getRecentlyModified = function() {
    return recentlyModifiedControl
  }

  function toArray(list) {
    return Array.prototype.slice.call(list || [])
  }

  function findControl(cardId, controlName) {
    var controls = toArray(document.querySelectorAll('.mixer-control[data-card-id]'))
    for (var i = 0; i < controls.length; i++) {
      if (
        controls[i].getAttribute('data-card-id') === String(cardId) &&
        controls[i].getAttribute('data-control-name') === controlName
      ) {
        return controls[i]
      }
    }
    return null
  }

  function updateVolume(cardId, controlName, volume) {
    if (shouldSkipUpdate(controlName)) {
      debug.log('[SSE] skipping volume update for recently modified:', controlName)
      return
    }

    var control = findControl(cardId, controlName)
    if (!control) return

    var slider = control.querySelector('.mixer-control__volume[role="slider"]')
    if (!slider) return

    var clamped = Math.max(0, Math.min(100, parseInt(volume, 10) || 0))
    slider.setAttribute('aria-valuenow', String(clamped))
    slider.setAttribute('aria-valuetext', clamped + '%')
    slider.style.setProperty('--volume-percent', clamped + '%')

    var valueEl = slider.querySelector('.mixer-control__value')
    if (valueEl) {
      valueEl.textContent = String(clamped)
    }
  }

  function updateMute(cardId, controlName, muted) {
    if (shouldSkipUpdate(controlName)) {
      debug.log('[SSE] skipping mute update for recently modified:', controlName)
      return
    }

    var control = findControl(cardId, controlName)
    if (!control) return

    var toggle = control.querySelector('.mixer-control__toggle--mute')
    if (!toggle) return

    var next = !!muted
    toggle.setAttribute('aria-checked', next ? 'true' : 'false')

    var label = toggle.querySelector('.mixer-control__toggle-label')
    if (label) {
      label.textContent = next ? 'Muted' : 'Unmuted'
    }

    var srText = toggle.querySelector('.sr-only')
    if (srText) {
      var name = toggle.dataset.controlName || ''
      srText.textContent = next ? 'Mute enabled for ' + name + '.' : 'Mute disabled for ' + name + '.'
    }
  }

  function updateCapture(cardId, controlName, active) {
    if (shouldSkipUpdate(controlName)) {
      debug.log('[SSE] skipping capture update for recently modified:', controlName)
      return
    }

    var control = findControl(cardId, controlName)
    if (!control) return

    var toggle = control.querySelector('.mixer-control__toggle--capture')
    if (!toggle) return

    var next = !!active
    toggle.setAttribute('aria-checked', next ? 'true' : 'false')

    var label = toggle.querySelector('.mixer-control__toggle-label')
    if (label) {
      label.textContent = next ? 'Capture On' : 'Capture Off'
    }

    var srText = toggle.querySelector('.sr-only')
    if (srText) {
      var name = toggle.dataset.controlName || ''
      srText.textContent = next ? 'Capture enabled for ' + name + '.' : 'Capture disabled for ' + name + '.'
    }
  }

  function handleToggleResponse(btn) {
    if (!btn || !btn.classList || !btn.dataset) return
    if (!btn.classList.contains('mixer-control__toggle')) return

    var kind = btn.dataset.controlKind
    var cardId = btn.dataset.cardId
    var controlName = btn.dataset.controlName

    debug.log('[HTMX toggle response]', kind, cardId, controlName)

    var current = btn.getAttribute('aria-checked') === 'true'
    var next = !current
    btn.setAttribute('aria-checked', next ? 'true' : 'false')

    var label = btn.querySelector('.mixer-control__toggle-label')
    if (label) {
      if (kind === 'mute') {
        label.textContent = next ? 'Muted' : 'Unmuted'
      } else if (kind === 'capture') {
        label.textContent = next ? 'Capture On' : 'Capture Off'
      }
    }

    var srText = btn.querySelector('.sr-only')
    if (srText && controlName) {
      if (kind === 'mute') {
        srText.textContent = next ? 'Mute enabled for ' + controlName + '.' : 'Mute disabled for ' + controlName + '.'
      } else if (kind === 'capture') {
        srText.textContent = next ? 'Capture enabled for ' + controlName + '.' : 'Capture disabled for ' + controlName + '.'
      }
    }
  }

  function handleMixerUpdate(payload) {
    if (!payload || !payload.state || !payload.state.Cards) return

    var isFromHandler = payload.source === 'handler'
    var changedControl = payload.control

    // If we have a recently modified control, skip ALL updates from non-handler sources
    // This prevents stale monitor state from overwriting user's active changes
    var skipAllExceptHandler = !isFromHandler && recentlyModifiedControl

    var cards = payload.state.Cards
    Object.keys(cards).forEach(function (cardId) {
      var cardState = cards[cardId]
      if (!cardState || !cardState.Controls) return
      var controls = cardState.Controls
      Object.keys(controls).forEach(function (controlName) {
        var state = controls[controlName]
        if (!state) return

        // Skip if this is from handler and matches recently modified
        if (isFromHandler && shouldSkipUpdate(controlName)) {
          debug.log('[SSE mixer-update] skipping handler update for:', controlName)
          return
        }

        // Skip ALL updates from monitor while user interaction is pending
        if (skipAllExceptHandler) {
          debug.log('[SSE mixer-update] skipping monitor update due to recent user interaction')
          return
        }

        if (Array.isArray(state.Volume) && state.Volume.length) {
          updateVolume(cardId, controlName, state.Volume[0])
        }
        if (typeof state.Mute === 'boolean') {
          updateMute(cardId, controlName, state.Mute)
        }
      })
    })
  }

  function setupSSE() {
    var source = new EventSource('/events')

    // Connection status handling
    var statusEl = document.getElementById('connection-status')
    source.onopen = function() {
      debug.log('[SSE] ✅ connected')
      if (statusEl) {
        statusEl.classList.remove('is-disconnected')
        var valueEl = statusEl.querySelector('[data-connection-state]')
        if (valueEl) valueEl.textContent = '✅ Connected'
      }
    }
    source.onerror = function() {
      debug.log('[SSE] ❌ disconnected')
      if (statusEl) {
        statusEl.classList.add('is-disconnected')
        var valueEl = statusEl.querySelector('[data-connection-state]')
        if (valueEl) valueEl.textContent = '❌ Disconnected'
      }
    }

    // Handle control-update events (from HTMX POST responses - other clients' changes)
    // These come with HTML payload for hx-swap-oob OR JSON for JS clients
    source.addEventListener('control-update', function (event) {
      var raw = event.data || ''
      debug.log('[SSE control-update]', raw.substring(0, 100))
      // If it starts with '<', it's HTML from hx-swap-oob - we ignore it since we're using JS-only
      // If it's JSON, parse it and handle like mixer-update
      if (raw.charAt(0) === '<') {
        debug.log('[SSE control-update] HTML payload ignored (using JS-only)')
        return
      }
      try {
        var data = JSON.parse(raw)
        handleMixerUpdate(data)
      } catch (e) {
        debug.log('[SSE control-update] failed to parse:', e)
      }
    })

    // Handle mixer-update events (from ALSA monitor - external changes)
    source.addEventListener('mixer-update', function (event) {
      var data = JSON.parse(event.data || '{}')
      debug.log('[SSE mixer-update]', data)
      handleMixerUpdate(data)
    })

    // Handle config-change events
    source.addEventListener('config-change', function (event) {
      var data = JSON.parse(event.data || '{}')
      debug.log('[SSE config-change]', data)
      // Could reload page or update UI for config changes
    })

    // Fallback: handle any unnamed messages
    source.onmessage = function (event) {
      debug.log('[SSE message]', event.data)
      try {
        var data = JSON.parse(event.data || '{}')
        handleMixerUpdate(data)
      } catch (e) {}
    }
  }

  function setupHTMXToggleHandlers() {
    document.body.addEventListener('htmx:afterRequest', function (event) {
      var btn = event.target
      if (btn && btn.classList && btn.classList.contains('mixer-control__toggle')) {
        handleToggleResponse(btn)
      }
    })
  }

  document.addEventListener('DOMContentLoaded', function () {
    setupSSE()
    setupHTMXToggleHandlers()
  })
})()
