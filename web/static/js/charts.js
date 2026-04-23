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
 * initTimeSeries fetches JSON from apiUrl and renders a time-series line chart
 * (exports vs imports) inside the given container element.
 *
 * The API must return an array of objects with the shape:
 *   { year: number, exports: number, imports: number }
 *
 * @param {string} divId  - The id of the container element.
 * @param {string} apiUrl - URL of the time-series JSON endpoint.
 * @returns {Promise<echarts.ECharts|null>}
 */
async function initTimeSeries(divId, apiUrl) {
    const el = document.getElementById(divId);
    if (!el) return null;

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
 * initTreemap fetches JSON from apiUrl and renders an ECharts treemap inside
 * the given container element.
 *
 * The API must return an object with the shape:
 *   { name: string, children: [{ name: string, value: number }, ...] }
 *
 * @param {string} divId  - The id of the container element.
 * @param {string} apiUrl - URL of the treemap JSON endpoint.
 * @returns {Promise<echarts.ECharts|null>}
 */
async function initTreemap(divId, apiUrl) {
    const el = document.getElementById(divId);
    if (!el) return null;

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
