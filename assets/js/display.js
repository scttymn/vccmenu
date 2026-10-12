(function () {
  'use strict';
  var board = document.getElementById('board');
  var columns = document.getElementById('columns');
	var endpoint = '/api/menu';
  var cacheKey = 'vcc-menu-v1:' + endpoint;
  var last = '';
  function node(tag, className, value) {
    var el = document.createElement(tag); el.className = className || '';
    if (value !== undefined) el.textContent = value;
    return el;
  }
  function fit() {
    var height = Math.max(board.scrollHeight, 1080);
    var width = window.innerWidth, available = window.innerHeight;
    var scale = Math.min(width / 1920, available / height);
    board.style.transform = 'scale(' + scale + ')';
    board.style.left = ((width - 1920 * scale) / 2) + 'px';
    board.style.top = ((available - height * scale) / 2) + 'px';
    board.style.visibility = 'visible';
  }
  function render(menu) {
    if (!menu || !Array.isArray(menu.sections) || menu.sections.length !== 3) throw new Error('Invalid menu');
    var serialized = JSON.stringify(menu);
    if (serialized === last) return;
    var fragment = document.createDocumentFragment();
    menu.sections.forEach(function (section) {
      var el = node('section', 'section');
      el.appendChild(node('h2', '', section.name));
      if (section.note) el.appendChild(node('p', 'note', section.note));
      var list = node('ul', 'items');
      section.items.forEach(function (item) {
        if (item.visible === false) return;
        var row = node('li', 'item');
        row.appendChild(node('span', 'item-name', item.name));
        row.appendChild(node('span', 'price', item.price)); list.appendChild(row);
      });
      el.appendChild(list);
      if (section.footer || (section.extras && section.extras.length)) {
        var bottom = node('div', 'bottom');
        if (section.footer) bottom.appendChild(node('p', 'footer', section.footer));
        var extras = node('ul', 'extras');
        (section.extras || []).forEach(function (extra) { extras.appendChild(node('li', '', extra)); });
        bottom.appendChild(extras); el.appendChild(bottom);
      }
      fragment.appendChild(el);
    });
    document.getElementById('brand').textContent = menu.brand;
    document.getElementById('title').textContent = menu.title;
    columns.textContent = ''; columns.appendChild(fragment);
    last = serialized; document.body.classList.add('ready'); fit();
    try { localStorage.setItem(cacheKey, serialized); } catch (_) {}
  }
  var pending = false;
  function refresh() {
    if (pending) return;
    pending = true;
    var xhr = new XMLHttpRequest();
    xhr.open('GET', endpoint + (endpoint.indexOf('?') === -1 ? '?' : '&') + 't=' + Date.now());
    xhr.timeout = 15000;
    xhr.onload = function () {
      try { if (xhr.status !== 200) throw new Error('Unavailable'); render(JSON.parse(xhr.responseText)); }
      catch (_) { if (!last) document.getElementById('loading').textContent = 'Connecting to menu…'; }
    };
    xhr.onloadend = function () { pending = false; };
    xhr.send();
  }
  try { var saved = localStorage.getItem(cacheKey); if (saved) render(JSON.parse(saved)); } catch (_) {}
  refresh(); setInterval(refresh, 30000);
  window.addEventListener('resize', fit);
  document.addEventListener('fullscreenchange', fit);
  document.addEventListener('visibilitychange', function () { if (!document.hidden) refresh(); });
  function fullscreen() {
    var root = document.documentElement;
    var action = document.fullscreenElement ? document.exitFullscreen && document.exitFullscreen() : root.requestFullscreen && root.requestFullscreen();
    if (action && action.catch) action.catch(function () {});
  }
  document.getElementById('screen').addEventListener('click', fullscreen);
  document.addEventListener('keydown', function (e) { if (e.key === 'Enter' || e.key === 'f') fullscreen(); });
}());
