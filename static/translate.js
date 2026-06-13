// Translate only the nostr post body (the [itemprop="articleBody"] element)
// into the visitor's browser language, using Google's free (keyless) endpoint.
// Self-contained: injects its own styles so no Tailwind rebuild is needed.
(function () {
  var style = document.createElement('style')
  style.textContent =
    '.njump-translate-btn:hover{background:#e32a6d;color:#fff;}' +
    '.njump-translation{margin-top:0.75rem;border-left:3px solid #e32a6d;' +
    'padding-left:0.75rem;white-space:pre-wrap;line-height:1.5rem;}'
  document.head.appendChild(style)

  function targetLang() {
    return navigator.language || navigator.userLanguage || 'en'
  }

  function translate(text, target) {
    var url =
      'https://translate.googleapis.com/translate_a/single?client=gtx&sl=auto&tl=' +
      encodeURIComponent(target) +
      '&dt=t&q=' +
      encodeURIComponent(text)
    return fetch(url)
      .then(function (r) {
        if (!r.ok) throw new Error('HTTP ' + r.status)
        return r.json()
      })
      .then(function (data) {
        return ((data && data[0]) || [])
          .map(function (s) {
            return s && s[0] ? s[0] : ''
          })
          .join('')
      })
  }

  document.addEventListener('click', function (ev) {
    var btn = ev.target.closest && ev.target.closest('.njump-translate-btn')
    if (!btn) return
    ev.preventDefault()

    var content = document.querySelector('[itemprop="articleBody"]')
    if (!content) return

    var existing = document.querySelector('.njump-translation')
    if (existing) {
      existing.remove()
      btn.textContent = btn.getAttribute('data-label-show')
      return
    }

    var text = (content.innerText || content.textContent || '').trim()
    if (!text) return

    btn.disabled = true
    btn.textContent = btn.getAttribute('data-label-loading') || '…'

    translate(text, targetLang())
      .then(function (translated) {
        var box = document.createElement('div')
        box.className = 'njump-translation'
        box.lang = targetLang()
        box.setAttribute('dir', 'auto')
        box.textContent = translated
        content.insertAdjacentElement('afterend', box)
        btn.textContent = btn.getAttribute('data-label-hide') || 'Show original'
      })
      .catch(function (err) {
        console.error('njump translate:', err)
        btn.textContent = btn.getAttribute('data-label-error') || 'Translation failed'
        window.setTimeout(function () {
          btn.textContent = btn.getAttribute('data-label-show')
        }, 2500)
      })
      .finally(function () {
        btn.disabled = false
      })
  })
})()
