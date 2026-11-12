(() => {
  class CortexWelcomeSocket {
    constructor() {
      this.listeners = new Map();
      this.connected = false;
      this.initialized = false;
      this.settingsJSON = '';
      this.bannerJSON = '';
      this.timerSignature = '';
      this.transitionJSON = '';
      this.mutationQueue = Promise.resolve();
      this.polling = false;
      this.pollTimer = window.setInterval(() => this.poll(), 1000);
      this.poll();
    }

    on(event, callback) {
      if (!this.listeners.has(event)) this.listeners.set(event, new Set());
      this.listeners.get(event).add(callback);
      return this;
    }

    emit(event, payload) {
      const action = async () => {
        if (event === 'updateSettings') {
          const result = await this.request('/api/modules/welcome/settings', 'PUT', payload || {});
          if (result.settings) this.acceptSettings(result.settings);
        } else if (event === 'updateBanner') {
          const result = await this.request('/api/modules/welcome/settings', 'PUT', { banner: payload || {} });
          if (result.settings) this.acceptSettings(result.settings);
        } else if (event === 'controlTimer') {
          const result = await this.request('/api/modules/welcome/timer', 'POST', payload || {});
          if (result.timer) this.acceptTimer(result.timer, payload && payload.action);
        } else if (event === 'testObsConnection') {
          const result = await this.request('/api/modules/welcome/obs/test', 'POST', {});
          this.dispatch('obsStatus', result);
        } else if (event === 'triggerObsTransition') {
          try {
            const result = await this.request('/api/modules/welcome/obs/transition', 'POST', {});
            this.acceptTransition(result);
          } catch (error) {
            this.acceptTransition({ success: false, error: error.message });
          }
        }
      };
      this.mutationQueue = this.mutationQueue.then(action).catch(() => {});
      return this;
    }

    async request(url, method = 'GET', data) {
      const options = { method, credentials: 'same-origin', headers: {} };
      if (data !== undefined) {
        options.headers['Content-Type'] = 'application/json';
        options.body = JSON.stringify(data);
      }
      const response = await fetch(url, options);
      const contentType = response.headers.get('content-type') || '';
      const result = contentType.includes('application/json') ? await response.json() : null;
      if (!response.ok) {
        const message = result && result.error ? result.error : `La solicitud falló (${response.status})`;
        throw new Error(message);
      }
      return result || {};
    }

    dispatch(event, value) {
      const callbacks = this.listeners.get(event);
      if (!callbacks) return;
      callbacks.forEach((callback) => {
        try { callback(value); } catch (error) { console.error('Error de interfaz del módulo:', error); }
      });
    }

    acceptSettings(settings) {
      const nextJSON = JSON.stringify(settings);
      const bannerJSON = JSON.stringify(settings.banner || {});
      if (!this.initialized) {
        this.initialized = true;
        this.settingsJSON = nextJSON;
        this.bannerJSON = bannerJSON;
        this.dispatch('initSettings', settings);
        return;
      }
      if (nextJSON !== this.settingsJSON) {
        this.settingsJSON = nextJSON;
        this.dispatch('settingsUpdated', settings);
      }
      if (bannerJSON !== this.bannerJSON) {
        this.bannerJSON = bannerJSON;
        this.dispatch('bannerUpdated', settings.banner || {});
      }
    }

    acceptTimer(timer, preferredAction) {
      const signature = JSON.stringify([timer.running, timer.remainingSeconds, timer.totalDuration, timer.completed]);
      if (signature === this.timerSignature) return;
      const wasRunning = this.timerSignature ? JSON.parse(this.timerSignature)[0] : false;
      this.timerSignature = signature;
      let action = 'sync';
      if (preferredAction === 'reset') action = 'reset';
      else if (timer.running && !wasRunning) action = 'start';
      else if (!timer.running && wasRunning) action = 'pause';
      if ((action === 'pause' || action === 'reset') && timer.remainingSeconds !== undefined) {
        this.dispatch('timerCommand', { action: 'sync', value: timer.remainingSeconds, totalDuration: timer.totalDuration });
      }
      this.dispatch('timerCommand', {
        action,
        value: timer.remainingSeconds,
        totalDuration: timer.totalDuration,
      });
    }

    acceptTransition(result) {
      const signature = JSON.stringify(result);
      if (signature === this.transitionJSON) return;
      this.transitionJSON = signature;
      this.dispatch('obsTransitionResult', result);
    }

    async poll() {
      if (this.polling) return;
      this.polling = true;
      try {
        const state = await this.request('/api/modules/welcome/state');
        if (!this.connected) {
          this.connected = true;
          this.dispatch('connect');
        }
        this.acceptSettings(state.settings || {});
        this.acceptTimer(state.timer || {}, '');
        if (state.obs) {
          this.dispatch('obsStatus', {
            connected: state.obs.state === 'connected',
            connecting: state.obs.state === 'checking',
            error: state.obs.detail,
          });
        }
        if (state.lastTransition) this.acceptTransition(state.lastTransition);
      } catch (_) {
        if (this.connected) {
          this.connected = false;
          this.dispatch('disconnect');
        }
      } finally {
        this.polling = false;
      }
    }
  }

  window.io = () => new CortexWelcomeSocket();
})();
