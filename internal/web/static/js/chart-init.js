function initResponseChart(canvasId, data) {
    if (!data || data.length === 0) return;

    const ctx = document.getElementById(canvasId).getContext('2d');
    new Chart(ctx, {
        type: 'line',
        data: {
            labels: data.map(d => new Date(d.timestamp).toLocaleString()),
            datasets: [{
                label: 'Response Time (ms)',
                data: data.map(d => d.avg_response_ms),
                borderColor: '#2563eb',
                backgroundColor: 'rgba(37, 99, 235, 0.1)',
                fill: true,
                tension: 0.3,
                pointRadius: 0,
            }]
        },
        options: {
            responsive: true,
            scales: {
                y: { beginAtZero: true, title: { display: true, text: 'ms' } },
                x: { display: false }
            },
            plugins: { legend: { display: false } }
        }
    });
}
