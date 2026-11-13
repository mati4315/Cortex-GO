(() => {
  const listeners = new Map();
  const latestState = new Map();
  const clientId = window.crypto?.randomUUID?.() ||
    `lights-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
  let connected = false;
  const eventSource = new EventSource(
    `/api/modules/lights/events?type=${encodeURIComponent(document.body.dataset.lightsClient || 'unknown')}&clientId=${encodeURIComponent(clientId)}`
  );

  function dispatch(name, value) {
    if (name === 'configUpdate' || name === 'connectionStatus') latestState.set(name, value);
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

  for (const name of ['configUpdate', 'connectionStatus', 'beatTrigger']) listen(name);

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
        fetch('/api/modules/lights/settings', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ changes: payload })
        }).then(async response => {
          if (response.ok) return;
          const message = await response.text().catch(() => 'No se pudo guardar la configuración');
          throw new Error(message || 'No se pudo guardar la configuración');
        }).catch(error => {
          console.error('Luces: no se pudo guardar la configuración', error);
          dispatch('connect_error', error);
        });
        return true;
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
