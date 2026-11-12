(() => {
  const socket = io();
  let currentJSON = '';

  function applyBanner(settings) {
    if (!settings || !settings.banner) return;
    const nextJSON = JSON.stringify(settings.banner);
    if (nextJSON === currentJSON) return;
    currentJSON = nextJSON;
    window.BannerStudio.setState(settings.banner);
  }

  socket.on('initSettings', applyBanner);
  socket.on('settingsUpdated', applyBanner);
  socket.on('bannerUpdated', (banner) => applyBanner({ banner }));
})();
