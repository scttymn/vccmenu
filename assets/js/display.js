(function () {
  'use strict';
  var board = document.getElementById('board');
  var columns = document.getElementById('columns');
  var rail = document.getElementById('rail');
  var endpoint = '/api/menu';
  var cacheKey = 'vcc-board-v2:/api/menu';
  var legacyKey = 'vcc-menu-v1:/api/menu';
  var stored = null;
  var paintedLeft = '';
  var paintedPanel = '';
  var tick = 0;

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
  function renderColumns(sections) {
    var fragment = document.createDocumentFragment();
    sections.forEach(function (section) {
      var el = node('section', 'section');
      el.appendChild(node('h2', '', section.name));
      if (section.note) el.appendChild(node('p', 'note', section.note));
      var list = node('ul', 'items');
      (section.items || []).forEach(function (item) {
        if (item.visible === false) return;
        var row = node('li', 'item');
        row.appendChild(node('span', 'item-name', item.name));
        row.appendChild(node('span', 'price', item.price));
        list.appendChild(row);
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
    columns.textContent = ''; columns.appendChild(fragment);
  }
  function renderRail(panel) {
    rail.textContent = '';
    if (!panel) {
      rail.hidden = true;
      board.classList.remove('has-rail');
      return;
    }
    rail.hidden = false;
    board.classList.add('has-rail');
    rail.appendChild(node('p', 'rail-kicker', 'ORDER BY NUMBER'));
    rail.appendChild(node('h2', 'rail-name', panel.name || ''));
    (panel.combos || []).forEach(function (combo) {
      var row = node('article', 'combo');
      row.appendChild(node('div', 'combo-number', String(combo.number)));
      var body = node('div', '');
      var top = node('div', 'combo-top');
      top.appendChild(node('p', 'combo-name', combo.name || ''));
      top.appendChild(node('p', 'combo-price', combo.price || ''));
      body.appendChild(top);
      body.appendChild(node('p', 'combo-contents', combo.contents || ''));
      if (combo.individual) body.appendChild(node('p', 'combo-individual', combo.individual));
      if (combo.savings) body.appendChild(node('p', 'combo-savings', combo.savings));
      row.appendChild(body);
      rail.appendChild(row);
    });
  }
  function paint(snapshot, span) {
    var leftKey = JSON.stringify(snapshot.left);
    var panel = span ? span.panel : null;
    var panelKey = JSON.stringify(panel || null);
    var changed = false;
    if (leftKey !== paintedLeft) {
      renderColumns(snapshot.left);
      paintedLeft = leftKey;
      changed = true;
    }
    if (!span) panelKey = paintedPanel;
    if (panelKey !== paintedPanel) {
      renderRail(panel);
      paintedPanel = panelKey;
      changed = true;
    }
    document.getElementById('brand').textContent = snapshot.brand || '';
    document.getElementById('title').textContent = snapshot.title || '';
    document.body.classList.add('ready');
    if (changed) fit();
  }
  function paintLegacy(menu) {
    renderColumns(menu.sections);
    renderRail(null);
    paintedLeft = JSON.stringify(menu.sections);
    paintedPanel = 'null';
    document.getElementById('brand').textContent = menu.brand || '';
    document.getElementById('title').textContent = menu.title || '';
    document.body.classList.add('ready');
    fit();
  }
  function spanContaining(timeline, shopNow) {
    if (!timeline) return null;
    for (var i = 0; i < timeline.length; i++) {
      var span = timeline[i];
      var start = Date.parse(span.start);
      var end = Date.parse(span.end);
      if (start <= shopNow && shopNow < end) return span;
    }
    return null;
  }
  function clockOffset(value) {
    return Date.parse(value.snapshot.serverNow) - value.fetchedAt;
  }
  function shopTime(value) {
    var at = new URLSearchParams(location.search).get('at');
    if (at) {
      var frozen = Date.parse(at);
      if (!isNaN(frozen)) return frozen;
    }
    return Date.now() + clockOffset(value);
  }
  function arm() {
    clearTimeout(tick);
    if (!stored) return;
    var now = shopTime(stored);
    var span = spanContaining(stored.snapshot.timeline, now);
    paint(stored.snapshot, span);
    var wait = 60000;
    if (span) {
      var until = Date.parse(span.end) - now;
      if (until > 0 && until < wait) wait = until;
    }
    tick = setTimeout(arm, wait);
  }
  function validSnapshot(snapshot) {
    return snapshot && Array.isArray(snapshot.left) && snapshot.left.length === 3 && Array.isArray(snapshot.timeline);
  }
  function recall() {
    try {
      var raw = localStorage.getItem(cacheKey);
      if (!raw) return null;
      var value = JSON.parse(raw);
      if (!value || typeof value.fetchedAt !== 'number' || !validSnapshot(value.snapshot)) return null;
      return value;
    } catch (e) { return null; }
  }
  function recallLegacy() {
    try {
      var raw = localStorage.getItem(legacyKey);
      if (!raw) return null;
      var menu = JSON.parse(raw);
      if (!menu || !Array.isArray(menu.sections) || menu.sections.length !== 3) return null;
      return menu;
    } catch (e) { return null; }
  }
  var pending = false;
  function refresh() {
    if (pending) return;
    pending = true;
    var xhr = new XMLHttpRequest();
    xhr.open('GET', endpoint + (endpoint.indexOf('?') === -1 ? '?' : '&') + 't=' + Date.now());
    xhr.timeout = 15000;
    xhr.onload = function () {
      try {
        if (xhr.status !== 200) throw new Error('Unavailable');
        var snapshot = JSON.parse(xhr.responseText);
        if (!validSnapshot(snapshot)) throw new Error('Invalid menu');
        stored = { fetchedAt: Date.now(), snapshot: snapshot };
        try { localStorage.setItem(cacheKey, JSON.stringify(stored)); } catch (e) {}
        arm();
      } catch (e) {
        if (!document.body.classList.contains('ready')) document.getElementById('loading').textContent = 'Connecting to menu…';
      }
    };
    xhr.onloadend = function () { pending = false; };
    xhr.send();
  }
  stored = recall();
  if (stored) arm();
  else {
    var legacy = recallLegacy();
    if (legacy) paintLegacy(legacy);
  }
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