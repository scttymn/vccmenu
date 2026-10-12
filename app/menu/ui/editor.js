(function () {
  'use strict';
  var form = document.getElementById('menu-form'), save = document.getElementById('save');
  var error = document.getElementById('error'), status = document.getElementById('status');
  var retry = document.getElementById('retry');
  var menu, dirty = false, saving = false;
  function fail(message) { error.textContent = message; error.hidden = false; }
  function changed() {
    dirty = true; save.disabled = false;
    document.getElementById('save-label').textContent = 'You have unsaved changes';
    status.textContent = 'Save when you’re ready to update the TV.';
  }
  function count() {
    var total = menu.sections.reduce(function (n, s) { return n + s.items.filter(function (i) { return i.visible; }).length; }, 0);
    document.getElementById('summary').textContent = total + ' items on the menu · 3 sections';
  }
  function drawItems(section, card) {
    var list = card.querySelector('.item-list'); list.textContent = '';
    section.items.forEach(function (item, index) {
      var row = document.getElementById('item-template').content.firstElementChild.cloneNode(true);
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
  function draw() {
    document.getElementById('brand').value = menu.brand;
    document.getElementById('title').value = menu.title;
    var sections = document.getElementById('sections'); sections.textContent = '';
    menu.sections.forEach(function (section, index) {
      var card = document.getElementById('section-template').content.firstElementChild.cloneNode(true);
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
    try { menu = await request('/admin/menu'); draw(); }
    catch (e) { fail('Could not load your menu. ' + e.message); retry.hidden = false; }
  }
  retry.onclick = load;
  ['brand','title'].forEach(function (field) {document.getElementById(field).oninput = function (e) {menu[field] = e.target.value; changed();};});
  // Open collapsed sections when a required field needs attention.
  form.addEventListener('invalid', function (e) {var p=e.target.parentElement;while(p){if(p.tagName==='DETAILS')p.open=true;p=p.parentElement;}}, true);
  form.onsubmit = async function (e) {
    e.preventDefault(); if (!dirty || saving) return;
    saving = true; save.disabled = true; form.classList.add('saving'); error.hidden = true;
    document.getElementById('save-label').textContent = 'Saving your menu…';
    try {
      menu = await request('/admin/menu', {method:'PUT', headers:{'Content-Type':'application/json','Accept':'application/json'}, body:JSON.stringify(menu)});
      // Keep the draft object bound to inputs after a successful save.
      draw(); dirty = false;
      document.getElementById('save-label').textContent = 'Saved. Fresh menu, coming up.';
      status.textContent = 'The TV will pick up your changes within 30 seconds.';
    } catch (e) {fail(e.message); document.getElementById('save-label').textContent = 'Your changes haven’t been saved'; save.disabled = false;}
    finally {saving = false; form.classList.remove('saving');}
  };
  window.addEventListener('beforeunload', function (e) {if(dirty){e.preventDefault();e.returnValue='';}});
  var initial = document.getElementById('initial-menu');
  if (initial) { menu = JSON.parse(initial.textContent); draw(); } else { load(); }
}());
