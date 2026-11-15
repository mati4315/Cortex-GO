(() => {
  'use strict';

  const $ = (id) => document.getElementById(id);
  const timezoneOptions = [
    ['America/Argentina/Buenos_Aires', 'Buenos Aires · UTC−3'],
    ['Australia/Brisbane', 'Brisbane · UTC+10'],
    ['America/Lima', 'Lima · UTC−5'], ['America/Bogota', 'Bogotá · UTC−5'],
    ['America/Mexico_City', 'Ciudad de México'], ['America/Santiago', 'Santiago'],
    ['America/Caracas', 'Caracas · UTC−4'], ['America/Montevideo', 'Montevideo'],
    ['America/Asuncion', 'Asunción'], ['America/La_Paz', 'La Paz · UTC−4'],
    ['America/New_York', 'Nueva York'], ['Europe/Madrid', 'Madrid'],
    ['Europe/London', 'Londres'], ['Asia/Tokyo', 'Tokio · UTC+9'], ['UTC', 'UTC']
  ];
  const flags = [
    ['none', 'Sin bandera', '🏳️'], ['ar', 'Argentina', '🇦🇷'], ['au', 'Australia', '🇦🇺']
  ];
  const defaults = {
    locations: [], timeFormat: '12h', cycleDuration: 10, themeColor: '#f59e0b',
    themePreset: 'classic', fontFamily: 'Roboto', transitionType: 'flip',
    cyclePaused: false, activeViewIndex: 0, showLiveBar: true, liveBarText: 'EN VIVO',
    layoutStyle: 'card', position: 'top-left'
  };
  let config = structuredClone(defaults);
  let weather = {};
  let pendingChanges = {};
  let saveTimer = 0;
  let connected = false;
  let saveInFlight = false;
  let clientID = '';
  const clientType = 'dashboard';

  function newID() {
    return `loc_${(crypto.randomUUID ? crypto.randomUUID() : `${Date.now()}_${Math.random()}`).replaceAll('-', '').slice(0, 22)}`;
  }

  async function request(path, method = 'GET', body) {
    const response = await fetch(path, {
      method, cache: 'no-store',
      headers: body ? { 'Content-Type': 'application/json' } : {},
      body: body ? JSON.stringify(body) : undefined
    });
    const text = await response.text();
    let data = {};
    try { data = text ? JSON.parse(text) : {}; } catch { data = { error: text }; }
    if (!response.ok) throw new Error(data.error || `Error HTTP ${response.status}`);
    return data;
  }

  function settingsSignature(value) {
    const { activeViewIndex, ...settings } = value || {};
    return JSON.stringify(settings);
  }

  function updateSaveStatus(message, state = '') {
    const element = $('save-status');
    element.textContent = message;
    element.classList.toggle('error', state === 'error');
  }

  function applyTheme() {
    document.documentElement.style.setProperty('--accent', config.themeColor || '#f59e0b');
    const color = $('theme-color');
    const label = $('theme-color-value');
    if (color) color.value = config.themeColor || '#f59e0b';
    if (label) label.textContent = (config.themeColor || '#f59e0b').toUpperCase();
  }

  function syncControls() {
    $('cycle-duration').value = config.cycleDuration ?? 10;
    $('theme-preset').value = config.themePreset || 'classic';
    $('font-family').value = config.fontFamily || 'Roboto';
    $('transition-type').value = config.transitionType || 'slide-up';
    $('layout-style').value = config.layoutStyle || 'card';
    $('position').value = config.position || 'top-left';
    $('time-format').value = config.timeFormat || '12h';
    $('live-text').value = config.liveBarText || '';
    $('show-live').checked = Boolean(config.showLiveBar);
    $('toggle-rotation').textContent = config.cyclePaused ? '▶ Reanudar' : 'Ⅱ Pausar';
    $('rotation-state').textContent = config.cyclePaused ? 'Pausada' : 'Rotando';
    $('rotation-state').classList.toggle('running', !config.cyclePaused);
    applyTheme();
  }

  function makeField(label, control) {
    const wrapper = document.createElement('label');
    wrapper.className = 'field';
    const title = document.createElement('span');
    title.textContent = label;
    wrapper.append(title, control);
    return wrapper;
  }

  function makeSelect(options, value) {
    const select = document.createElement('select');
    for (const [key, label] of options) {
      const option = document.createElement('option');
      option.value = key;
      option.textContent = label;
      select.append(option);
    }
    if (![...select.options].some(option => option.value === value)) {
      const option = document.createElement('option');
      option.value = value || 'UTC';
      option.textContent = value || 'UTC';
      select.prepend(option);
    }
    select.value = value;
    return select;
  }

  function flagEmoji(code) {
    const found = flags.find(item => item[0] === code);
    if (found) return found[2];
    if (code && /^[a-z]{2}$/i.test(code)) return String.fromCodePoint(...code.toUpperCase().split('').map(char => char.charCodeAt(0) + 127397));
    return '🏳️';
  }

  function renderLocations() {
    const container = $('locations-list');
    container.replaceChildren();
    if (!config.locations.length) {
      const empty = document.createElement('div');
      empty.className = 'empty-locations';
      empty.textContent = 'Aún no hay ubicaciones. Añade una para empezar a preparar el overlay.';
      container.append(empty);
      return;
    }
    config.locations.forEach((location, index) => {
      const card = document.createElement('article');
      card.className = `location-card${location.visible ? '' : ' disabled'}`;
      const head = document.createElement('div');
      head.className = 'location-head';
      const title = document.createElement('div');
      title.className = 'location-title';
      const number = document.createElement('span');
      number.className = 'location-number';
      number.textContent = String(index + 1).padStart(2, '0');
      const name = document.createElement('strong');
      name.textContent = location.text || `Ubicación ${index + 1}`;
      title.append(number, name);
      const condition = weather[location.id];
      if (location.city.trim()) {
        const badge = document.createElement('span');
        badge.className = `weather-badge ${condition?.status === 'stale' ? 'stale' : condition?.status === 'unavailable' ? 'unavailable' : ''}`;
        badge.textContent = condition ? `${condition.icon || '🌡️'} ${condition.temp || '--°C'}` : '🌡️ --°C';
        badge.title = condition?.error || condition?.description || location.city;
        title.append(badge);
      }
      const actions = document.createElement('div');
      actions.className = 'location-actions';
      const up = document.createElement('button');
      up.type = 'button'; up.className = 'small-icon'; up.textContent = '↑'; up.title = 'Subir'; up.setAttribute('aria-label', 'Subir ubicación'); up.disabled = index === 0;
      up.addEventListener('click', () => moveLocation(index, -1));
      const down = document.createElement('button');
      down.type = 'button'; down.className = 'small-icon'; down.textContent = '↓'; down.title = 'Bajar'; down.setAttribute('aria-label', 'Bajar ubicación'); down.disabled = index === config.locations.length - 1;
      down.addEventListener('click', () => moveLocation(index, 1));
      const remove = document.createElement('button');
      remove.type = 'button'; remove.className = 'remove-location'; remove.textContent = 'Quitar';
      remove.addEventListener('click', () => removeLocation(index));
      actions.append(up, down, remove);
      head.append(title, actions);

      const fields = document.createElement('div');
      fields.className = 'location-fields';
      const labelInput = document.createElement('input');
      labelInput.type = 'text'; labelInput.maxLength = 48; labelInput.value = location.text;
      labelInput.addEventListener('input', () => {
        location.text = labelInput.value;
        name.textContent = location.text || `Ubicación ${index + 1}`;
        queueSave({ locations: structuredClone(config.locations) });
        renderQueue();
      });
      fields.append(makeField('Etiqueta en pantalla', labelInput));

      const timezone = makeSelect(timezoneOptions, location.timezone);
      timezone.addEventListener('change', () => { location.timezone = timezone.value; updateSettings({ locations: structuredClone(config.locations) }); });
      fields.append(makeField('Zona horaria', timezone));

      const city = document.createElement('input');
      city.type = 'text'; city.maxLength = 100; city.value = location.city; city.placeholder = 'Ej.: Gold Coast';
      city.addEventListener('input', () => {
        location.city = city.value;
        queueSave({ locations: structuredClone(config.locations) });
        renderQueue();
      });
      city.addEventListener('change', () => { location.city = city.value.trim(); city.value = location.city; updateSettings({ locations: structuredClone(config.locations) }); renderLocations(); });
      fields.append(makeField('Ciudad para el clima', city));

      const flagWrap = document.createElement('div'); flagWrap.className = 'flag-control';
      const flagPreview = document.createElement('span'); flagPreview.className = 'flag-preview'; flagPreview.textContent = flagEmoji(location.flag);
      const flagSelect = makeSelect(flags.map(item => [item[0], `${item[2]}  ${item[1]}`]), location.flag);
      flagSelect.addEventListener('change', () => {
        location.flag = flagSelect.value;
        flagPreview.textContent = flagEmoji(location.flag);
        updateSettings({ locations: structuredClone(config.locations) });
      });
      flagWrap.append(flagPreview, flagSelect);
      fields.append(makeField('Bandera', flagWrap));

      const toggles = document.createElement('div'); toggles.className = 'switch-line';
      toggles.append(
        makeMiniSwitch('Mostrar ubicación', location.visible, value => { location.visible = value; updateSettings({ locations: structuredClone(config.locations) }); renderLocations(); }),
        makeMiniSwitch('Reloj', location.showClock, value => { location.showClock = value; updateSettings({ locations: structuredClone(config.locations) }); renderQueue(); }),
        makeMiniSwitch('Clima', location.showWeather, value => { location.showWeather = value; updateSettings({ locations: structuredClone(config.locations) }); renderQueue(); })
      );
      card.append(head, fields, toggles);
      container.append(card);
    });
  }

  function makeMiniSwitch(label, checked, onChange) {
    const wrapper = document.createElement('label'); wrapper.className = 'mini-switch';
    const input = document.createElement('input'); input.type = 'checkbox'; input.checked = Boolean(checked);
    input.addEventListener('change', () => onChange(input.checked));
    const text = document.createElement('span'); text.textContent = label;
    wrapper.append(input, text);
    return wrapper;
  }

  function buildQueue() {
    const queue = [];
    for (const location of config.locations) {
      if (!location.visible) continue;
      if (location.showClock) queue.push({ type: 'time', location, label: `${location.text} · Hora` });
      if (location.showWeather && location.city.trim()) queue.push({ type: 'weather', location, label: `${location.text} · ${location.city}` });
    }
    return queue;
  }

  function renderQueue() {
    const container = $('view-queue');
    const queue = buildQueue();
    container.replaceChildren();
    if (!queue.length) {
      const empty = document.createElement('div'); empty.className = 'queue-empty'; empty.textContent = 'No hay vistas activas. Activa el reloj o el clima en alguna ubicación.';
      container.append(empty);
      return;
    }
    if (config.activeViewIndex >= queue.length) config.activeViewIndex = 0;
    queue.forEach((view, index) => {
      const button = document.createElement('button'); button.type = 'button';
      button.className = `queue-item${index === config.activeViewIndex ? ' active' : ''}`;
      const label = document.createElement('span'); label.className = 'queue-label'; label.textContent = view.label;
      const kind = document.createElement('span'); kind.className = 'queue-kind'; kind.textContent = view.type === 'time' ? `◷ ${config.timeFormat === '24h' ? '24 h' : '12 h'}` : `${weather[view.location.id]?.icon || '🌡️'} ${weather[view.location.id]?.temp || '--°C'}`;
      button.append(label, kind);
      button.addEventListener('click', () => control('select', index));
      container.append(button);
    });
  }

  function queueSave(patch) {
    Object.assign(pendingChanges, patch);
    updateSaveStatus('Cambios pendientes…');
    clearTimeout(saveTimer);
    saveTimer = setTimeout(flushChanges, 450);
  }

  function updateSettings(patch) {
    config = { ...config, ...patch };
    if (Object.hasOwn(patch, 'themeColor')) applyTheme();
    if (Object.hasOwn(patch, 'cycleDuration')) $('cycle-duration').value = config.cycleDuration;
    if (Object.hasOwn(patch, 'cyclePaused')) syncControls();
    queueSave(patch);
    renderQueue();
  }

  async function flushChanges() {
    clearTimeout(saveTimer);
    if (saveInFlight || !Object.keys(pendingChanges).length) return;
    const changes = pendingChanges;
    pendingChanges = {};
    saveInFlight = true;
    let retryLater = false;
    updateSaveStatus('Guardando…');
    try {
      const response = await request('/api/modules/weather/settings', 'PUT', { changes });
      config = { ...config, ...response.config };
      updateSaveStatus('Guardado · cambios en vivo');
    } catch (error) {
      Object.assign(pendingChanges, changes);
      retryLater = true;
      updateSaveStatus(error.message, 'error');
    } finally {
      saveInFlight = false;
      if (Object.keys(pendingChanges).length) setTimeout(flushChanges, retryLater ? 1800 : 100);
    }
  }

  function addLocation() {
    if (config.locations.length >= 20) { updateSaveStatus('Ya hay 20 ubicaciones configuradas.', 'error'); return; }
    config.locations.push({
      id: newID(), text: `Nueva ubicación ${config.locations.length + 1}`,
      timezone: 'America/Argentina/Buenos_Aires', city: '', visible: true, flag: 'none',
      showClock: true, showWeather: true
    });
    renderLocations(); renderQueue();
    updateSettings({ locations: structuredClone(config.locations) });
  }

  function moveLocation(index, direction) {
    const target = index + direction;
    if (target < 0 || target >= config.locations.length) return;
    [config.locations[index], config.locations[target]] = [config.locations[target], config.locations[index]];
    renderLocations(); renderQueue();
    updateSettings({ locations: structuredClone(config.locations) });
  }

  function removeLocation(index) {
    if (config.locations.length > 1 && !window.confirm(`¿Quitar «${config.locations[index].text}» del overlay?`)) return;
    config.locations.splice(index, 1);
    renderLocations(); renderQueue();
    updateSettings({ locations: structuredClone(config.locations) });
  }

  async function control(action, index) {
    try {
      const response = await request('/api/modules/weather/control', 'POST', { action, ...(index === undefined ? {} : { index }) });
      config = { ...config, ...response.config };
      syncControls(); renderQueue();
    } catch (error) { updateSaveStatus(error.message, 'error'); }
  }

  function applyWeather(payload) {
    weather = payload?.conditions || weather;
    renderLocations(); renderQueue();
    const status = $('weather-status');
    status.classList.remove('loading', 'offline');
    if (payload?.refreshing) {
      status.classList.add('loading'); status.innerHTML = '<i></i> Actualizando clima…';
    } else if (payload?.lastError) {
      status.classList.add('loading'); status.innerHTML = '<i></i> Lecturas disponibles · hay errores';
      status.title = payload.lastError;
    } else if (payload?.lastRefresh) {
      status.innerHTML = '<i></i> Clima actualizado';
      status.title = `Última actualización: ${new Date(payload.lastRefresh).toLocaleString()}`;
    } else {
      status.innerHTML = '<i></i> Clima pendiente';
    }
  }

  function applyConnections(value) {
    const overlayCount = Number(value?.overlays || 0);
    const dashboardCount = Number(value?.dashboards || 0);
    $('overlay-count').textContent = `${overlayCount} ${overlayCount === 1 ? 'overlay conectado' : 'overlays conectados'}`;
    $('connection-detail').textContent = `${overlayCount} overlay${overlayCount === 1 ? '' : 's'} · ${dashboardCount} panel${dashboardCount === 1 ? '' : 'es'} conectado${dashboardCount === 1 ? '' : 's'}`;
  }

  function connectEvents() {
    clientID = `weather-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
    const source = new EventSource(`/api/modules/weather/events?type=${clientType}&clientId=${encodeURIComponent(clientID)}`);
    source.onopen = () => {
      connected = true;
      const pill = $('connection-status'); pill.classList.remove('offline', 'loading');
      pill.innerHTML = '<i></i> Panel conectado en tiempo real';
    };
    source.onerror = () => {
      connected = false;
      const pill = $('connection-status'); pill.classList.add('offline');
      pill.innerHTML = '<i></i> Reconectando…';
    };
    source.addEventListener('configUpdate', event => {
      try {
        const incoming = JSON.parse(event.data);
        if (settingsSignature(incoming) !== settingsSignature(config)) {
          config = { ...defaults, ...incoming };
          syncControls(); renderLocations();
        }
        config.activeViewIndex = Number(incoming.activeViewIndex || 0);
        syncControls(); renderQueue();
      } catch (error) { console.error('Clima y Tiempo: configuración SSE inválida', error); }
    });
    source.addEventListener('weatherUpdate', event => {
      try { applyWeather(JSON.parse(event.data)); } catch (error) { console.error('Clima y Tiempo: estado meteorológico inválido', error); }
    });
    source.addEventListener('connectionStatus', event => {
      try { applyConnections(JSON.parse(event.data)); } catch { /* Evento incompleto; llegará el siguiente. */ }
    });
    source.addEventListener('viewChanged', event => {
      try { config.activeViewIndex = Number(JSON.parse(event.data).index || 0); renderQueue(); } catch { /* Se sincroniza desde configUpdate. */ }
    });
    window.addEventListener('pagehide', () => source.close(), { once: true });
  }

  function bindControls() {
    $('add-location').addEventListener('click', addLocation);
    $('previous-view').addEventListener('click', () => control('previous'));
    $('next-view').addEventListener('click', () => control('next'));
    $('toggle-rotation').addEventListener('click', () => control('toggle'));
    $('cycle-duration').addEventListener('change', event => {
      const value = Math.min(60, Math.max(3, Number(event.target.value) || 10));
      event.target.value = value; updateSettings({ cycleDuration: value });
    });
    $('theme-color').addEventListener('input', event => updateSettings({ themeColor: event.target.value }));
    document.querySelectorAll('[data-color]').forEach(button => button.addEventListener('click', () => updateSettings({ themeColor: button.dataset.color })));
    for (const [id, key] of [['theme-preset','themePreset'],['font-family','fontFamily'],['transition-type','transitionType'],['layout-style','layoutStyle'],['position','position'],['time-format','timeFormat']]) {
      $(id).addEventListener('change', event => updateSettings({ [key]: event.target.value }));
    }
    $('live-text').addEventListener('input', event => updateSettings({ liveBarText: event.target.value }));
    $('show-live').addEventListener('change', event => updateSettings({ showLiveBar: event.target.checked }));
    $('save-now').addEventListener('click', () => { clearTimeout(saveTimer); flushChanges(); });
    $('refresh-weather').addEventListener('click', async event => {
      const button = event.currentTarget; button.disabled = true;
      try { await request('/api/modules/weather/refresh', 'POST', {}); applyWeather({ conditions: weather, refreshing: true }); }
      catch (error) { updateSaveStatus(error.message, 'error'); }
      setTimeout(() => { button.disabled = false; }, 1300);
    });
    document.querySelectorAll('[data-bg]').forEach(button => button.addEventListener('click', () => {
      const backdrop = button.dataset.bg;
      $('preview-frame-wrap').dataset.backdrop = backdrop;
      document.querySelectorAll('[data-bg]').forEach(item => item.classList.toggle('selected', item === button));
      $('preview-frame').src = `/modules/clima/overlay?preview=1&bg=${encodeURIComponent(backdrop)}`;
    }));
  }

  async function initialize() {
    bindControls();
    try {
      const state = await request('/api/modules/weather/state');
      config = { ...defaults, ...state.config };
      weather = state.weather || {};
      syncControls(); renderLocations(); renderQueue();
      applyWeather({ conditions: weather, refreshing: state.refreshing, lastRefresh: state.lastRefresh, lastError: state.lastError });
      applyConnections(state.connections);
      updateSaveStatus('Ajustes listos');
    } catch (error) {
      updateSaveStatus(`No se pudo cargar el panel: ${error.message}`, 'error');
    }
    connectEvents();
  }

  initialize();
})();
