(function () {
  var header = document.querySelector('.masthead');
  var toggle = header.querySelector('.nav-toggle');
  var mobile = window.matchMedia('(max-width: 850px)');
  function close() {
    header.classList.remove('nav-open');
    toggle.setAttribute('aria-expanded', 'false');
    toggle.setAttribute('aria-label', 'Open navigation');
  }
  toggle.hidden = false;
  header.classList.add('nav-ready');
  toggle.addEventListener('click', function () {
    var open = toggle.getAttribute('aria-expanded') !== 'true';
    header.classList.toggle('nav-open', open);
    toggle.setAttribute('aria-expanded', String(open));
    toggle.setAttribute('aria-label', open ? 'Close navigation' : 'Open navigation');
  });
  document.addEventListener('keydown', function (event) {
    if (event.key === 'Escape' && header.classList.contains('nav-open')) {
      close();
      toggle.focus();
    }
  });
  document.addEventListener('click', function (event) {
    if (!header.contains(event.target)) close();
  });
  mobile.addEventListener('change', close);
}());
