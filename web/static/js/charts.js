/* NZ Trade Intelligence Dashboard — ECharts initialisation helpers */

/**
 * initBarChart initialises an ECharts bar chart inside the given DOM element.
 * @param {string} elementId - The id of the container element.
 * @param {object} options   - ECharts option object.
 * @returns {echarts.ECharts} The chart instance.
 */
function initBarChart(elementId, options) {
    const el = document.getElementById(elementId);
    if (!el) return null;
    const chart = echarts.init(el);
    chart.setOption(options || {});
    window.addEventListener('resize', () => chart.resize());
    return chart;
}

/**
 * initLineChart initialises an ECharts line chart.
 * @param {string} elementId
 * @param {object} options
 * @returns {echarts.ECharts}
 */
function initLineChart(elementId, options) {
    return initBarChart(elementId, options);
}

/**
 * initPieChart initialises an ECharts pie/donut chart.
 * @param {string} elementId
 * @param {object} options
 * @returns {echarts.ECharts}
 */
function initPieChart(elementId, options) {
    return initBarChart(elementId, options);
}

/**
 * initTimeSeries fetches JSON from baseUrl (with filter params) and renders a
 * time-series line chart (exports vs imports) inside the given container.
 *
 * @param {string} divId    - The id of the container element.
 * @param {string} baseUrl  - Base URL of the time-series JSON endpoint.
 * @param {string|number} yearFrom
 * @param {string|number} yearTo
 * @param {string} typeIE   - Direction filter ("Exports", "Imports", "Both", or "").
 * @param {string} typeGS   - Type filter ("Goods", "Services", "Total", or "").
 * @returns {Promise<echarts.ECharts|null>}
 */
async function initTimeSeries(divId, baseUrl, yearFrom, yearTo, typeIE, typeGS) {
    const el = document.getElementById(divId);
    if (!el) return null;

    // Build URL with filter params.
    const params = new URLSearchParams();
    if (yearFrom != null) params.set('year_from', yearFrom);
    if (yearTo   != null) params.set('year_to',   yearTo);
    if (typeIE)           params.set('type_ie',   typeIE);
    if (typeGS)           params.set('type_gs',   typeGS);
    const apiUrl = baseUrl + (params.toString() ? '?' + params.toString() : '');

    // Dispose previous chart instance if present.
    const existing = echarts.getInstanceByDom(el);
    if (existing) existing.dispose();

    const chart = echarts.init(el);
    chart.showLoading();

    try {
        const resp = await fetch(apiUrl);
        if (!resp.ok) throw new Error('HTTP ' + resp.status);
        const data = await resp.json();

        const years        = data.map(d => d.year);
        const exportValues = data.map(d => d.exports);
        const importValues = data.map(d => d.imports);

        chart.setOption({
            tooltip: { trigger: 'axis' },
            legend: { data: ['Exports', 'Imports'] },
            xAxis: { type: 'category', data: years, name: 'Year' },
            yAxis: { type: 'value', name: 'NZD' },
            series: [
                { name: 'Exports', type: 'line', data: exportValues, smooth: true },
                { name: 'Imports', type: 'line', data: importValues, smooth: true }
            ]
        });
    } catch (err) {
        console.error('initTimeSeries error:', err);
        el.innerHTML = '<p class="text-danger text-center py-4">Failed to load chart data.</p>';
        return null;
    } finally {
        chart.hideLoading();
    }

    window.addEventListener('resize', () => chart.resize());
    return chart;
}

/**
 * initTreemap fetches JSON from baseUrl (with filter params) and renders an
 * ECharts treemap inside the given container element.
 *
 * @param {string} divId   - The id of the container element.
 * @param {string} baseUrl - Base URL of the treemap JSON endpoint.
 * @param {string|number} year
 * @param {string} typeIE  - Direction filter; Imports → Imports, otherwise Exports.
 * @param {string} typeGS  - Type filter ("Goods", "Services", "Total", or "").
 * @returns {Promise<echarts.ECharts|null>}
 */
async function initTreemap(divId, baseUrl, year, typeIE, typeGS) {
    const el = document.getElementById(divId);
    if (!el) return null;

    const params = new URLSearchParams();
    if (year != null) params.set('year', year);
    // Map typeIE to the treemap direction (single direction required).
    const direction = (typeIE === 'Imports') ? 'Imports' : 'Exports';
    params.set('direction', direction);
    if (typeGS) params.set('type_gs', typeGS);
    const apiUrl = baseUrl + (params.toString() ? '?' + params.toString() : '');

    // Dispose previous chart instance if present.
    const existing = echarts.getInstanceByDom(el);
    if (existing) existing.dispose();

    const chart = echarts.init(el);
    chart.showLoading();

    try {
        const resp = await fetch(apiUrl);
        if (!resp.ok) throw new Error('HTTP ' + resp.status);
        const root = await resp.json();

        chart.setOption({
            tooltip: { formatter: '{b}: {c}' },
            series: [{
                type: 'treemap',
                name: root.name,
                data: (root.children || []).map(c => ({ name: c.name, value: c.value })),
                label: { show: true, formatter: '{b}' }
            }]
        });
    } catch (err) {
        console.error('initTreemap error:', err);
        el.innerHTML = '<p class="text-danger text-center py-4">Failed to load chart data.</p>';
        return null;
    } finally {
        chart.hideLoading();
    }

    window.addEventListener('resize', () => chart.resize());
    return chart;
}
