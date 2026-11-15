(() => {
  'use strict';

  const params = new URLSearchParams(location.search);
  const preview = params.get('preview') === '1';
  const backdrop = params.get('bg') || 'checkerboard';
  const clientType = preview ? 'preview' : 'overlay';
  if (preview) document.body.classList.add('preview-mode', `preview-${backdrop}`);

  const widget = document.getElementById('widget');
  const locationName = document.getElementById('location-name');
  const content = document.getElementById('content');
  const flag = document.getElementById('flag');
  const icon = document.getElementById('weather-icon');
  const value = document.getElementById('value');
  const liveBar = document.getElementById('live-bar');
  const liveText = document.getElementById('live-text');
  let config = null;
  let conditions = {};
  let activeIndex = 0;
  let lastViewKey = '';
  let hadConfig = false;
  const countryNames = { ar: 'Argentina', au: 'Australia' };

  function viewQueue() {
    const queue = [];
    for (const place of config?.locations || []) {
      if (!place.visible) continue;
      if (place.showClock) queue.push({ type: 'time', place });
      if (place.showWeather && place.city?.trim()) queue.push({ type: 'weather', place });
    }
    return queue;
  }

  function setCountryFlag(code) {
    const normalized = String(code || '').toLowerCase();
    const countryName = countryNames[normalized];
    if (!countryName) {
      flag.removeAttribute('src');
      flag.removeAttribute('title');
      flag.classList.remove('visible');
      return;
    }
    flag.alt = `Bandera de ${countryName}`;
    flag.title = countryName;
    flag.src = `/modules/clima/images/flags/${normalized}.svg`;
    flag.classList.add('visible');
  }

  flag.addEventListener('error', () => {
    flag.removeAttribute('src');
    flag.classList.remove('visible');
  });

  function formattedTime(timezone) {
    const options = { timeZone: timezone || 'UTC', hour: '2-digit', minute: '2-digit', hour12: config?.timeFormat !== '24h' };
    try { return new Intl.DateTimeFormat('es', options).format(new Date()); }
    catch { return new Intl.DateTimeFormat('es', { ...options, timeZone: 'UTC' }).format(new Date()); }
  }

  function animate() {
    const effect = config?.transitionType || 'slide-up';
    for (const node of [locationName, content]) {
      node.classList.remove('enter-slide-up', 'enter-slide-left', 'enter-slide-right', 'enter-fade', 'enter-zoom', 'enter-flip');
      void node.offsetWidth;
      node.classList.add(`enter-${effect}`);
    }
  }

  function render(forceAnimation = false) {
    if (!config) return;
    const queue = viewQueue();
    const max = queue.length;
    if (!max) {
      widget.style.display = 'none';
      if (preview) {
        widget.style.display = 'flex';
        locationName.textContent = 'SIN VISTAS ACTIVAS';
        value.textContent = 'Configura el panel';
        flag.classList.remove('visible'); icon.classList.remove('visible'); liveBar.hidden = true;
      }
      return;
    }
    widget.style.display = 'flex';
    activeIndex = Math.min(Math.max(0, activeIndex), max - 1);
    const view = queue[activeIndex];
    const place = view.place;
    widget.className = `widget-container layout-${config.layoutStyle || 'card'} pos-${config.position || 'top-left'} preset-${config.themePreset || 'classic'}`;
    document.documentElement.style.setProperty('--theme', config.themeColor || '#f59e0b');
    document.documentElement.style.setProperty('--font', `'${config.fontFamily || 'Roboto'}',system-ui,sans-serif`);
    locationName.textContent = place.text || place.city || 'Ubicación';
    liveBar.hidden = !config.showLiveBar;
    liveText.textContent = config.liveBarText || 'EN VIVO';

    let viewKey = `${view.type}:${place.id}:${view.type === 'time' ? config.timeFormat : conditions[place.id]?.updatedAt || conditions[place.id]?.temp || ''}`;
    if (view.type === 'time') {
      setCountryFlag(place.flag);
      icon.classList.remove('visible');
      value.textContent = formattedTime(place.timezone);
      value.title = `Hora local · ${place.timezone}`;
      content.setAttribute('aria-label', `Hora local ${value.textContent} en ${place.text || place.city || 'esta ubicación'}`);
    } else {
      const condition = conditions[place.id] || {};
      setCountryFlag('none');
      icon.textContent = condition.icon || '🌡️';
      icon.classList.add('visible');
      value.textContent = condition.temp || '--°C';
      value.title = condition.description || place.city || 'Clima actual';
      content.setAttribute('aria-label', `${condition.description || 'Clima actual'} ${value.textContent} en ${place.city}`);
      viewKey += `:${condition.description || ''}`;
    }
    if (forceAnimation || viewKey !== lastViewKey) animate();
    lastViewKey = viewKey;
  }

  function connect() {
    const clientID = `weather-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
    const source = new EventSource(`/api/modules/weather/events?type=${clientType}&clientId=${encodeURIComponent(clientID)}`);
    source.addEventListener('configUpdate', event => {
      try {
        const incoming = JSON.parse(event.data);
        config = incoming;
        activeIndex = Number.isInteger(incoming.activeViewIndex) ? incoming.activeViewIndex : activeIndex;
        render(hadConfig);
        hadConfig = true;
      } catch (error) { console.error('Clima y Tiempo: configuración inválida', error); }
    });
    source.addEventListener('weatherUpdate', event => {
      try {
        const incoming = JSON.parse(event.data);
        conditions = incoming.conditions || conditions;
        render(false);
      } catch (error) { console.error('Clima y Tiempo: lectura meteorológica inválida', error); }
    });
    source.addEventListener('viewChanged', event => {
      try {
        const incoming = JSON.parse(event.data);
        if (Number.isInteger(incoming.index)) { activeIndex = incoming.index; render(true); }
      } catch { /* El siguiente evento de estado vuelve a sincronizar la vista. */ }
    });
    source.onerror = () => {
      if (!config) {
        locationName.textContent = 'SIN CONEXIÓN';
        widget.style.opacity = '1';
      }
    };
    window.addEventListener('pagehide', () => source.close(), { once: true });
  }

  setInterval(() => {
    const queue = viewQueue();
    const current = queue[activeIndex];
    if (current?.type === 'time') value.textContent = formattedTime(current.place.timezone);
  }, 1000);
  connect();
})();
