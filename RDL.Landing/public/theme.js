// Aplica el tema antes de pintar (sin parpadeo): el elegido con el botón o, si no hay, el del sistema.
// Es un archivo y no código en línea: la CSP no necesita 'unsafe-inline'.
;(function () {
  var theme = null
  try {
    theme = localStorage.getItem('rdl.theme')
  } catch (e) {}
  if (theme !== 'light' && theme !== 'dark') {
    theme = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  }
  document.documentElement.setAttribute('data-theme', theme)
})()
