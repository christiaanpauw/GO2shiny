/* NZ Trade Intelligence Dashboard — Tabulator data-table helpers */

/**
 * initTradeTable initialises a Tabulator remote-pagination table and wires up
 * a CSV download button and a debounced search input.
 *
 * @param {string} tableId        - The id of the container element.
 * @param {string} apiUrl         - Base URL of the paginated table JSON endpoint.
 * @param {string} downloadBtnId  - The id of the CSV download button.
 * @param {string} searchInputId  - The id of the search text input.
 * @returns {Tabulator|null}
 */
function initTradeTable(tableId, apiUrl, downloadBtnId, searchInputId) {
    const el = document.getElementById(tableId);
    if (!el) return null;

    let currentSearch = '';

    const table = new Tabulator('#' + tableId, {
        ajaxURL: apiUrl,
        ajaxParams: function() {
            const params = { page: table.getPage(), size: table.getPageSize() };
            if (currentSearch) params.q = currentSearch;
            return params;
        },
        ajaxResponse: function(_url, _params, response) {
            return {
                last_page: Math.ceil(response.total / response.size) || 1,
                data: response.rows || [],
            };
        },
        pagination: true,
        paginationMode: 'remote',
        paginationSize: 25,
        paginationSizeSelector: [10, 25, 50, 100],
        sortMode: 'local',
        layout: 'fitColumns',
        responsiveLayout: 'collapse',
        columns: [
            { title: 'Year',          field: 'year',      sorter: 'number', width: 80  },
            { title: 'Country',       field: 'country',   sorter: 'string'             },
            { title: 'Direction',     field: 'type_ie',   sorter: 'string', width: 110 },
            { title: 'Type',          field: 'type_gs',   sorter: 'string', width: 100 },
            { title: 'Commodity',     field: 'commodity', sorter: 'string'             },
            {
                title: 'Value (NZD M)',
                field: 'value_nzd',
                sorter: 'number',
                hozAlign: 'right',
                formatter: 'money',
                formatterParams: { precision: 2 },
            },
        ],
    });

    // CSV download button.
    const downloadBtn = document.getElementById(downloadBtnId);
    if (downloadBtn) {
        downloadBtn.addEventListener('click', function() {
            table.download('csv', 'nz-trade-data.csv');
        });
    }

    // Debounced search input.
    const searchInput = document.getElementById(searchInputId);
    if (searchInput) {
        let debounceTimer;
        searchInput.addEventListener('input', function() {
            clearTimeout(debounceTimer);
            debounceTimer = setTimeout(function() {
                currentSearch = searchInput.value.trim();
                table.setData(apiUrl);
            }, 300);
        });
    }

    return table;
}
