(() => {
  const listeners = new Map();
  const latestState = new Map();
  let pendingConfigRequest = null;
  let pendingResolvers = [];
  let configWriteActive = false;
  let activeConfigWriteKind = null;
  const clientId = window.crypto?.randomUUID?.() ||
    `lights-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
  let connected = false;
  const eventSource = new EventSource(
    `/api/modules/lights/events?type=${encodeURIComponent(document.body.dataset.lightsClient || 'unknown')}&clientId=${encodeURIComponent(clientId)}`
  );

  function dispatch(name, value) {
    if (name === 'configUpdate' || name === 'connectionStatus' || name === 'blackoutUpdate' ||
        name === 'connect' || name === 'disconnect') {
      latestState.set(name, value);
    }
    for (const listener of listeners.get(name) || []) {
      try { listener(value); } catch (error) { console.error(`Luces: error en evento ${name}`, error); }
    }
  }

  function listen(name) {
    eventSource.addEventListener(name, event => {
      try { dispatch(name, JSON.parse(event.data)); }
      catch (error) { console.error(`Luces: evento ${name} no válido`, error); }
    });
  }

  for (const name of ['configUpdate', 'connectionStatus', 'blackoutUpdate', 'beatTrigger']) listen(name);

  async function send(path, body) {
    const response = await fetch(path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });
    if (!response.ok) {
      const message = await response.text().catch(() => 'No se pudo comunicar con Cortex Go');
      throw new Error(message || 'No se pudo comunicar con Cortex Go');
    }
    return response.json();
  }

  function queueConfig(kind, config) {
    if (kind === 'preview' && (activeConfigWriteKind === 'settings' || pendingConfigRequest?.kind === 'settings')) {
      return Promise.resolve({ success: true, skipped: true });
    }
    if (pendingConfigRequest !== null) {
      pendingResolvers.splice(0).forEach(({ resolve }) => resolve({ success: true, skipped: true }));
    }
    pendingConfigRequest = { kind, config };
    const result = new Promise((resolve, reject) => pendingResolvers.push({ resolve, reject }));
    void flushConfigQueue();
    return result;
  }

  async function flushConfigQueue() {
    if (configWriteActive || pendingConfigRequest === null) return;
    configWriteActive = true;
    const request = pendingConfigRequest;
    pendingConfigRequest = null;
    const resolvers = pendingResolvers.splice(0);
    activeConfigWriteKind = request.kind;
    const isPreview = request.kind === 'preview';
    const path = isPreview ? '/api/modules/lights/preview' : '/api/modules/lights/settings';
    try {
      const response = await fetch(path, {
        method: isPreview ? 'POST' : 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(isPreview
          ? { clientId, changes: request.config }
          : { changes: request.config })
      });
      if (!response.ok) {
        const message = await response.text().catch(() => 'No se pudo sincronizar la configuración');
        throw new Error(message || 'No se pudo sincronizar la configuración');
      }
      const saved = await response.json();
      resolvers.forEach(({ resolve }) => resolve(saved));
    } catch (error) {
      resolvers.forEach(({ reject }) => reject(error));
    } finally {
      configWriteActive = false;
      activeConfigWriteKind = null;
      if (pendingConfigRequest !== null) void flushConfigQueue();
    }
  }

  const socket = {
    get connected() { return connected; },
    on(name, listener) {
      if (!listeners.has(name)) listeners.set(name, []);
      listeners.get(name).push(listener);
      if (latestState.has(name)) listener(latestState.get(name));
      return socket;
    },
    emit(name, payload = {}) {
      if (name === 'updateConfig') {
        return queueConfig('settings', payload).catch(error => {
          console.error('Luces: no se pudo guardar la configuración', error);
          dispatch('connect_error', error);
          throw error;
        });
      }
      if (name === 'previewConfig') {
        return queueConfig('preview', payload).catch(error => {
          console.error('Luces: no se pudo actualizar la vista previa', error);
          dispatch('connect_error', error);
          throw error;
        });
      }
      if (name === 'beatTrigger') {
        send('/api/modules/lights/beat', {
          clientId,
          origin: payload.origin,
          time: payload.time || Date.now()
        }).catch(error => {
          console.error('Luces: no se pudo enviar el pulso', error);
          dispatch('connect_error', error);
        });
        return true;
      }
      if (name === 'blackout') {
        return send('/api/modules/lights/blackout', { active: Boolean(payload.active) })
          .catch(error => {
            console.error('Luces: no se pudo cambiar el estado Blackout', error);
            dispatch('connect_error', error);
            throw error;
          });
      }
      return false;
    }
  };

  eventSource.onopen = () => {
    connected = true;
    dispatch('connect');
  };
  eventSource.onerror = () => {
    connected = false;
    dispatch('disconnect');
  };

  window.io = () => socket;
})();
