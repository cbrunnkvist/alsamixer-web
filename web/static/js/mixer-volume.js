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

  // Quantize a value to the nearest valid step
  function quantizeToStep(value, min, max, step) {
    if (step <= 0 || step >= 100) return value
    // Round to nearest step
    var quantized = Math.round(value / step) * step
    return clamp(Math.round(quantized), min, max)
  }

  function initSlider(slider) {
    var current = parseIntAttr(slider, 'aria-valuenow', 0)
    var step = parseFloatAttr(slider, 'data-volume-step', 1)
    var min = parseIntAttr(slider, 'aria-valuemin', 0)
    var max = parseIntAttr(slider, 'aria-valuemax', 100)
    // Quantize on init too
    var quantized = quantizeToStep(current, min, max, step)
    syncSliderUI(slider, quantized)
  }

  function initSlidersIn(root, skipActive) {
    var scope = root || document
    var sliders = scope.querySelectorAll('.mixer-control__volume[role="slider"]')
    for (var i = 0; i < sliders.length; i++) {
      var slider = sliders[i]
      // Skip the slider that's currently being dragged or recently modified
      if (skipActive) {
        var sliderId = getControlId(slider)
        if (slider === activeSlider || sliderId === recentlyModifiedControl) {
          continue
        }
      }
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
    
    // Calculate raw value and quantize to nearest valid step
    var raw = min + ratio * (max - min)
    var volume = quantizeToStep(raw, min, max, step)

    syncSliderUI(slider, volume)
  }

  var activeSlider = null
  var recentlyModifiedControl = null // Track by control identity (cardId + controlName)
  var MODIFIED_COOLDOWN_MS = 1000
  
  // Helper to get control identity for a slider
  function getControlId(slider) {
    return slider.dataset.cardId + '|' + slider.dataset.controlName
  }
  
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
    // Mark this slider as actively being dragged
    slider.classList.add('volume-slider--dragging')
    
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
    
    // Remove dragging marker
    activeSlider.classList.remove('volume-slider--dragging')

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
    
    // Mark this control as recently modified to prevent SSE from overriding it
    recentlyModifiedControl = getControlId(activeSlider)
    var prevControlId = recentlyModifiedControl
    activeSlider = null
    
    // Clear the cooldown after a short delay
    setTimeout(function() {
      if (recentlyModifiedControl === prevControlId) {
        recentlyModifiedControl = null
      }
    }, MODIFIED_COOLDOWN_MS)
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
    initSlidersIn(document, false)

    document.addEventListener('pointerdown', handlePointerDown)
    document.addEventListener('pointermove', handlePointerMove)
    document.addEventListener('pointerup', clearPointerCapture)
    document.addEventListener('pointercancel', clearPointerCapture)
    document.addEventListener('keydown', handleKeyDown)
  })

  document.body && document.body.addEventListener('htmx:afterSwap', function (event) {
    // After SSE swap, reinit sliders but skip the one being dragged or recently modified
    if (event && event.target) {
      initSlidersIn(event.target, true)
    }
  })
})()
