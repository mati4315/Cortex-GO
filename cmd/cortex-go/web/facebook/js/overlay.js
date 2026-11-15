(() => {
  'use strict';

  const params = new URLSearchParams(location.search);
  const list = document.getElementById('messages');
  const status = document.getElementById('connection-status');
  const featuredSpotlight = document.getElementById('featured-spotlight');
  const ruloUpdateNotice = document.getElementById('rulo-update-notice');
  const ruloUpdateText = document.getElementById('rulo-update-text');
  const preview = params.get('preview') === '1';
  const debug = params.get('debug') === '1';
  const maxMessages = integerParam('limit', 20, 1, 60);
  const showtime = integerParam('showtime', 30000, 0, 3600000);
  const textScrollDelay = integerParam('scrollDelay', 4500, 0, 20000);
  const textScrollSpeed = numberParam('scrollSpeed', 18, 4, 60);
  const textScrollEndHold = 1600;
  const fadeZone = integerParam('fadezone', 160, 0, 1000);
  const width = integerParam('width', 350, 240, 900);
  const chatHeight = integerParam('chatHeight', 260, 80, 1000);
  const featuredHeight = integerParam('featuredHeight', 360, 100, 1200);
  const scale = numberParam('scale', 1, 0.65, 1.8);
  const feedFilter = ['a', 'b'].includes(params.get('feedId')) ? params.get('feedId') : '';
  const streamFilter = (params.get('streamId') || '').slice(0, 128);
  const showTimeLabel = params.get('time') === '1';
  let source = null;
  let lastEventID = 0;
  let openedOnce = false;
  let featuredTimer = 0;
  let featuredHideTimer = 0;
  let featuredTextScroll = null;
  let ruloNoticeTimer = 0;
  let ruloNoticeHideTimer = 0;
  let ruloSequence = 0;
  let ruloLoaded = false;
  let ruloMuted = false;
  let ruloMuteRevision = 0;
  let ruloRecoveryBusy = false;
  let ruloRecoveryPending = false;
  const ruloVersions = new Map();
  const ruloCards = new Map();
  const featuredEventIDs = new Set();

  document.documentElement.style.setProperty('--overlay-width', `${width}px`);
  document.documentElement.style.setProperty('--overlay-chat-height', `${chatHeight}px`);
  document.documentElement.style.setProperty('--overlay-featured-height', `${featuredHeight}px`);
  document.documentElement.style.setProperty('--overlay-scale', String(scale));
  document.documentElement.style.setProperty('--fade-zone', `${fadeZone}px`);
  document.body.classList.toggle('reverse', params.has('reverse'));
  document.body.classList.toggle('no-fade', fadeZone === 0);
  document.body.classList.toggle('preview', preview);
  status.hidden = !debug;

  function integerParam(name, fallback, min, max) {
    const value = Number.parseInt(params.get(name), 10);
    return Number.isFinite(value) ? Math.min(max, Math.max(min, value)) : fallback;
  }

  function numberParam(name, fallback, min, max) {
    const value = Number.parseFloat(params.get(name));
    return Number.isFinite(value) ? Math.min(max, Math.max(min, value)) : fallback;
  }

  function setStatus(message) {
    if (debug) status.textContent = message;
  }

  function decodeEntities(value) {
    const textarea = document.createElement('textarea');
    textarea.innerHTML = value;
    return textarea.value;
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

  function removeMessage(message) {
    if (!message) return;
    message.textScrollController?.cancel();
    clearTimeout(message.expiryTimer);
    clearTimeout(message.removeTimer);
    if (message.dataset.ruloResponseId) ruloCards.delete(message.dataset.ruloResponseId);
    if (message.isConnected) message.remove();
  }

  function rememberRuloVersion(responseID, version) {
    ruloVersions.delete(responseID);
    ruloVersions.set(responseID, version);
    while (ruloVersions.size > 300) ruloVersions.delete(ruloVersions.keys().next().value);
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

  function accountName(feedID) {
    return feedID === 'b' ? 'Cuenta 2' : 'Cuenta 1';
  }

  function addMessage(event) {
    const created = Date.parse(event.createdAt || '');
    const age = Number.isFinite(created) ? Math.max(0, Date.now() - created) : 0;
    if (showtime > 0 && age >= showtime) return;

    const feedID = event.feedId === 'b' ? 'b' : 'a';
    const card = document.createElement('article');
    card.className = `message account-${feedID}`;
    card.dataset.eventId = String(event.id);

    const heading = document.createElement('div');
    heading.className = 'message-head';
    const author = document.createElement('span');
    author.className = 'name';
    author.textContent = event.author || 'Usuario de Facebook';
    const account = document.createElement('span');
    account.className = `account account-${feedID}`;
    account.textContent = accountName(feedID);
    heading.append(author, account);

    const message = document.createElement('p');
    message.className = 'text';
    message.textContent = displayComment(event.message);
    card.append(heading, message);

    if (showTimeLabel) {
      const time = document.createElement('time');
      time.className = 'time';
      const timestamp = Date.parse(event.eventAt || event.createdAt || '');
      time.textContent = Number.isFinite(timestamp)
        ? new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit' }).format(timestamp)
        : '';
      if (time.textContent) card.append(time);
    }

    const first = list.querySelector('.message:not(#preview-message)');
    if (params.has('reverse') && first) list.insertBefore(card, first);
    else list.append(card);
    card.textScrollController = startSlowScroll(message);
    requestAnimationFrame(() => card.classList.add('visible'));

    const messages = [...list.querySelectorAll('.message:not(#preview-message)')];
    while (messages.length > maxMessages) {
      const oldest = params.has('reverse') ? messages.pop() : messages.shift();
      removeMessage(oldest);
    }

    if (showtime > 0) {
      const remaining = Math.max(0, showtime - age);
      card.expiryTimer = window.setTimeout(() => {
        card.classList.add('fading');
        card.removeTimer = window.setTimeout(() => removeMessage(card), 500);
      }, remaining);
    }
  }

  function addRuloResponse(response, isUpdate = false) {
    const responseID = String(response?.responseId || '');
    const eventID = Number(response?.eventId);
    const version = Number(response?.version);
    const text = String(response?.text || '').trim();
    if (!responseID || !Number.isSafeInteger(eventID) || eventID < 1 || !Number.isSafeInteger(version) || version < 1 || !text || ruloMuted) return;
    if (feedFilter && response.feedId !== feedFilter) return;
    if (streamFilter && response.streamId !== streamFilter) return;
    if ((ruloVersions.get(responseID) || 0) >= version) return;

    const existing = ruloCards.get(responseID);
    if (isUpdate) {
      rememberRuloVersion(responseID, version);
      if (existing?.isConnected) {
        const body = existing.querySelector('.text');
        if (body) {
          existing.textScrollController?.cancel();
          body.textContent = text;
          existing.textScrollController = startSlowScroll(body);
          existing.dataset.version = String(version);
        }
      }
      showRuloUpdate(text);
      return;
    }

    const updated = Date.parse(response.updatedAt || response.createdAt || '');
    const age = Number.isFinite(updated) ? Math.max(0, Date.now() - updated) : 0;
    if (showtime > 0 && age >= showtime) {
      rememberRuloVersion(responseID, version);
      return;
    }

    if (existing?.isConnected) {
      existing.querySelector('.text').textContent = text;
      existing.dataset.version = String(version);
      rememberRuloVersion(responseID, version);
      return;
    }

    const card = document.createElement('article');
    const signal = ['warning', 'error', 'success', 'speaking'].includes(response.signal) ? response.signal : 'speaking';
    card.className = `message rulo-response signal-${signal}`;
    card.dataset.eventId = String(eventID);
    card.dataset.ruloResponseId = responseID;
    card.dataset.version = String(version);
    const heading = document.createElement('div');
    heading.className = 'message-head';
    const author = document.createElement('span');
    author.className = 'name';
    author.textContent = 'Rulo';
    const badge = document.createElement('span');
    badge.className = 'account';
    badge.textContent = 'Respuesta';
    heading.append(author, badge);
    const body = document.createElement('p');
    body.className = 'text';
    body.textContent = text;
    card.append(heading, body);

    const first = list.querySelector('.message:not(#preview-message)');
    if (params.has('reverse') && first) list.insertBefore(card, first);
    else list.append(card);
    card.textScrollController = startSlowScroll(body);
    requestAnimationFrame(() => card.classList.add('visible'));
    ruloCards.set(responseID, card);
    rememberRuloVersion(responseID, version);

    const messages = [...list.querySelectorAll('.message:not(#preview-message)')];
    while (messages.length > maxMessages) {
      const oldest = params.has('reverse') ? messages.pop() : messages.shift();
      removeMessage(oldest);
    }
    if (showtime > 0) {
      card.expiryTimer = window.setTimeout(() => {
        card.classList.add('fading');
        card.removeTimer = window.setTimeout(() => removeMessage(card), 500);
      }, Math.max(0, showtime - age));
    }
  }

  function showRuloUpdate(value) {
    const compact = Array.from(String(value || '').replace(/\s+/g, ' ').trim());
    ruloUpdateText.textContent = compact.length > 140 ? `${compact.slice(0, 137).join('')}…` : compact.join('');
    ruloUpdateNotice.hidden = false;
    ruloUpdateNotice.classList.remove('fading');
    clearTimeout(ruloNoticeTimer);
    clearTimeout(ruloNoticeHideTimer);
    ruloNoticeTimer = window.setTimeout(() => {
      ruloUpdateNotice.classList.add('fading');
      ruloNoticeHideTimer = window.setTimeout(() => { ruloUpdateNotice.hidden = true; }, 280);
    }, 6500);
  }

  async function recoverRuloResponses(reconnecting) {
    if (ruloRecoveryBusy) {
      ruloRecoveryPending = true;
      return;
    }
    ruloRecoveryBusy = true;
    const muteRevision = ruloMuteRevision;
    try {
      if (!ruloLoaded) {
        const response = await fetch('/api/modules/rulo/responses?limit=50', { cache: 'no-store' });
        if (!response.ok) throw new Error('No se pudieron recuperar las respuestas de Rulo');
        const result = await response.json();
        if (muteRevision === ruloMuteRevision) ruloMuted = Boolean(result.presentationMuted);
        ruloSequence = Math.max(ruloSequence, Number(result.cursor) || 0);
        if (!ruloMuted) (result.responses || []).forEach(item => addRuloResponse(item));
        ruloLoaded = true;
      } else if (reconnecting) {
        let pages = 0;
        while (pages < 9) {
          const response = await fetch(`/api/modules/rulo/responses?afterSequence=${ruloSequence}&limit=250`, { cache: 'no-store' });
          if (!response.ok) throw new Error('No se pudieron recuperar las respuestas de Rulo');
          const result = await response.json();
          if (muteRevision === ruloMuteRevision) ruloMuted = Boolean(result.presentationMuted);
          if (ruloMuted) {
            ruloSequence = Math.max(ruloSequence, Number(result.cursor) || 0);
            break;
          }
          const updates = result.updates || [];
          updates.forEach(item => addRuloResponse(item.response, Number(item.response?.version) > 1));
          if (updates.length) ruloSequence = Math.max(ruloSequence, Number(updates[updates.length - 1].sequence) || 0);
          if (updates.length < 250) {
            ruloSequence = Math.max(ruloSequence, Number(result.cursor) || 0);
            break;
          }
          pages += 1;
        }
      }
    } catch (error) {
      setStatus(error.message);
    } finally {
      ruloRecoveryBusy = false;
      if (ruloRecoveryPending) {
        ruloRecoveryPending = false;
        recoverRuloResponses(true);
      }
    }
  }

  function onRuloResponse(event) {
    try {
      const payload = JSON.parse(event.data);
      const sequence = Number(payload.sequence) || 0;
      if (sequence > 0 && sequence <= ruloSequence) return;
      recoverRuloResponses(true);
    } catch {
      setStatus('Se recibió una respuesta de Rulo ilegible');
    }
  }

  function onRuloPresentation(event) {
    try {
      const payload = JSON.parse(event.data);
      ruloMuteRevision += 1;
      ruloMuted = Boolean(payload.muted);
      ruloSequence = Math.max(ruloSequence, Number(payload.cursor) || 0);
      if (ruloMuted) {
        list.querySelectorAll('.rulo-response').forEach(removeMessage);
        ruloVersions.clear();
        clearTimeout(ruloNoticeTimer);
        clearTimeout(ruloNoticeHideTimer);
        ruloUpdateNotice.hidden = true;
      }
    } catch {
      setStatus('No se pudo sincronizar el silencio de Rulo');
    }
  }

  function onReady(event) {
    try {
      const payload = JSON.parse(event.data);
      if (!payload.reconnecting) lastEventID = Number(payload.cursor) || 0;
      setStatus(payload.reconnecting ? 'Conectado · recuperando comentarios' : 'Conectado · esperando comentarios nuevos');
      recoverRuloResponses(Boolean(payload.reconnecting));
    } catch {
      setStatus('Conectado');
    }
  }

  function onComment(event) {
    let comment;
    try { comment = JSON.parse(event.data); }
    catch { setStatus('Se recibió un comentario ilegible'); return; }

    const eventID = Number(comment.id);
    if (!Number.isSafeInteger(eventID) || eventID <= lastEventID) return;
    lastEventID = eventID;
    if (feedFilter && comment.feedId !== feedFilter) return;
    if (streamFilter && comment.streamId !== streamFilter) return;
    if (featuredEventIDs.has(eventID)) return;
    addMessage(comment);
  }

  function showFeatured(payload) {
    const event = payload?.event;
    if (!event) return;
    const featuredAt = Date.parse(payload.featuredAt || '') || Date.now();
    const age = Math.max(0, Date.now() - featuredAt);
    if (showtime > 0 && age >= showtime) return;

    const featuredEventID = Number(event.id) || 0;
    if (featuredEventID) {
      featuredEventIDs.add(featuredEventID);
      if (featuredEventIDs.size > 100) featuredEventIDs.delete(featuredEventIDs.values().next().value);
      removeMessage(list.querySelector(`.message[data-event-id="${featuredEventID}"]`));
    }

    document.getElementById('featured-author').textContent = event.author || 'Usuario de Facebook';
    document.getElementById('featured-account').textContent = accountName(event.feedId);
    const featuredText = document.getElementById('featured-text');
    featuredText.textContent = displayComment(event.message);
    const eventTime = Date.parse(event.eventAt || event.createdAt || '');
    document.getElementById('featured-time').textContent = Number.isFinite(eventTime)
      ? new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit' }).format(eventTime)
      : '';
    clearTimeout(featuredHideTimer);
    clearTimeout(featuredTimer);
    featuredTextScroll?.cancel();
    featuredSpotlight.hidden = false;
    featuredSpotlight.classList.remove('visible');
    requestAnimationFrame(() => featuredSpotlight.classList.add('visible'));
    featuredTextScroll = startSlowScroll(featuredText);
    if (showtime > 0) {
      const remaining = Math.max(0, showtime - age, featuredTextScroll.completionMs);
      featuredTimer = window.setTimeout(hideFeatured, remaining);
    }
    setStatus('Comentario destacado en pantalla');
  }

  function hideFeatured() {
    if (featuredSpotlight.hidden) return;
    clearTimeout(featuredTimer);
    clearTimeout(featuredHideTimer);
    featuredTextScroll?.cancel();
    featuredTextScroll = null;
    featuredSpotlight.classList.remove('visible');
    featuredHideTimer = window.setTimeout(() => { featuredSpotlight.hidden = true; }, 380);
  }

  function connect() {
    if (preview || source) return;
    const clientID = `facebook-overlay-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
    source = new EventSource(`/api/modules/facebook/events?type=overlay&clientId=${encodeURIComponent(clientID)}`);
    source.addEventListener('ready', onReady);
    source.addEventListener('comment', onComment);
    source.addEventListener('featured', event => {
      try { showFeatured(JSON.parse(event.data)); }
      catch { setStatus('No se pudo mostrar el comentario destacado'); }
    });
    source.addEventListener('ruloResponse', onRuloResponse);
    source.addEventListener('ruloResponseUpdate', onRuloResponse);
    source.addEventListener('ruloPresentation', onRuloPresentation);
    source.addEventListener('streamError', event => {
      try { setStatus(JSON.parse(event.data).message || 'Error de conexión'); }
      catch { setStatus('Error de conexión'); }
    });
    source.onopen = () => {
      openedOnce = true;
      setStatus('Conectando con Cortex…');
    };
    source.onerror = () => setStatus(openedOnce ? 'Reconectando con Cortex…' : 'Esperando a Cortex…');
  }

  function disconnect() {
    if (source) {
      source.close();
      source = null;
    }
    clearTimeout(featuredTimer);
    clearTimeout(featuredHideTimer);
    featuredTextScroll?.cancel();
    featuredTextScroll = null;
    const previewCard = document.getElementById('preview-message');
    previewCard?.textScrollController?.cancel();
  }

  window.addEventListener('pagehide', disconnect, { once: true });
  if (preview && params.get('sample') === 'long') {
    const longText = [
      'Este ejemplo permite revisar cómo se muestra un comentario largo en OBS. El texto conserva los saltos de línea y, si supera el espacio disponible, primero espera para que puedas leer el comienzo.',
      'Después baja lentamente hasta el final. Ajusta la pausa y la velocidad desde Más opciones en el panel de Cortex; el comentario destacado permanece visible hasta terminar el recorrido.',
      'La vista previa muestra solo la tarjeta destacada para que el mismo comentario no aparezca dos veces.',
    ].join('\n\n');
    const sampleCard = document.getElementById('preview-message');
    const sampleText = sampleCard.querySelector('.text');
    document.body.classList.add('preview-featured-only');
    sampleCard.hidden = true;
    sampleText.textContent = longText;
    const now = new Date().toISOString();
    showFeatured({
      event: { author: 'María Ejemplo', feedId: 'a', message: longText, eventAt: now, createdAt: now },
      featuredAt: now,
    });
  }
  connect();
})();
