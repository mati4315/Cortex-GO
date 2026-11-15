(() => {
  'use strict';

  const PAGE_SIZE = 50;
  const REFRESH_MS = 5000;
  const FONT_KEY = 'cortexFacebookHistoryFontScale';
  const THEME_KEY = 'cortexFacebookHistoryTheme';
  const OPTIONS_KEY = 'cortexFacebookHistoryOptions';
  const $ = (id) => document.getElementById(id);
  const list = $('list');
  const toolbar = $('toolbar');
  const search = $('filterText');
  const streamFilter = $('filterStream');
  const source = $('filterSource');
  const filterFrom = $('filterFrom');
  const filterTo = $('filterTo');
  const state = { events: [], total: 0, offset: 0, busy: false, checkingNew: false, failed: false, archiveId: null, archiveProgram: '', archiveEvents: [], latestSeenID: 0, newCount: 0 };
  let clearArmed = false;
  let clearTimer = 0;
  let toastTimer = 0;
  let searchTimer = 0;
  let featuredEventID = 0;
  let featuredArchiveID = 0;

  function showToast(message) {
    const toast = $('toast');
    toast.textContent = message;
    toast.hidden = false;
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => { toast.hidden = true; }, 3500);
  }

  async function request(path, options = {}) {
    const response = await fetch(path, { cache: 'no-store', ...options });
    if (!response.ok) {
      let message = 'No se pudo completar la solicitud';
      try { message = (await response.text()).slice(0, 300) || message; } catch (_) {}
      throw new Error(message);
    }
    return response.status === 204 ? null : response.json();
  }

  function dateBounds() {
    const toStartISO = (value, nextDay = false) => {
      if (!value) return '';
      const [year, month, day] = value.split('-').map(Number);
      return new Date(year, month - 1, day + (nextDay ? 1 : 0)).toISOString();
    };
    return { from: toStartISO(filterFrom.value), to: toStartISO(filterTo.value, true) };
  }

  function validateDateRange() {
    if (filterFrom.value && filterTo.value && filterFrom.value > filterTo.value) {
      showToast('La fecha inicial debe ser anterior o igual a la fecha final.');
      return false;
    }
    return true;
  }

  function updateNewNotice() {
    const button = $('newCommentsNotice');
    const count = state.newCount;
    button.hidden = count < 1 || state.archiveId !== null;
    button.textContent = count === 1 ? '1 comentario nuevo · ver más recientes' : `${count} comentarios nuevos · ver más recientes`;
  }

  function queryString(includePage = true) {
    const params = new URLSearchParams();
    const text = search.value.trim();
    if (text) params.set('q', text);
    if (source.value) params.set('feedId', source.value);
    const streamID = streamFilter.value.trim();
    if (streamID) params.set('streamId', streamID);
    const dates = dateBounds();
    if (dates.from) params.set('from', dates.from);
    if (dates.to) params.set('to', dates.to);
    if (includePage) {
      params.set('limit', String(PAGE_SIZE));
      params.set('offset', String(state.offset));
    }
    return params.toString();
  }

  function formatDate(value) {
    if (!value) return 'Hora no disponible';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return 'Hora no disponible';
    return new Intl.DateTimeFormat('es-AR', {
      day: '2-digit', month: '2-digit', year: 'numeric',
      hour: '2-digit', minute: '2-digit', second: '2-digit',
    }).format(date);
  }

  function element(tag, className, content) {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (content !== undefined) node.textContent = content;
    return node;
  }

  function renderEmpty(title, detail, error = false) {
    const empty = element('div', 'empty' + (error ? ' error-state' : ''));
    empty.append(element('strong', '', title), element('span', '', detail));
    list.replaceChildren(empty);
  }

  function makeRow(event, index, archived = false, archiveID = 0) {
    const isFeatured = Number(event.id) === featuredEventID && Number(archiveID) === featuredArchiveID;
    const row = element('article', 'row' + (index % 2 ? ' zebra-b' : '') + (isFeatured ? ' featured' : ''));
    row.dataset.eventId = String(event.id);

    const comment = element('div', 'cell comment-cell feature-selectable');
    comment.title = 'Toca el comentario o el botón para destacarlo en el overlay de OBS';
    const avatar = element('div', 'avatar', (event.author || '?').trim().charAt(0).toLocaleUpperCase('es-AR') || '?');
    avatar.setAttribute('aria-hidden', 'true');
    const content = element('div', 'comment-content');
    const label = element('div', 'label');
    label.append(element('span', 'name', event.author || 'Usuario de Facebook'));
    label.append(element('span', 'badge', 'Facebook'));
    label.append(element('span', 'badge account', event.feedId === 'b' ? 'Cuenta 2' : 'Cuenta 1'));
    const featureHint = element('span', 'badge account feature-hint', isFeatured ? 'Destacado en OBS' : 'Toca para destacar');
    label.append(featureHint);
    label.append(element('time', 'stamp', formatDate(event.eventAt || event.createdAt)));
    const message = element('div', 'comment-text', event.message || '');
    let ruloResponse = null;
    if (event.ruloResponse && event.ruloResponse.text) {
      ruloResponse = element('div', 'rulo-response');
      const responseLabel = `Rulo${Number(event.ruloResponse.version) > 1 ? ' · actualizado' : ''}`;
      ruloResponse.append(element('span', 'rulo-response-label', responseLabel));
      ruloResponse.append(element('div', 'rulo-response-text', event.ruloResponse.text));
    }
    const featureButton = element('button', 'feature-select-button' + (isFeatured ? ' is-featured' : ''), isFeatured ? 'Destacado ✓' : 'Destacar');
    featureButton.type = 'button';
    featureButton.setAttribute('aria-label', `Destacar comentario de ${event.author || 'Facebook'} en el overlay`);
    const feature = async () => {
      if (row.dataset.featurePending === '1') return;
      row.dataset.featurePending = '1';
      row.classList.add('feature-pending');
      featureButton.disabled = true;
      featureButton.textContent = 'Enviando…';
      showToast('Enviando comentario al overlay…');
      try {
        const payload = { eventId: Number(event.id) };
        if (archiveID > 0) payload.archiveId = Number(archiveID);
        await request('/api/modules/facebook/featured', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload),
        });
        featuredEventID = Number(event.id);
        featuredArchiveID = Number(archiveID);
        document.querySelectorAll('.row.featured').forEach((item) => item.classList.remove('featured'));
        document.querySelectorAll('.feature-hint').forEach((item) => { item.textContent = 'Toca para destacar'; });
        document.querySelectorAll('.feature-select-button').forEach((item) => {
          item.textContent = 'Destacar';
          item.classList.remove('is-featured');
        });
        row.classList.add('featured');
        featureHint.textContent = 'Destacado en OBS';
        featureButton.textContent = 'Destacado ✓';
        featureButton.classList.add('is-featured');
        showToast('Comentario destacado en OBS');
      } catch (error) {
        featureButton.textContent = 'Reintentar';
        showToast('No se pudo destacar: ' + error.message);
      } finally {
        delete row.dataset.featurePending;
        row.classList.remove('feature-pending');
        featureButton.disabled = false;
      }
    };
    comment.addEventListener('click', (clickEvent) => {
      if (clickEvent.target instanceof Element && clickEvent.target.closest('.feature-select-button')) return;
      feature();
    });
    featureButton.addEventListener('click', feature);
    content.append(label, message);
    if (ruloResponse) content.append(ruloResponse);
    content.append(featureButton);
    comment.append(avatar, content);

    const details = element('div', 'cell details-cell');
    const detailText = element('div', 'details');
    detailText.append(element('div', '', 'Capturado por Cortex'));
    const received = element('div');
    received.append(element('strong', '', 'Recibido: '), document.createTextNode(formatDate(event.createdAt)));
    detailText.append(received);
    if (event.streamId) {
      const stream = element('div');
      stream.append(element('strong', '', 'Transmisión: '), document.createTextNode(event.streamId));
      detailText.append(stream);
    }
    const remove = element('button', 'delete-event', 'Borrar');
    remove.type = 'button';
    remove.hidden = archived;
    remove.setAttribute('aria-label', 'Borrar comentario de ' + (event.author || 'Facebook'));
    remove.addEventListener('click', async () => {
      if (!window.confirm('¿Borrar este comentario de la base activa? Las copias de seguridad anteriores pueden conservarlo.')) return;
      remove.disabled = true;
      try {
        await request('/api/events/' + encodeURIComponent(event.id), { method: 'DELETE' });
        showToast('Comentario borrado del historial activo');
        await refreshEvents();
      } catch (error) {
        remove.disabled = false;
        showToast(error.message);
      }
    });
    details.append(detailText, remove);
    row.append(comment, details);
    return row;
  }

  function updateControls() {
    const start = state.total === 0 ? 0 : state.offset + 1;
    const end = Math.min(state.offset + state.events.length, state.total);
    $('counter').textContent = `${start}–${end} de ${state.total} comentarios`;
    $('pageLabel').textContent = `Página ${Math.floor(state.offset / PAGE_SIZE) + 1}`;
    $('newerPage').disabled = state.offset === 0;
    $('olderPage').disabled = state.offset + state.events.length >= state.total || state.events.length === 0;
    $('clearHistory').disabled = state.archiveId !== null || state.total === 0;
    $('saveCurrentChat').disabled = state.archiveId !== null || state.total === 0;
    $('deleteSavedChat').disabled = state.archiveId === null;
    $('autoRefresh').disabled = state.archiveId !== null;
    $('lastUpdate').textContent = 'Actualizado ' + new Intl.DateTimeFormat('es-AR', { hour: '2-digit', minute: '2-digit', second: '2-digit' }).format(new Date());
    $('moreOptions').classList.toggle('has-filters', Boolean(search.value.trim() || streamFilter.value.trim() || source.value || filterFrom.value || filterTo.value));
  }

  function updateCaptureState(status) {
    const pill = $('wsState');
    const feeds = status && Array.isArray(status.feeds) ? status.feeds : [];
    const active = feeds.some((feed) => feed.captureAgentConnected && feed.commentsPage);
    const open = feeds.some((feed) => feed.captureAgentConnected);
    const error = feeds.some((feed) => feed.lastError);
    pill.className = 'chip ' + (active ? 'ok' : error ? 'bad' : 'warn');
    pill.textContent = active ? 'Captura activa' : error ? 'Revisar capturador' : open ? 'Abre Comentarios' : 'Esperando capturador';
  }

  async function refreshEvents(options = {}) {
    if (state.archiveId !== null) {
      await loadArchive(state.archiveId);
      return;
    }
    if (state.busy) return;
    state.busy = true;
    const query = queryString(true);
    try {
      const [result, status] = await Promise.all([
        request('/api/events?' + query),
        request('/api/status').catch(() => null),
      ]);
      state.events = Array.isArray(result.events) ? result.events : [];
      state.total = Number.isFinite(Number(result.total)) ? Number(result.total) : state.events.length;
      if (state.offset === 0) {
        state.latestSeenID = Number(result.latestId) || (state.events[0] ? Number(state.events[0].id) : 0);
        state.newCount = 0;
        updateNewNotice();
      }
      state.failed = false;
      updateCaptureState(status);
      if (!state.events.length) {
        const filtered = Boolean(search.value.trim() || streamFilter.value.trim() || source.value || filterFrom.value || filterTo.value);
        renderEmpty(filtered ? 'No hay coincidencias' : 'Todavía no hay comentarios guardados', filtered ? 'Prueba con otro texto o selecciona todas las cuentas.' : 'Los comentarios nuevos aparecerán aquí cuando el capturador esté conectado.');
      } else {
        list.replaceChildren(...state.events.map(makeRow));
      }
      updateControls();
    } catch (error) {
      state.failed = true;
      const pill = $('wsState');
      pill.className = 'chip bad';
      pill.textContent = 'Sin conexión';
      if (!state.events.length) renderEmpty('No se pudo cargar el historial', error.message, true);
      else showToast('No se pudo actualizar el historial. Se mantienen los comentarios visibles.');
      $('lastUpdate').textContent = '';
    } finally {
      state.busy = false;
    }
  }

  function resetAndRefresh() {
    if (!validateDateRange()) return;
    state.offset = 0;
    state.latestSeenID = 0;
    state.newCount = 0;
    updateNewNotice();
    if (state.archiveId !== null) renderArchivePage();
    else refreshEvents();
  }

  async function checkForNewComments() {
    if (state.checkingNew || state.latestSeenID < 1 || state.archiveId !== null) return;
    state.checkingNew = true;
    try {
      const params = new URLSearchParams(queryString(false));
      params.set('limit', '1');
      params.set('offset', '0');
      params.set('afterId', String(state.latestSeenID));
      const result = await request('/api/events?' + params.toString());
      state.newCount = Math.max(0, Number(result.newCount) || 0);
      updateNewNotice();
    } catch (_) {
      // El refresco normal muestra el estado de conexión si el servicio sigue fuera de línea.
    } finally {
      state.checkingNew = false;
    }
  }

  function filteredArchiveEvents() {
    const needle = search.value.trim().toLocaleLowerCase('es-AR');
    const streamID = streamFilter.value.trim();
    const dates = dateBounds();
    const fromTime = dates.from ? Date.parse(dates.from) : 0;
    const toTime = dates.to ? Date.parse(dates.to) : Infinity;
    return state.archiveEvents.filter((event) => {
      if (source.value && event.feedId !== source.value) return false;
      if (streamID && event.streamId !== streamID) return false;
      const eventTime = Date.parse(event.eventAt || event.createdAt);
      if (Number.isFinite(eventTime) && (eventTime < fromTime || eventTime >= toTime)) return false;
      if (!needle) return true;
      return [event.author, event.message, event.sourceId, event.streamId]
        .some((value) => String(value || '').toLocaleLowerCase('es-AR').includes(needle));
    });
  }

  function renderArchivePage() {
    const filtered = filteredArchiveEvents();
    state.total = filtered.length;
    state.events = filtered.slice(state.offset, state.offset + PAGE_SIZE);
    state.newCount = 0;
    updateNewNotice();
    const pill = $('wsState');
    pill.className = 'chip';
    pill.textContent = `Programa ${state.archiveProgram} · copia guardada`;
    if (!state.events.length) {
      renderEmpty('No hay comentarios en esta vista', 'Prueba con otro texto o vuelve al historial actual.');
    } else {
      list.replaceChildren(...state.events.map((event, index) => makeRow(event, index, true, state.archiveId)));
    }
    updateControls();
  }

  async function refreshArchives(selectedID = '') {
    const select = $('savedChatPrograms');
    try {
      const result = await request('/api/event-archives');
      select.replaceChildren(
        new Option('Chats guardados…', ''),
        new Option('Volver al chat actual', '__current__'),
      );
      for (const archive of result.items || []) {
        const option = new Option(`Programa ${archive.program} · ${archive.comments} comentarios`, String(archive.id));
        select.append(option);
      }
      select.value = selectedID || (state.archiveId !== null ? String(state.archiveId) : '__current__');
      $('archiveStatus').textContent = (result.items || []).length
        ? `${result.items.length} chat(s) guardado(s). Cada copia conserva sus comentarios hasta que la borres.`
        : 'Los chats guardados son copias independientes y permanecen hasta que los borres.';
    } catch (error) {
      $('archiveStatus').textContent = 'No se pudieron cargar los chats guardados: ' + error.message;
    }
  }

  async function loadArchive(id) {
    if (state.busy) return;
    state.busy = true;
    try {
      const result = await request('/api/event-archives/' + encodeURIComponent(id));
      state.archiveId = Number(id);
      state.archiveProgram = result.archive.program;
      state.archiveEvents = Array.isArray(result.archive.events) ? result.archive.events : [];
      state.offset = 0;
      state.newCount = 0;
      $('savedChatPrograms').value = String(id);
      renderArchivePage();
    } catch (error) {
      showToast(error.message);
      state.archiveId = null;
      state.archiveEvents = [];
      await refreshArchives('__current__');
      await refreshEvents();
    } finally {
      state.busy = false;
    }
  }

  async function loadCurrentChat() {
    state.archiveId = null;
    state.archiveProgram = '';
    state.archiveEvents = [];
    state.offset = 0;
    state.latestSeenID = 0;
    state.newCount = 0;
    updateNewNotice();
    $('savedChatPrograms').value = '__current__';
    await refreshEvents();
  }

  $('refreshEvents').addEventListener('click', () => refreshEvents());
  $('newCommentsNotice').addEventListener('click', () => {
    state.offset = 0;
    state.newCount = 0;
    updateNewNotice();
    list.scrollTo({ top: 0, behavior: 'smooth' });
    refreshEvents();
  });
  $('newerPage').addEventListener('click', () => {
    state.offset = Math.max(0, state.offset - PAGE_SIZE);
    if (state.archiveId !== null) renderArchivePage(); else refreshEvents();
  });
  $('olderPage').addEventListener('click', () => {
    state.offset += PAGE_SIZE;
    if (state.archiveId !== null) renderArchivePage(); else refreshEvents();
  });
  search.addEventListener('input', () => {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(resetAndRefresh, 250);
  });
  streamFilter.addEventListener('input', () => {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(resetAndRefresh, 250);
  });
  source.addEventListener('change', resetAndRefresh);
  filterFrom.addEventListener('change', resetAndRefresh);
  filterTo.addEventListener('change', resetAndRefresh);
  $('clearFilters').addEventListener('click', () => {
    search.value = '';
    streamFilter.value = '';
    source.value = '';
    filterFrom.value = '';
    filterTo.value = '';
    resetAndRefresh();
  });
  $('autoRefresh').addEventListener('change', (event) => {
    try { localStorage.setItem('cortexFacebookHistoryAutoRefresh', event.target.checked ? '1' : '0'); } catch (_) {}
  });

  $('savedChatPrograms').addEventListener('change', (event) => {
    if (event.target.value === '__current__') loadCurrentChat();
    else if (event.target.value) loadArchive(event.target.value);
  });

  $('saveCurrentChat').addEventListener('click', async (event) => {
    if (!validateDateRange()) return;
    const program = window.prompt('¿Qué número de programa quieres asignar a este chat? (Ejemplo: 11)');
    if (program === null) return;
    const normalized = program.trim();
    if (!/^\d{1,20}$/.test(normalized)) {
      showToast('Ingresa un número de programa válido.');
      return;
    }
    const button = event.currentTarget;
    button.disabled = true;
    $('archiveStatus').textContent = 'Guardando una copia de los comentarios que coinciden con los filtros…';
    try {
      const dates = dateBounds();
      const body = {
        program: normalized,
        q: search.value.trim(),
        feedId: source.value,
        streamId: streamFilter.value.trim(),
        from: dates.from,
        to: dates.to,
      };
      const archive = await request('/api/event-archives', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      showToast(`Chat guardado para el programa ${archive.program} (${archive.comments} comentarios).`);
      await refreshArchives('__current__');
    } catch (error) {
      $('archiveStatus').textContent = error.message;
    } finally {
      button.disabled = state.archiveId !== null || state.total === 0;
    }
  });

  $('deleteSavedChat').addEventListener('click', async () => {
    if (state.archiveId === null) return;
    if (!window.confirm(`¿Borrar la copia guardada del programa ${state.archiveProgram}? Esto no borra comentarios del historial activo.`)) return;
    try {
      await request('/api/event-archives/' + encodeURIComponent(state.archiveId), { method: 'DELETE' });
      showToast(`Copia del programa ${state.archiveProgram} borrada`);
      await loadCurrentChat();
      await refreshArchives('__current__');
    } catch (error) {
      showToast(error.message);
    }
  });

  $('exportEvents').addEventListener('click', () => {
    if (!validateDateRange()) return;
    if (state.archiveId !== null) {
      const csvField = (value) => {
        let text = String(value ?? '');
        if (/^[\s\u0000-\u001f]*[=+\-@]/.test(text)) text = "'" + text;
        return '"' + text.replace(/"/g, '""') + '"';
      };
      const rows = [['id', 'cuenta', 'origen', 'transmision', 'autor', 'comentario', 'hora_comentario', 'guardado_en']];
      for (const item of filteredArchiveEvents()) {
        rows.push([item.id, item.feedId, item.sourceId || '', item.streamId || '', item.author, item.message, item.eventAt, item.createdAt]);
      }
      const csv = rows.map((row) => row.map(csvField).join(',')).join('\r\n');
      const url = URL.createObjectURL(new Blob(['\ufeff', csv], { type: 'text/csv;charset=utf-8' }));
      const download = document.createElement('a');
      download.href = url;
      download.download = `cortex-facebook-programa-${state.archiveProgram}.csv`;
      download.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
      return;
    }
    const query = queryString(false);
    window.location.assign('/api/events/export.csv' + (query ? '?' + query : ''));
  });

  $('createBackup').addEventListener('click', async (event) => {
    const button = event.currentTarget;
    button.disabled = true;
    $('backupStatus').textContent = 'Creando una copia SQLite validada…';
    try {
      const result = await request('/api/backup', { method: 'POST' });
      $('backupStatus').textContent = 'Respaldo creado: ' + result.fileName + '. Las copias pueden conservar comentarios borrados.';
    } catch (error) {
      $('backupStatus').textContent = error.message;
    } finally {
      button.disabled = false;
    }
  });

  $('clearHistory').addEventListener('click', async (event) => {
    const button = event.currentTarget;
    if (!clearArmed) {
      clearArmed = true;
      button.classList.add('armed');
      button.textContent = '¿Confirmar borrado?';
      clearTimer = setTimeout(() => {
        clearArmed = false;
        button.classList.remove('armed');
      button.textContent = 'Borrar historial activo';
      }, 4000);
      return;
    }
    clearTimeout(clearTimer);
    clearArmed = false;
    button.classList.remove('armed');
    button.textContent = 'Borrar historial activo';
    button.disabled = true;
    try {
      await request('/api/events', { method: 'DELETE' });
      state.offset = 0;
      showToast('Historial activo borrado. Las copias existentes no se modificaron.');
      await refreshEvents();
    } catch (error) {
      button.disabled = false;
      showToast(error.message);
    }
  });

  const header = $('header');
  const headerToggle = $('headerToggle');
  function setHeaderMinimized(minimized) {
    header.classList.toggle('minimized', minimized);
    headerToggle.title = minimized ? 'Mostrar el encabezado' : 'Ocultar el encabezado';
    headerToggle.setAttribute('aria-pressed', minimized ? 'true' : 'false');
  }
  headerToggle.addEventListener('click', () => setHeaderMinimized(!header.classList.contains('minimized')));
  setHeaderMinimized(true);

  const moreButton = $('moreOptions');
  function setOptionsOpen(open) {
    toolbar.classList.toggle('collapsed', !open);
    moreButton.setAttribute('aria-expanded', open ? 'true' : 'false');
    try { localStorage.setItem(OPTIONS_KEY, open ? '1' : '0'); } catch (_) {}
  }
  moreButton.addEventListener('click', () => setOptionsOpen(toolbar.classList.contains('collapsed')));
  document.addEventListener('keydown', (event) => {
    if (event.key === 'Escape' && !toolbar.classList.contains('collapsed')) setOptionsOpen(false);
  });
  try { setOptionsOpen(localStorage.getItem(OPTIONS_KEY) === '1'); } catch (_) { setOptionsOpen(false); }

  const themeToggle = $('themeToggle');
  function applyTheme(theme) {
    const light = theme === 'light';
    document.documentElement.classList.toggle('theme-light', light);
    themeToggle.title = light ? 'Cambiar a tema oscuro' : 'Cambiar a tema claro';
    themeToggle.setAttribute('aria-pressed', light ? 'true' : 'false');
    try { localStorage.setItem(THEME_KEY, light ? 'light' : 'dark'); } catch (_) {}
  }
  themeToggle.addEventListener('click', () => applyTheme(document.documentElement.classList.contains('theme-light') ? 'dark' : 'light'));
  let savedTheme = 'dark';
  try { if (localStorage.getItem(THEME_KEY) === 'light') savedTheme = 'light'; } catch (_) {}
  applyTheme(savedTheme);

  let fontScale = 1.1;
  try {
    const savedScale = Number.parseFloat(localStorage.getItem(FONT_KEY));
    if (Number.isFinite(savedScale)) fontScale = Math.min(1.8, Math.max(0.8, savedScale));
  } catch (_) {}
  function applyFontScale(value) {
    fontScale = Math.min(1.8, Math.max(0.8, value));
    document.documentElement.style.setProperty('--font-scale', String(fontScale));
    $('fontLevel').textContent = Math.round(fontScale * 100) + '%';
    try { localStorage.setItem(FONT_KEY, String(fontScale)); } catch (_) {}
  }
  $('fontDown').addEventListener('click', () => applyFontScale(fontScale - 0.1));
  $('fontUp').addEventListener('click', () => applyFontScale(fontScale + 0.1));
  $('fontReset').addEventListener('click', () => applyFontScale(1.1));
  applyFontScale(fontScale);

  try { $('autoRefresh').checked = localStorage.getItem('cortexFacebookHistoryAutoRefresh') !== '0'; } catch (_) {}
  refreshEvents();
  refreshArchives();
  setInterval(() => {
    if (!$('autoRefresh').checked || state.archiveId !== null || document.hidden) return;
    if (state.offset === 0 && list.scrollTop <= 24) refreshEvents({ silent: true });
    else checkForNewComments();
  }, REFRESH_MS);
})();
