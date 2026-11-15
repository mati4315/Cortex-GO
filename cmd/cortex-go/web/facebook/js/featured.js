(() => {
  'use strict';

  const params = new URLSearchParams(location.search);
  const feature = document.getElementById('feature');
  const status = document.getElementById('status');
  const debug = params.get('debug') === '1';
  const preview = params.get('preview') === '1';
  const showtime = readShowtime();
  const featuredHeight = readIntegerParam('featuredHeight', 360, 100, 1200);
  const textScrollDelay = readIntegerParam('scrollDelay', 4500, 0, 20000);
  const textScrollSpeed = readNumberParam('scrollSpeed', 18, 4, 60);
  const textScrollEndHold = 1600;
  let timer = 0;
  let hideTimer = 0;
  let textScroll = null;
  let source = null;
  let openedOnce = false;
  status.hidden = !debug;
  document.body.classList.toggle('preview', preview);
  document.documentElement.style.setProperty('--featured-height', `${featuredHeight}px`);

  function readShowtime() {
    const raw = params.get('showtime') || params.get('timer');
    if (raw === null || raw === '') return 30000;
    const value = Number.parseInt(raw, 10);
    return Number.isFinite(value) ? Math.min(3600000, Math.max(0, value)) : 30000;
  }

  function readIntegerParam(name, fallback, min, max) {
    const value = Number.parseInt(params.get(name), 10);
    return Number.isFinite(value) ? Math.min(max, Math.max(min, value)) : fallback;
  }

  function readNumberParam(name, fallback, min, max) {
    const value = Number.parseFloat(params.get(name));
    return Number.isFinite(value) ? Math.min(max, Math.max(min, value)) : fallback;
  }

  function setStatus(text) {
    if (debug) status.textContent = text;
  }

  function accountName(feedID) {
    return feedID === 'b' ? 'Cuenta 2 · Facebook' : 'Cuenta 1 · Facebook';
  }

  function startSlowScroll(element) {
    element.scrollTop = 0;
    const distance = Math.max(0, element.scrollHeight - element.clientHeight);
    if (distance < 2) return { completionMs: 0, cancel() {} };

    const travelMs = Math.max(1000, Math.ceil(distance / textScrollSpeed * 1000));
    let delayTimer = 0;
    let frame = 0;
    let cancelled = false;
    element.classList.add('scroll-pending');
    delayTimer = window.setTimeout(() => {
      if (cancelled) return;
      element.classList.remove('scroll-pending');
      element.classList.add('scroll-active');
      const startedAt = performance.now();
      const step = (now) => {
        if (cancelled) return;
        const progress = Math.min(1, (now - startedAt) / travelMs);
        element.scrollTop = distance * progress;
        if (progress < 1) frame = requestAnimationFrame(step);
        else element.classList.remove('scroll-active');
      };
      frame = requestAnimationFrame(step);
    }, textScrollDelay);

    return {
      completionMs: textScrollDelay + travelMs + textScrollEndHold,
      cancel() {
        cancelled = true;
        clearTimeout(delayTimer);
        cancelAnimationFrame(frame);
        element.classList.remove('scroll-pending', 'scroll-active');
      },
    };
  }

  function show(payload) {
    const event = payload?.event;
    if (!event) return;
    const featuredAt = Date.parse(payload.featuredAt || '') || Date.now();
    const age = Math.max(0, Date.now() - featuredAt);
    if (showtime > 0 && age >= showtime) return;

    document.getElementById('author').textContent = event.author || 'Usuario de Facebook';
    document.getElementById('account').textContent = accountName(event.feedId);
    const message = document.getElementById('message');
    message.textContent = displayComment(event.message);
    const eventTime = Date.parse(event.eventAt || event.createdAt || '');
    document.getElementById('time').textContent = Number.isFinite(eventTime)
      ? new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit' }).format(eventTime)
      : '';
    clearTimeout(timer);
    clearTimeout(hideTimer);
    textScroll?.cancel();
    feature.hidden = false;
    feature.classList.remove('visible');
    requestAnimationFrame(() => feature.classList.add('visible'));
    textScroll = startSlowScroll(message);
    if (showtime > 0) timer = window.setTimeout(hide, Math.max(0, showtime - age, textScroll.completionMs));
    setStatus('Comentario destacado');
  }

  function hide() {
    if (feature.hidden) return;
    clearTimeout(timer);
    clearTimeout(hideTimer);
    textScroll?.cancel();
    textScroll = null;
    feature.classList.remove('visible');
    hideTimer = window.setTimeout(() => {
      if (!feature.classList.contains('visible')) feature.hidden = true;
    }, 380);
  }

  function displayComment(value) {
    let text = String(value || '');
    text = text.replace(/<img\b[^>]*>/gi, tag => {
      const alt = /\balt\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))/i.exec(tag);
      return alt ? decodeEntities(alt[1] ?? alt[2] ?? alt[3] ?? '') : '';
    });
    text = text.replace(/<br\s*\/?>/gi, '\n').replace(/<\/(?:div|p|li)\s*>/gi, '\n').replace(/<[^>]*>/g, '');
    return decodeEntities(text).replace(/[ \t]+\n/g, '\n').replace(/\n{3,}/g, '\n\n').trim();
  }

  function decodeEntities(value) {
    const textarea = document.createElement('textarea');
    textarea.innerHTML = value;
    return textarea.value;
  }

  function connect() {
    if (preview || source) return;
    const clientID = `facebook-featured-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
    source = new EventSource(`/api/modules/facebook/featured/events?type=featured&clientId=${encodeURIComponent(clientID)}`);
    source.addEventListener('ready', () => setStatus('Conectado · esperando destacados'));
    source.addEventListener('featured', event => {
      try { show(JSON.parse(event.data)); }
      catch { setStatus('No se pudo mostrar el comentario destacado'); }
    });
    source.onerror = () => setStatus(openedOnce ? 'Reconectando con Cortex…' : 'Esperando a Cortex…');
    source.onopen = () => { openedOnce = true; setStatus('Conectando con Cortex…'); };
  }

  function disconnect() {
    if (source) {
      source.close();
      source = null;
    }
    clearTimeout(timer);
    clearTimeout(hideTimer);
    textScroll?.cancel();
    textScroll = null;
  }

  if (preview) {
    document.body.classList.add('preview');
    const now = new Date().toISOString();
    const sampleMessage = params.get('sample') === 'long'
      ? [
        'Esta es una vista previa de un comentario largo. Sirve para comprobar cómo se lee el texto y cuánto espacio ocupa dentro de la fuente de navegador de OBS.',
        'El comentario conserva sus saltos de línea y se desplaza automáticamente cuando supera la altura disponible. Primero queda quieto para que se pueda leer el comienzo; después avanza lentamente hacia el final.',
        'La pausa y la velocidad se pueden ajustar en la dirección de la fuente con scrollDelay, expresado en milisegundos, y scrollSpeed, expresado en píxeles por segundo. También se puede abrir esta vista desde un teléfono para revisar el tamaño del texto.',
        'En el chat normal la tarjeta conserva el tiempo configurado para que varios comentarios extensos no se acumulen. Al destacar uno desde el historial, el overlay central permanece visible hasta completar el recorrido y permite leer el comentario entero.',
        'Este texto de ejemplo es intencionalmente largo. Deja que la animación avance para revisar la altura del área de lectura, el contraste, el ritmo y la pausa final en tu escena real de OBS.',
      ].join('\n\n')
      : '¡Qué buen programa! 👏';
    show({ event: { author: 'María Ejemplo', feedId: 'a', message: sampleMessage, eventAt: now, createdAt: now }, featuredAt: now });
  } else {
    connect();
  }
  window.addEventListener('pagehide', disconnect, { once: true });
})();
