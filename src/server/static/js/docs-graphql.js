document.addEventListener('DOMContentLoaded', function() {
    const graphiqlEl = document.getElementById('graphiql');
    if (!graphiqlEl || typeof GraphiQL === 'undefined') return;
    const fetcher = GraphiQL.createFetcher({ url: graphiqlEl.getAttribute('data-endpoint-url') });
    const defaultQuery = '# ' + translate('graphql.editor_welcome') + '\n# ' + translate('graphql.editor_run_hint') + '\n\n' +
        'query GetAirport {\n  airport(code: "KJFK") {\n    icao\n    iata\n    name\n    city\n    country\n    coordinates { lat lon }\n  }\n}\n\n' +
        'query SearchNearby {\n  nearby(lat: 40.6398, lon: -73.7789, radius: 50) {\n    icao\n    name\n    distance\n  }\n}\n';
    ReactDOM.createRoot(graphiqlEl)
        .render(React.createElement(GraphiQL, { fetcher: fetcher, defaultQuery: defaultQuery }));
});
