(function () {
  'use strict';
  var form = document.getElementById('menu-form'), save = document.getElementById('save');
  var error = document.getElementById('error'), status = document.getElementById('status');
  var retry = document.getElementById('retry');
  var dialog = document.getElementById('menu-dialog');
  var board, selected, dirty = false, saving = false, dialogMode = 'new';
  var DAYS = [['sun', 'Sun'], ['mon', 'Mon'], ['tue', 'Tue'], ['wed', 'Wed'], ['thu', 'Thu'], ['fri', 'Fri'], ['sat', 'Sat']];

  function fail(message) { error.textContent = message; error.hidden = false; }
  function changed() {
    dirty = true; save.disabled = false;
    document.getElementById('save-label').textContent = 'You have unsaved changes';
    status.textContent = 'Save when you’re ready to update the TV.';
  }
  function current() {
    if (board.allDay && board.allDay.id === selected) return board.allDay;
    for (var i = 0; i < board.dayparts.length; i++) {
      if (board.dayparts[i].id === selected) return board.dayparts[i];
    }
    selected = board.allDay.id;
    return board.allDay;
  }
  function allDay() { return current() === board.allDay; }
  function prepareMenu(menu) {
    menu.specials = menu.specials || [];
    (menu.sections || []).forEach(function (section) {
      section.items = section.items || [];
      section.extras = section.extras || [];
    });
    return menu;
  }
  function prepareBoard(value) {
    value.dayparts = value.dayparts || [];
    prepareMenu(value.allDay);
    value.dayparts.forEach(prepareMenu);
    return value;
  }
  function count() {
    var menu = current();
    var total = menu.sections.reduce(function (n, section) {
      return n + section.items.filter(function (item) { return item.visible; }).length;
    }, 0);
    document.getElementById('summary').textContent = (menu.name || 'Menu') + ' · ' + total + ' items on the TV · ' + menu.specials.length + ' specials';
  }
  function emptySections() {
    return [0, 1, 2].map(function () { return { name: '', note: '', footer: '', extras: [], items: [] }; });
  }
  function newID() {
    var bytes = new Uint8Array(16);
    if (window.crypto && crypto.getRandomValues) crypto.getRandomValues(bytes);
    else for (var i = 0; i < bytes.length; i++) bytes[i] = Math.floor(Math.random() * 256);
    var hex = '';
    for (var j = 0; j < bytes.length; j++) hex += ('0' + bytes[j].toString(16)).slice(-2);
    return hex;
  }
  function todayIn(timeZone) {
    try {
      return new Intl.DateTimeFormat('en-CA', { timeZone: timeZone || 'America/Chicago', year: 'numeric', month: '2-digit', day: '2-digit' }).format(new Date());
    } catch (e) {
      return new Date().toISOString().slice(0, 10);
    }
  }
  function field(text, input) {
    var label = document.createElement('label');
    label.appendChild(document.createTextNode(text));
    label.appendChild(input);
    return label;
  }
  function clockInput(key, required) {
    var input = document.createElement('input');
    input.type = 'time';
    input.step = '60';
    input.dataset.clock = key;
    input.required = !!required;
    return input;
  }
  function dateInput(key) {
    var input = document.createElement('input');
    input.type = 'date';
    input.dataset.date = key;
    input.required = true;
    return input;
  }
  function buildWhen(root, radioName) {
    root.textContent = '';
    var kinds = document.createElement('div');
    kinds.className = 'kind-choice';
    [['weekly', 'Every week'], ['oneOff', 'Dates']].forEach(function (pair) {
      var label = document.createElement('label');
      var input = document.createElement('input');
      input.type = 'radio';
      input.name = radioName;
      input.value = pair[0];
      label.appendChild(input);
      label.appendChild(document.createTextNode(pair[1]));
      kinds.appendChild(label);
    });
    root.appendChild(kinds);
    var weekly = document.createElement('div');
    weekly.dataset.panel = 'weekly';
    var days = document.createElement('div');
    days.className = 'day-row';
    DAYS.forEach(function (day) {
      var label = document.createElement('label');
      label.className = 'day-check';
      var input = document.createElement('input');
      input.type = 'checkbox';
      input.dataset.day = day[0];
      input.value = day[0];
      label.appendChild(input);
      label.appendChild(document.createTextNode(day[1]));
      days.appendChild(label);
    });
    weekly.appendChild(days);
    var times = document.createElement('div');
    times.className = 'schedule-grid';
    times.appendChild(field('Starts', clockInput('start', true)));
    times.appendChild(field('Ends', clockInput('end', false)));
    weekly.appendChild(times);
    var weeklyHint = document.createElement('p');
    weeklyHint.className = 'menu-note';
    weeklyHint.textContent = 'Leave the end empty to finish at midnight. That time is not included.';
    weekly.appendChild(weeklyHint);
    root.appendChild(weekly);
    var once = document.createElement('div');
    once.dataset.panel = 'oneOff';
    var dates = document.createElement('div');
    dates.className = 'schedule-grid';
    dates.appendChild(field('First day', dateInput('first')));
    dates.appendChild(field('Last day', dateInput('last')));
    dates.appendChild(field('Starts', clockInput('from', false)));
    dates.appendChild(field('Ends', clockInput('to', false)));
    once.appendChild(dates);
    var onceHint = document.createElement('p');
    onceHint.className = 'menu-note';
    onceHint.textContent = 'Empty times cover the whole day, including the last day.';
    once.appendChild(onceHint);
    root.appendChild(once);
  }
  function clockValue(value) {
    if (!value || value === '24:00') return '';
    return value;
  }
  function applyWhen(root, when) {
    var kind = when && when.kind === 'oneOff' ? 'oneOff' : 'weekly';
    root.querySelectorAll('input[type=radio]').forEach(function (input) { input.checked = input.value === kind; });
    var days = (when && when.days) || [];
    root.querySelectorAll('[data-day]').forEach(function (input) { input.checked = days.indexOf(input.value) !== -1; });
    root.querySelector('[data-clock=start]').value = clockValue(when && when.start);
    root.querySelector('[data-clock=end]').value = clockValue(when && when.end);
    root.querySelector('[data-date=first]').value = (when && when.first) || '';
    root.querySelector('[data-date=last]').value = (when && when.last) || '';
    root.querySelector('[data-clock=from]').value = clockValue(when && when.from);
    root.querySelector('[data-clock=to]').value = clockValue(when && when.to);
    syncPanels(root);
  }
  function syncPanels(root) {
    var selectedKind = root.querySelector('input[type=radio]:checked');
    var kind = selectedKind ? selectedKind.value : 'weekly';
    root.querySelectorAll('[data-panel]').forEach(function (panel) {
      var on = panel.dataset.panel === kind;
      panel.hidden = !on;
      panel.querySelectorAll('input').forEach(function (input) { input.disabled = !on; });
    });
  }
  function readWhen(root) {
    var selectedKind = root.querySelector('input[type=radio]:checked');
    var kind = selectedKind ? selectedKind.value : 'weekly';
    if (kind === 'weekly') {
      var days = [];
      root.querySelectorAll('[data-day]:checked').forEach(function (input) { days.push(input.value); });
      var end = root.querySelector('[data-clock=end]').value;
      return { kind: 'weekly', days: days, start: root.querySelector('[data-clock=start]').value, end: end || '24:00' };
    }
    var when = {
      kind: 'oneOff',
      first: root.querySelector('[data-date=first]').value,
      last: root.querySelector('[data-date=last]').value
    };
    var from = root.querySelector('[data-clock=from]').value;
    var to = root.querySelector('[data-clock=to]').value;
    if (from) when.from = from;
    if (to) when.to = to;
    return when;
  }
  function defaultWhen() {
    var today = todayIn(board.timezone);
    return { kind: 'weekly', days: ['mon', 'tue', 'wed', 'thu', 'fri'], start: '06:00', end: '10:30', first: today, last: today };
  }
  function flushSchedule() {
    var root = document.getElementById('schedule');
    if (!root.hidden && current().when) current().when = readWhen(root);
  }
  function scheduleProblem() {
    flushSchedule();
    for (var i = 0; i < board.dayparts.length; i++) {
      var part = board.dayparts[i];
      var when = part.when || {};
      var name = part.name || 'a menu';
      if (when.kind === 'weekly' && (!(when.days && when.days.length) || !when.start)) return 'Choose at least one day and a start time for ' + name + '.';
      if (when.kind === 'oneOff' && (!when.first || !when.last)) return 'Choose the first and last day for ' + name + '.';
    }
    return '';
  }
  function drawItems(section, card) {
    var list = card.querySelector('.item-list'); list.textContent = '';
    section.items.forEach(function (item, index) {
      var row = document.getElementById('item-template').content.firstElementChild.cloneNode(true);
      row.classList.toggle('is-hidden', !item.visible);
      row.querySelectorAll('[data-field]').forEach(function (input) {
        var key = input.dataset.field;
        if (key === 'visible') input.checked = item.visible; else input.value = item[key];
        input.addEventListener('input', function () {
          item[key] = key === 'visible' ? input.checked : input.value;
          row.classList.toggle('is-hidden', !item.visible);
          changed(); count();
        });
      });
      var up = row.querySelector('.move-up'), down = row.querySelector('.move-down');
      up.disabled = index === 0; down.disabled = index === section.items.length - 1;
      function move(to) { section.items.splice(index, 1); section.items.splice(to, 0, item); changed(); drawItems(section, card); }
      up.onclick = function () { move(index - 1); }; down.onclick = function () { move(index + 1); };
      row.querySelector('.remove').onclick = function () {
        if (!window.confirm('Remove ' + (item.name || 'this item') + '? You can hide it using “On the menu” instead.')) return;
        section.items.splice(index, 1); changed(); drawItems(section, card); count();
      };
      list.appendChild(row);
    });
    card.querySelector('.item-count').textContent = section.items.length + ' items';
  }
  function drawSections() {
    var menu = current();
    var sections = document.getElementById('sections'); sections.textContent = '';
    document.getElementById('section-note').hidden = allDay();
    menu.sections.forEach(function (section, index) {
      var card = document.getElementById('section-template').content.firstElementChild.cloneNode(true);
      card.open = false;
      card.querySelector('h2').textContent = section.name || 'Section';
      card.querySelector('.section-number').textContent = '0' + (index + 1);
      card.querySelectorAll('[data-section]').forEach(function (input) {
        var key = input.dataset.section;
        if (key === 'name') input.required = allDay();
        input.value = key === 'extras' ? section.extras.join('\n') : section[key];
        input.oninput = function () {
          section[key] = key === 'extras' ? input.value.split('\n').map(function (value) { return value.trim(); }).filter(Boolean) : input.value;
          if (key === 'name') card.querySelector('h2').textContent = input.value || 'Section';
          changed();
        };
      });
      card.querySelector('.add').onclick = function () {
        if (section.items.length >= 40) { fail('Each section can have up to 40 items.'); return; }
        section.items.push({ name: '', price: '', visible: true }); changed(); drawItems(section, card); count();
        card.querySelector('.item-editor:last-child input').focus();
      };
      drawItems(section, card); sections.appendChild(card);
    });
  }
  function drawSpecials() {
    var menu = current();
    var root = document.getElementById('specials'); root.textContent = '';
    var head = document.createElement('div');
    head.className = 'specials-head';
    var title = document.createElement('h2');
    title.textContent = 'Order by number';
    var note = document.createElement('p');
    note.className = 'menu-note';
    note.textContent = allDay() ? 'These show on the right when no scheduled menu is on.' : 'These show on the right while this menu is on.';
    var add = document.createElement('button');
    add.type = 'button'; add.className = 'secondary'; add.textContent = 'Add a special';
    add.onclick = function () {
      if (menu.specials.length >= 20) { fail('Each menu can have up to 20 specials.'); return; }
      var number = 1;
      menu.specials.forEach(function (special) { if (special.number >= number) number = special.number + 1; });
      if (number > 99) number = 1;
      menu.specials.push({ number: number, name: '', contents: '', price: '', individual: '', savings: '' });
      changed(); drawSpecials(); count();
      var fields = root.querySelectorAll('.special-editor');
      fields[fields.length - 1].querySelector('input').focus();
    };
    head.appendChild(title); head.appendChild(note); head.appendChild(add); root.appendChild(head);
    menu.specials.forEach(function (special, index) {
      var row = document.getElementById('special-template').content.firstElementChild.cloneNode(true);
      row.querySelectorAll('[data-special]').forEach(function (input) {
        var key = input.dataset.special;
        input.value = special[key] == null ? '' : special[key];
        input.addEventListener('input', function () {
          special[key] = key === 'number' ? Number(input.value) : input.value;
          changed();
        });
      });
      row.querySelector('.remove').onclick = function () {
        if (!window.confirm('Remove special ' + (special.number || '') + '?')) return;
        menu.specials.splice(index, 1); changed(); drawSpecials(); count();
      };
      root.appendChild(row);
    });
  }
  function movePart(index, delta) {
    var next = index + delta;
    if (next < 0 || next >= board.dayparts.length) return;
    var part = board.dayparts.splice(index, 1)[0];
    board.dayparts.splice(next, 0, part);
    changed(); draw();
  }
  function drawMenus() {
    var list = document.getElementById('menu-list'); list.textContent = '';
    function rowFor(menu, partIndex) {
      var row = document.createElement('div');
      row.className = 'menu-row' + (menu.id === selected ? ' is-selected' : '');
      var button = document.createElement('button');
      button.type = 'button'; button.className = 'menu-pick';
      button.textContent = (menu.name || 'Untitled') + (partIndex == null ? ' · Always on the left' : '');
      button.onclick = function () { flushSchedule(); selected = menu.id; draw(); };
      row.appendChild(button);
      if (partIndex != null) {
        var up = document.createElement('button');
        up.type = 'button'; up.className = 'secondary'; up.textContent = 'Up'; up.setAttribute('aria-label', 'Move menu up');
        up.disabled = partIndex === 0;
        up.onclick = function () { movePart(partIndex, -1); };
        var down = document.createElement('button');
        down.type = 'button'; down.className = 'secondary'; down.textContent = 'Down'; down.setAttribute('aria-label', 'Move menu down');
        down.disabled = partIndex === board.dayparts.length - 1;
        down.onclick = function () { movePart(partIndex, 1); };
        var remove = document.createElement('button');
        remove.type = 'button'; remove.className = 'secondary remove'; remove.textContent = 'Delete';
        remove.onclick = function () {
          if (!window.confirm('Remove ' + (menu.name || 'this menu') + '? The all-day menu stays on the left.')) return;
          board.dayparts.splice(partIndex, 1);
          if (selected === menu.id) selected = board.allDay.id;
          changed(); draw();
        };
        row.appendChild(up); row.appendChild(down); row.appendChild(remove);
      }
      list.appendChild(row);
    }
    rowFor(board.allDay, null);
    board.dayparts.forEach(function (part, index) { rowFor(part, index); });
  }
  function drawSchedule() {
    var root = document.getElementById('schedule');
    var menu = current();
    if (!menu.when) { root.hidden = true; root.textContent = ''; return; }
    root.hidden = false;
    buildWhen(root, 'schedule-kind');
    applyWhen(root, menu.when);
    root.onchange = function (e) {
      if (e.target && e.target.type === 'radio' && e.target.value !== menu.when.kind) {
        var today = todayIn(board.timezone);
        menu.when = e.target.value === 'oneOff'
          ? { kind: 'oneOff', first: today, last: today }
          : { kind: 'weekly', days: ['mon', 'tue', 'wed', 'thu', 'fri'], start: '06:00', end: '10:30' };
        applyWhen(root, menu.when);
      } else menu.when = readWhen(root);
      changed();
    };
  }
  function syncTimezone() {
    var select = document.getElementById('timezone');
    var name = board.timezone || 'America/Chicago';
    var found = false;
    for (var i = 0; i < select.options.length; i++) if (select.options[i].value === name) found = true;
    if (!found) {
      var option = document.createElement('option');
      option.value = name; option.textContent = name; select.appendChild(option);
    }
    select.value = name;
    board.timezone = name;
  }
  function draw() {
    document.getElementById('brand').value = board.brand;
    document.getElementById('title').value = board.title;
    syncTimezone();
    var menu = current();
    var nameInput = document.getElementById('menu-name');
    nameInput.value = menu.name || '';
    nameInput.oninput = function () { menu.name = nameInput.value; drawMenus(); count(); changed(); };
    drawMenus(); drawSchedule(); drawSections(); drawSpecials(); count();
    form.hidden = false;
  }
  async function request(url, options) {
    var response = await fetch(url, Object.assign({ headers: { 'Accept': 'application/json' }, cache: 'no-store' }, options || {}));
    if (response.redirected || response.status === 401) throw new Error('Your sign-in expired. Open the editor in another tab to sign in, then try saving again.');
    var body = await response.json().catch(function () { return {}; });
    if (!response.ok) throw new Error(body.error || body.message || 'Could not save. Please try again.');
    return body;
  }
  function openDialog(mode) {
    dialogMode = mode;
    error.hidden = true;
    document.getElementById('dialog-title').textContent = mode === 'copy' ? 'Copy this menu' : 'New menu';
    document.getElementById('dialog-confirm').textContent = mode === 'copy' ? 'Copy menu' : 'Add menu';
    document.getElementById('dialog-name').value = mode === 'copy' ? ((current().name || 'Menu') + ' copy') : '';
    var root = document.getElementById('dialog-when');
    if (!root.dataset.ready) {
      buildWhen(root, 'dialog-kind');
      root.dataset.ready = '1';
      root.addEventListener('change', function (e) { if (e.target.type === 'radio') syncPanels(root); });
    }
    var seed = defaultWhen();
    applyWhen(root, seed);
    root.querySelector('[data-date=first]').value = seed.first;
    root.querySelector('[data-date=last]').value = seed.last;
    if (dialog.showModal) dialog.showModal(); else dialog.hidden = false;
    document.getElementById('dialog-name').focus();
  }
  document.getElementById('new-menu').onclick = function () { openDialog('new'); };
  document.getElementById('copy-menu').onclick = function () { flushSchedule(); openDialog('copy'); };
  document.getElementById('dialog-cancel').onclick = function () { if (dialog.close) dialog.close(); else dialog.hidden = true; };
  document.getElementById('menu-dialog-form').onsubmit = async function (e) {
    e.preventDefault();
    var name = document.getElementById('dialog-name').value.trim();
    var when = readWhen(document.getElementById('dialog-when'));
    if (when.kind === 'weekly' && (!when.days.length || !when.start)) { fail('Choose at least one day and a start time.'); return; }
    if (when.kind === 'oneOff' && (!when.first || !when.last)) { fail('Choose the first and last day.'); return; }
    if (dialogMode === 'new') {
      var part = { id: newID(), name: name, sections: emptySections(), specials: [], when: when };
      board.dayparts.push(part);
      selected = part.id;
      changed(); draw();
      if (dialog.close) dialog.close(); else dialog.hidden = true;
      return;
    }
    var confirm = document.getElementById('dialog-confirm');
    var copyButton = document.getElementById('copy-menu');
    confirm.disabled = true; copyButton.disabled = true;
    try {
      flushSchedule();
      board = prepareBoard(await request('/admin/menu/copy', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Accept': 'application/json' },
        body: JSON.stringify({ board: board, from: selected, name: name, when: when })
      }));
      selected = board.dayparts[board.dayparts.length - 1].id;
      changed(); draw();
      if (dialog.close) dialog.close(); else dialog.hidden = true;
    } catch (err) { fail(err.message); }
    finally { confirm.disabled = false; copyButton.disabled = false; }
  };
  async function load() {
    error.hidden = true; retry.hidden = true;
    try { board = prepareBoard(await request('/admin/menu')); selected = board.allDay.id; draw(); }
    catch (e) { fail('Could not load your menu. ' + e.message); retry.hidden = false; }
  }
  retry.onclick = load;
  ['brand', 'title'].forEach(function (key) {
    document.getElementById(key).oninput = function (e) { board[key] = e.target.value; changed(); };
  });
  document.getElementById('timezone').onchange = function (e) { board.timezone = e.target.value; changed(); };
  // Open collapsed sections when a required field needs attention.
  form.addEventListener('invalid', function (e) { var p = e.target.parentElement; while (p) { if (p.tagName === 'DETAILS') p.open = true; p = p.parentElement; } }, true);
  form.onsubmit = async function (e) {
    e.preventDefault(); if (!dirty || saving) return;
    var problem = scheduleProblem();
    if (problem) { fail(problem); return; }
    saving = true; save.disabled = true; form.classList.add('saving'); error.hidden = true;
    document.getElementById('save-label').textContent = 'Saving your menu…';
    try {
      board = prepareBoard(await request('/admin/menu', { method: 'PUT', headers: { 'Content-Type': 'application/json', 'Accept': 'application/json' }, body: JSON.stringify(board) }));
      draw(); dirty = false;
      document.getElementById('save-label').textContent = 'Saved. Fresh menu, coming up.';
      status.textContent = 'The TV will pick up your changes within 30 seconds.';
    } catch (err) { fail(err.message); document.getElementById('save-label').textContent = 'Your changes haven’t been saved'; save.disabled = false; }
    finally { saving = false; form.classList.remove('saving'); }
  };
  window.addEventListener('beforeunload', function (e) { if (dirty) { e.preventDefault(); e.returnValue = ''; } });
  var initial = document.getElementById('initial-menu');
  if (initial) { board = prepareBoard(JSON.parse(initial.textContent)); selected = board.allDay.id; draw(); }
  else load();
}());
