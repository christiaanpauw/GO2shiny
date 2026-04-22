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
