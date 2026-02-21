;(function () {
  function clamp(value, min, max) {
    return value < min ? min : value > max ? max : value
  }

  function parseIntAttr(el, name, fallback) {
    var raw = el.getAttribute(name)
    var n = raw == null ? NaN : parseInt(raw, 10)
    return isNaN(n) ? fallback : n
  }

  function parseFloatAttr(el, name, fallback) {
    var raw = el.getAttribute(name)
    var n = raw == null ? NaN : parseFloat(raw)
    return isNaN(n) ? fallback : n
  }

  function syncSliderUI(slider, volume) {
    var min = parseIntAttr(slider, 'aria-valuemin', 0)
    var max = parseIntAttr(slider, 'aria-valuemax', 100)
    var clamped = clamp(volume, min, max)

    slider.setAttribute('aria-valuenow', String(clamped))
    slider.setAttribute('aria-valuetext', clamped + '%')

    // Drive the ASCII bar via the CSS custom property
    slider.style.setProperty('--volume-percent', clamped + '%')

    var valueEl = slider.querySelector('.mixer-control__value')
    if (valueEl) {
      valueEl.textContent = String(clamped)
    }
  }

  function initSlider(slider) {
    var current = parseIntAttr(slider, 'aria-valuenow', 0)
    syncSliderUI(slider, current)
  }

  function initSlidersIn(root) {
    var scope = root || document
    var sliders = scope.querySelectorAll('.mixer-control__volume[role="slider"]')
    for (var i = 0; i < sliders.length; i++) {
      initSlider(sliders[i])
    }
  }

  function updateFromPointer(slider, event) {
    var track = slider.querySelector('.mixer-control__volume-track')
    if (!track) return

    var rect = track.getBoundingClientRect()
    if (!rect || rect.width === 0 || rect.height === 0) return

    var vertical = rect.height > rect.width
    var ratio = 0
    if (vertical) {
      var y = event.clientY
      ratio = 1 - (y - rect.top) / rect.height
    } else {
      var x = event.clientX
      ratio = (x - rect.left) / rect.width
    }
    ratio = clamp(ratio, 0, 1)

    var min = parseIntAttr(slider, 'aria-valuemin', 0)
    var max = parseIntAttr(slider, 'aria-valuemax', 100)
    
    // Get step size from data attribute (calculated from ALSA range)
    var step = parseFloatAttr(slider, 'data-volume-step', 1)
    if (step <= 0) step = 1
    
    // Calculate raw value and round to nearest step
    var raw = min + ratio * (max - min)
    var volume = Math.round(raw / step) * step
    volume = clamp(Math.round(volume), min, max)

    syncSliderUI(slider, volume)
  }

  var activeSlider = null
  
  // Throttling for server updates during drag
  var lastSendTime = 0
  var THROTTLE_MS = 100

  function sendVolumeThrottled() {
    if (!activeSlider) return
    
    var now = Date.now()
    if (now - lastSendTime < THROTTLE_MS) return
    
    lastSendTime = now
    
    if (typeof htmx !== 'undefined') {
      var card = activeSlider.dataset.cardId
      var control = activeSlider.dataset.controlName
      var volume = activeSlider.getAttribute('aria-valuenow')
      htmx.ajax('POST', '/control/volume', {
        values: { card: card, control: control, volume: volume },
        swap: 'none'
      })
    }
  }

  function handlePointerDown(event) {
    var slider = event.target.closest('.mixer-control__volume[role="slider"]')
    if (!slider) return

    activeSlider = slider
    if (typeof slider.setPointerCapture === 'function') {
      try {
        slider.setPointerCapture(event.pointerId)
      } catch (e) {}
    }

    updateFromPointer(slider, event)
  }

  function handlePointerMove(event) {
    if (!activeSlider) return
    updateFromPointer(activeSlider, event)
    // Send throttled update to server during drag
    sendVolumeThrottled()
  }

  function clearPointerCapture(event) {
    if (!activeSlider) return

    updateFromPointer(activeSlider, event)

    if (typeof activeSlider.releasePointerCapture === 'function') {
      try {
        activeSlider.releasePointerCapture(event.pointerId)
      } catch (e) {}
    }

    // Final update to ensure server has latest value
    if (typeof htmx !== 'undefined') {
      var card = activeSlider.dataset.cardId
      var control = activeSlider.dataset.controlName
      var volume = activeSlider.getAttribute('aria-valuenow')
      htmx.ajax('POST', '/control/volume', {
        values: { card: card, control: control, volume: volume },
        swap: 'none'
      })
    }

    activeSlider = null
  }

  function handleKeyDown(event) {
    var slider = event.target.closest('.mixer-control__volume[role="slider"]')
    if (!slider) return

    if (
      event.key !== 'ArrowLeft' &&
      event.key !== 'ArrowRight' &&
      event.key !== 'ArrowUp' &&
      event.key !== 'ArrowDown'
    ) {
      return
    }

    event.preventDefault()

    // Use step size for keyboard navigation too
    var step = parseFloatAttr(slider, 'data-volume-step', 2)
    if (step <= 0) step = 2
    
    var min = parseIntAttr(slider, 'aria-valuemin', 0)
    var max = parseIntAttr(slider, 'aria-valuemax', 100)
    var current = parseIntAttr(slider, 'aria-valuenow', 0)
    var delta = step
    if (event.key === 'ArrowLeft' || event.key === 'ArrowDown') {
      delta = -step
    }
    var next = clamp(current + delta, min, max)

    syncSliderUI(slider, next)

    // Trigger HTMX request to update volume on server
    if (typeof htmx !== 'undefined') {
      var card = slider.dataset.cardId
      var control = slider.dataset.controlName
      var volume = slider.getAttribute('aria-valuenow')
      htmx.ajax('POST', '/control/volume', {
        values: { card: card, control: control, volume: volume },
        swap: 'none'
      })
    }
  }

  document.addEventListener('DOMContentLoaded', function () {
    initSlidersIn(document)

    document.addEventListener('pointerdown', handlePointerDown)
    document.addEventListener('pointermove', handlePointerMove)
    document.addEventListener('pointerup', clearPointerCapture)
    document.addEventListener('pointercancel', clearPointerCapture)
    document.addEventListener('keydown', handleKeyDown)
  })

  document.body && document.body.addEventListener('htmx:afterSwap', function (event) {
    if (event && event.target) {
      initSlidersIn(event.target)
    }
  })
})()
