(function () {
  'use strict';
  var form = document.getElementById('menu-form'), save = document.getElementById('save');
  var error = document.getElementById('error'), status = document.getElementById('status');
  var retry = document.getElementById('retry'), url = form.dataset.url;
  var menu, dirty = false, saving = false;
  function $(id) { return document.getElementById(id); }
  function fail(message) { error.textContent = message; error.hidden = false; }
  // The unsaved draft, as the menu page would draw it, for a preview window.
  var draftKey = 'vcc-preview:' + url.split('/').pop();
  function preview(keep) {
    try {
      if (keep) localStorage.setItem(draftKey, JSON.stringify({at: Date.now(), menu: {brand: menu.brand, title: menu.title.trim() || menu.name, sections: menu.sections}}));
      else localStorage.removeItem(draftKey);
    } catch (_) {}
  }
  function changed() {
    dirty = true; save.disabled = false; preview(true);
    $('save-label').textContent = 'You have unsaved changes';
    status.textContent = 'Changes go live within 30 seconds of saving.';
  }
  function count() {
    var total = menu.sections.reduce(function (n, s) { return n + s.items.filter(function (i) { return i.visible; }).length; }, 0);
    $('summary').textContent = total + ' items on this menu · 3 sections';
  }
  function drawItems(section, card) {
    var list = card.querySelector('.item-list'); list.textContent = '';
    section.items.forEach(function (item, index) {
      var row = $('item-template').content.firstElementChild.cloneNode(true);
      row.classList.toggle('is-hidden', !item.visible);
      row.querySelectorAll('[data-field]').forEach(function (input) {
        var field = input.dataset.field;
        if (field === 'visible') input.checked = item.visible; else input.value = item[field];
        input.addEventListener('input', function () {
          item[field] = field === 'visible' ? input.checked : input.value;
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

  // The schedule. Times are the shop's local wall-clock strings, as the
  // server keeps them: 15:04 for weekly hours, 2006-01-02T15:04 for events.
  function pad(n) { return (n < 10 ? '0' : '') + n; }
  function stamp(d) { return d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate()) + 'T' + pad(d.getHours()) + ':' + pad(d.getMinutes()); }
  function parse(v) { var m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/.exec(v || ''); return m ? new Date(+m[1], m[2] - 1, +m[3], +m[4], +m[5]) : null; }
  function hourLabel(h) { var hh = Math.floor(h + 1e-6), mm = Math.round((h - hh) * 60); return ((hh + 11) % 12 + 1) + (mm ? ':' + pad(mm) : '') + (hh % 24 >= 12 ? 'p' : 'a'); }
  function schedule() { return menu.schedule; }
  function drawHours() {
    var rows = $('hours-rows'); rows.textContent = '';
    var s = schedule();
    if (!s.hours.length) s.hours.push({days: [], start: '07:00', end: '14:00'});
    s.hours.forEach(function (h, index) {
      var row = $('hours-template').content.firstElementChild.cloneNode(true);
      row.querySelectorAll('[data-day]').forEach(function (button) {
        var day = button.dataset.day;
        button.setAttribute('aria-pressed', String(h.days.indexOf(day) >= 0));
        button.onclick = function () {
          var on = h.days.indexOf(day) < 0;
          // A day has one set of hours: choosing it here takes it from the others.
          s.hours.forEach(function (other) { other.days = other.days.filter(function (d) { return d !== day; }); });
          if (on) h.days.push(day);
          scheduleChanged(); drawHours();
        };
      });
      row.querySelectorAll('[data-hours]').forEach(function (input) {
        input.value = h[input.dataset.hours];
        input.oninput = function () { h[input.dataset.hours] = input.value; scheduleChanged(); };
      });
      var remove = row.querySelector('.remove-hours');
      remove.hidden = s.hours.length === 1;
      remove.onclick = function () { s.hours.splice(index, 1); scheduleChanged(); drawHours(); };
      rows.appendChild(row);
    });
    $('add-hours').hidden = s.hours.length >= 7;
  }
  function drawSchedule() {
    var s = schedule(), weekly = s.kind !== 'event';
    document.querySelectorAll('input[name=kind]').forEach(function (r) { r.checked = r.value === s.kind; });
    $('kind-label').textContent = weekly ? 'RECURRING MENU' : 'ONE-OFF EVENT';
    $('weekly-fields').hidden = !weekly; $('event-fields').hidden = weekly;
    if (weekly) { drawHours(); $('season-from').value = s.from; $('season-until').value = s.until; }
    else { $('event-start').value = s.start; $('event-end').value = s.end; duration(); }
    $('published').checked = menu.published;
  }
  function duration() {
    var a = parse(schedule().start), b = parse(schedule().end), text = '';
    if (a && b && b > a) {
      var mins = Math.round((b - a) / 60000), days = Math.floor(mins / 1440), hrs = (mins - days * 1440) / 60, parts = [];
      if (days) parts.push(days + (days === 1 ? ' day' : ' days'));
      if (hrs) parts.push(hrs + (hrs === 1 ? ' hour' : ' hours'));
      text = 'Runs ' + parts.join(', ') + '.';
    }
    $('event-duration').textContent = text;
  }
  // payload is the schedule as it's saved: only its kind's fields, and only
  // hours with days chosen.
  function payload() {
    var s = schedule();
    if (s.kind === 'event') return {kind: 'event', hours: [], from: '', until: '', start: s.start, end: s.end};
    return {kind: 'weekly', hours: s.hours.filter(function (h) { return h.days.length; }), from: s.from, until: s.until, start: '', end: ''};
  }
  function scheduleChanged() { changed(); check(); }

  var checkTimer, checking = 0, last;
  function check() {
    clearTimeout(checkTimer);
    checkTimer = setTimeout(async function () {
      var id = ++checking;
      try {
        var result = await request(url + '/check', {method: 'POST', headers: {'Content-Type': 'application/json', 'Accept': 'application/json'}, body: JSON.stringify({name: menu.name, schedule: payload()})});
        if (id === checking) { last = result; drawCheck(); }
      } catch (_) {}
    }, 200);
  }
  function drawCheck() {
    var r = last, o = r.overlap, event = schedule().kind === 'event';
    $('schedule-summary').textContent = r.summary;
    $('schedule-flag').hidden = !o; $('schedule-flag').textContent = o ? ' · ' + o.short : '';
    $('schedule-error').hidden = !r.error; $('schedule-error').textContent = r.error || '';
    $('no-overlap').hidden = !!(o || r.error);
    $('overlap').hidden = !o;
    if (!o) return;
    $('overlap-title').textContent = o.title;
    $('overlap-note').textContent = o.note;
    var fix = $('overlap-fix');
    fix.hidden = !o.fix;
    if (o.fix) {
      fix.textContent = o.fix.label;
      fix.onclick = function () { schedule().start = o.fix.times[0]; schedule().end = o.fix.times[1]; drawSchedule(); scheduleChanged(); };
    }
    $('timeline').hidden = !event;
    if (event) drawTimeline();
  }

  // The timeline: an event's first day, the other menus above it, and the
  // event below, dragged or nudged with the arrow keys by 15 minutes.
  var view = {lo: 6, hi: 22};
  function dayStart() { var a = parse(schedule().start); return a && new Date(a.getFullYear(), a.getMonth(), a.getDate()); }
  function eventHours() {
    var day = dayStart(), a = parse(schedule().start), b = parse(schedule().end);
    return {s: (a - day) / 3600000, e: (b - day) / 3600000};
  }
  function pct(h) { return ((Math.max(view.lo, Math.min(view.hi, h)) - view.lo) / (view.hi - view.lo)) * 100; }
  function place(el, from, to) { el.style.left = pct(from) + '%'; el.style.width = (pct(to) - pct(from)) + '%'; }
  function drawTimeline() {
    var ev = eventHours(), r = last;
    var lo = Math.min(6, Math.floor(ev.s)), hi = Math.max(22, Math.ceil(Math.min(24, ev.e)));
    r.day.forEach(function (b) { lo = Math.min(lo, Math.floor(b.from)); hi = Math.max(hi, Math.ceil(b.to)); });
    view = {lo: lo, hi: hi};
    var ticks = $('timeline-ticks'); ticks.textContent = '';
    for (var t = lo; t <= hi; t += 4) { var tick = document.createElement('span'); tick.textContent = hourLabel(t).replace(/a$/, ' AM').replace(/p$/, ' PM'); ticks.appendChild(tick); }
    var others = $('timeline-others'); others.textContent = '';
    r.day.forEach(function (b) {
      var block = document.createElement('div'); block.className = 'block';
      block.textContent = b.name + ' ' + hourLabel(b.from) + '–' + hourLabel(b.to); block.title = block.textContent;
      place(block, b.from, b.to); others.appendChild(block);
    });
    var box = $('overlap-box'), o = r.overlap;
    box.hidden = !o || o.to <= o.from;
    if (o && o.to > o.from) place(box, o.from, o.to);
    drawBar();
  }
  function drawBar() {
    var ev = eventHours(), bar = $('event-bar'), multi = ev.e > 24;
    place(bar, ev.s, Math.min(ev.e, 24));
    $('event-bar-label').textContent = (menu.name || 'Event') + ' ' + hourLabel(ev.s) + (multi ? ' →' : '–' + hourLabel(ev.e));
    bar.setAttribute('aria-label', (menu.name || 'Event') + ', ' + $('event-start').value.replace('T', ' ') + ' to ' + $('event-end').value.replace('T', ' ') + '. Arrow keys move it 15 minutes; Shift plus arrows changes the end time, Alt plus arrows the start time.');
    var note = $('timeline-note'); note.hidden = !multi;
    if (multi) note.textContent = 'Showing the first day. The event continues until ' + parse(schedule().end).toLocaleString([], {weekday: 'short', month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit'}) + '.';
  }
  // shift moves the event's start and end by hours, as kind says: the
  // whole event, or one edge, keeping at least half an hour.
  function shift(kind, s0, e0, dh) {
    var s = s0, e = e0;
    if (kind === 'move') { s = s0 + dh; e = e0 + dh; }
    else if (kind === 'start') s = Math.min(e0 - 0.5, s0 + dh);
    else e = Math.max(s0 + 0.5, e0 + dh);
    var day = dayStart();
    schedule().start = stamp(new Date(day.getTime() + s * 3600000));
    schedule().end = stamp(new Date(day.getTime() + e * 3600000));
    $('event-start').value = schedule().start; $('event-end').value = schedule().end;
    duration(); drawBar(); scheduleChanged();
  }
  $('event-bar').addEventListener('pointerdown', function (e) {
    if (e.button !== 0) return;
    e.preventDefault();
    var kind = e.target.dataset.drag || 'move', bar = this, track = $('timeline-track');
    var perHour = track.getBoundingClientRect().width / (view.hi - view.lo), x0 = e.clientX, ev = eventHours();
    var applied = 0;
    bar.classList.add('dragging'); bar.setPointerCapture(e.pointerId);
    function move(m) {
      var dh = Math.round(((m.clientX - x0) / perHour) * 4) / 4;
      if (dh !== applied) { applied = dh; shift(kind, ev.s, ev.e, dh); }
    }
    function up() { bar.classList.remove('dragging'); bar.removeEventListener('pointermove', move); bar.removeEventListener('pointerup', up); bar.removeEventListener('pointercancel', up); }
    bar.addEventListener('pointermove', move); bar.addEventListener('pointerup', up); bar.addEventListener('pointercancel', up);
  });
  $('event-bar').addEventListener('keydown', function (e) {
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return;
    e.preventDefault();
    var ev = eventHours();
    shift(e.shiftKey ? 'end' : e.altKey ? 'start' : 'move', ev.s, ev.e, e.key === 'ArrowLeft' ? -0.25 : 0.25);
  });

  document.querySelectorAll('input[name=kind]').forEach(function (radio) {
    radio.onchange = function () {
      var s = schedule(); s.kind = radio.value;
      if (s.kind === 'event' && !parse(s.start)) {
        var d = new Date(); d.setDate(d.getDate() + 1); d.setHours(10, 0, 0, 0);
        s.start = stamp(d); d.setHours(14); s.end = stamp(d);
      }
      drawSchedule(); scheduleChanged();
    };
  });
  [['season-from', 'from'], ['season-until', 'until'], ['event-start', 'start'], ['event-end', 'end']].forEach(function (pair) {
    $(pair[0]).oninput = function (e) { schedule()[pair[1]] = e.target.value; duration(); scheduleChanged(); };
  });
  $('add-hours').onclick = function () { schedule().hours.push({days: [], start: '17:00', end: '20:00'}); drawHours(); };
  $('published').onchange = function (e) { menu.published = e.target.checked; changed(); };
  $('name').oninput = function (e) {
    menu.name = e.target.value; $('title').placeholder = menu.name;
    $('menu-heading').textContent = (menu.name || 'Untitled') + '.';
    changed(); check();
  };
  $('title').oninput = function (e) { menu.title = e.target.value; changed(); };

  function draw() {
    var s = menu.schedule;
    s.hours = s.hours || []; s.from = s.from || ''; s.until = s.until || ''; s.start = s.start || ''; s.end = s.end || '';
    $('name').value = menu.name; $('title').value = menu.title; $('title').placeholder = menu.name;
    drawSchedule(); check();
    var sections = $('sections'); sections.textContent = '';
    menu.sections.forEach(function (section, index) {
      var card = $('section-template').content.firstElementChild.cloneNode(true);
      card.open = false;
      card.querySelector('h2').textContent = section.name;
      card.querySelector('.section-number').textContent = '0' + (index + 1);
      card.querySelectorAll('[data-section]').forEach(function (input) {
        var field = input.dataset.section;
        input.value = field === 'extras' ? section.extras.join('\n') : section[field];
        input.oninput = function () {
          section[field] = field === 'extras' ? input.value.split('\n').map(function (v) { return v.trim(); }).filter(Boolean) : input.value;
          if (field === 'name') card.querySelector('h2').textContent = input.value;
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
    count(); form.hidden = false;
  }
  async function request(url, options) {
    var response = await fetch(url, Object.assign({headers: {'Accept':'application/json'}, cache: 'no-store'}, options || {}));
    if (response.redirected || response.status === 401) throw new Error('Your sign-in expired. Open the editor in another tab to sign in, then try saving again.');
    var body = await response.json();
    if (!response.ok) throw new Error(body.error || body.message || 'Could not save. Please try again.');
    return body;
  }
  async function load() {
    error.hidden = true; retry.hidden = true;
    try { menu = await request(url + '/data'); draw(); }
    catch (e) { fail('Could not load your menu. ' + e.message); retry.hidden = false; }
  }
  retry.onclick = load;
  // Open collapsed sections when a required field needs attention.
  form.addEventListener('invalid', function (e) {var p=e.target.parentElement;while(p){if(p.tagName==='DETAILS')p.open=true;p=p.parentElement;}}, true);
  form.onsubmit = async function (e) {
    e.preventDefault(); if (!dirty || saving) return;
    saving = true; save.disabled = true; form.classList.add('saving'); error.hidden = true;
    $('save-label').textContent = 'Saving your menu…';
    try {
      var body = Object.assign({}, menu, {schedule: payload()});
      var saved = await request(url, {method:'PUT', headers:{'Content-Type':'application/json','Accept':'application/json'}, body:JSON.stringify(body)});
      // Keep the draft's schedule (an empty hours row stays on screen); take the rest as saved.
      saved.schedule = menu.schedule; menu = saved;
      draw(); dirty = false; preview(false);
      $('save-label').textContent = 'Saved. Fresh menu, coming up.';
      status.textContent = menu.published ? 'Whenever this menu is live, changes show within 30 seconds.' : 'This menu is a draft, so it won’t go live.';
    } catch (e) {fail(e.message); $('save-label').textContent = 'Your changes haven’t been saved'; save.disabled = false;}
    finally {saving = false; form.classList.remove('saving');}
  };
  $('delete-form').onsubmit = function (e) {
    if (!window.confirm('Delete ' + (menu.name || 'this menu') + '? This can’t be undone.')) { e.preventDefault(); return; }
    dirty = false;
  };
  window.addEventListener('beforeunload', function (e) {if(dirty){e.preventDefault();e.returnValue='';}});
  // Leaving drops the draft, so a preview goes back to the saved version;
  // while open, it's renewed, as a preview ignores one gone stale (from a
  // tab that closed without saying so).
  window.addEventListener('pagehide', function () { preview(false); });
  // The preview opens in a window of its own, landscape, reused on later
  // clicks; a new tab if windows are blocked.
  $('preview').onclick = function (e) {
    if (window.open(this.href, 'menu-preview-' + menu.id, 'width=1280,height=720')) e.preventDefault();
  };
  setInterval(function () { if (dirty) preview(true); }, 10000);
  preview(false); // a draft left by an editor that closed without saying so
  var initial = $('initial-menu');
  if (initial) { menu = JSON.parse(initial.textContent); draw(); } else { load(); }
}());
