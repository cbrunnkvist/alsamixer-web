;(function () {
  function clamp(value, min, max) {
    return value < min ? min : value > max ? max : value
  }

  function parseIntAttr(el, name, fallback) {
    var raw = el.getAttribute(name)
    var n = raw == null ? NaN : parseInt(raw, 10)
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
    var volume = Math.round(min + ratio * (max - min))

    syncSliderUI(slider, volume)
  }

  var activeSlider = null

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
  }

  function clearPointerCapture(event) {
    if (!activeSlider) return

    updateFromPointer(activeSlider, event)

    if (typeof activeSlider.releasePointerCapture === 'function') {
      try {
        activeSlider.releasePointerCapture(event.pointerId)
      } catch (e) {}
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

    var step = 2
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
