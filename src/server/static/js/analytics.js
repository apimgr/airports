(function () {
    const loader = document.currentScript;
    if (!loader) return;

    const provider = loader.dataset.provider;
    const id = loader.dataset.id || '';
    const baseURL = (loader.dataset.url || '').replace(/\/+$/, '');

    function addScript(src, attributes) {
        const script = document.createElement('script');
        script.async = true;
        script.src = src;
        Object.entries(attributes || {}).forEach(function (entry) {
            script.setAttribute(entry[0], entry[1]);
        });
        document.head.appendChild(script);
    }

    switch (provider) {
    case 'google':
        if (id.startsWith('G-')) {
            window.dataLayer = window.dataLayer || [];
            window.gtag = window.gtag || function () { window.dataLayer.push(arguments); };
            window.gtag('js', new Date());
            window.gtag('config', id);
            addScript('https://www.googletagmanager.com/gtag/js?id=' + encodeURIComponent(id));
        } else {
            window.ga = window.ga || function () { (window.ga.q = window.ga.q || []).push(arguments); };
            window.ga.l = Date.now();
            window.ga('create', id, 'auto');
            window.ga('send', 'pageview');
            addScript('https://www.google-analytics.com/analytics.js');
        }
        break;
    case 'matomo':
    case 'piwik':
        window._paq = window._paq || [];
        window._paq.push(['trackPageView']);
        window._paq.push(['enableLinkTracking']);
        window._paq.push(['setTrackerUrl', baseURL + '/matomo.php']);
        window._paq.push(['setSiteId', id]);
        addScript(baseURL + '/matomo.js');
        break;
    case 'owa':
        window.owa_cmds = window.owa_cmds || [];
        window.owa_cmds.push(['setSiteId', id]);
        window.owa_cmds.push(['trackPageView']);
        addScript(baseURL + '/modules/base/js/owa.tracker-combined-min.js');
        break;
    case 'fathom':
        addScript(baseURL ? baseURL + '/tracker.js' : 'https://cdn.usefathom.com/script.js', { 'data-site': id });
        break;
    case 'plausible':
        addScript(baseURL ? baseURL + '/js/script.js' : 'https://plausible.io/js/script.js', { 'data-domain': id });
        break;
    case 'umami':
        addScript(baseURL + '/script.js', { 'data-website-id': id });
        break;
    case 'simple':
        addScript('https://scripts.simpleanalyticscdn.com/latest.js');
        break;
    case 'cloudflare':
        addScript('https://static.cloudflareinsights.com/beacon.min.js', {
            'data-cf-beacon': JSON.stringify({ token: id })
        });
        break;
    }
}());
