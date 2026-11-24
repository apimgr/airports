document.addEventListener('DOMContentLoaded', function() {
    const swaggerEl = document.getElementById('swagger-ui');
    if (!swaggerEl || typeof SwaggerUIBundle === 'undefined') return;
    window.ui = SwaggerUIBundle({
        url: swaggerEl.getAttribute('data-spec-url'),
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
            SwaggerUIBundle.presets.apis,
            SwaggerUIStandalonePreset
        ],
        plugins: [
            SwaggerUIBundle.plugins.DownloadUrl
        ],
        layout: 'StandaloneLayout'
    });
});
