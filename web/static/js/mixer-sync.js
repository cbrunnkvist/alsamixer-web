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

  document.addEventListener('DOMContentLoaded', function () {
    setupSSE()
  })
})()
