;(function () {
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
    if (!btn.classList.contains('mixer-control__toggle')) return

    var kind = btn.dataset.controlKind
    var cardId = btn.dataset.cardId
    var controlName = btn.dataset.controlName

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

    var cards = payload.state.Cards
    Object.keys(cards).forEach(function (cardId) {
      var cardState = cards[cardId]
      if (!cardState || !cardState.Controls) return
      var controls = cardState.Controls
      Object.keys(controls).forEach(function (controlName) {
        var state = controls[controlName]
        if (!state) return
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

    source.addEventListener('volume-change', function (event) {
      var data = JSON.parse(event.data || '{}')
      if (!data) return
      updateVolume(data.card, data.control, data.volume)
    })

    source.addEventListener('mute-change', function (event) {
      var data = JSON.parse(event.data || '{}')
      if (!data) return
      updateMute(data.card, data.control, data.muted)
    })

    source.addEventListener('mixer-update', function (event) {
      var data = JSON.parse(event.data || '{}')
      handleMixerUpdate(data)
    })
  }

  function setupHTMXToggleHandlers() {
    document.body.addEventListener('htmx:afterRequest', function (event) {
      var btn = event.target
      if (btn && btn.classList.contains('mixer-control__toggle')) {
        handleToggleResponse(btn)
      }
    })
  }

  document.addEventListener('DOMContentLoaded', function () {
    setupSSE()
    setupHTMXToggleHandlers()
  })
})()
