function initResponseChart(canvasId, data) {
    if (!data || data.length === 0) {
        var container = document.getElementById(canvasId).parentElement;
        container.innerHTML = '<p style="color: #64748b; text-align: center; padding: 4rem 0; font-size: 0.9rem;">No data yet</p>';
        return;
    }

    var ctx = document.getElementById(canvasId).getContext('2d');

    var gradient = ctx.createLinearGradient(0, 0, 0, 240);
    gradient.addColorStop(0, 'rgba(59, 130, 246, 0.2)');
    gradient.addColorStop(1, 'rgba(59, 130, 246, 0.0)');

    new Chart(ctx, {
        type: 'line',
        data: {
            labels: data.map(function(d) {
                return new Date(d.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
            }),
            datasets: [{
                label: 'Response Time (ms)',
                data: data.map(function(d) { return d.avg_response_ms; }),
                borderColor: '#3b82f6',
                backgroundColor: gradient,
                fill: true,
                tension: 0.4,
                pointRadius: 0,
                pointHitRadius: 10,
                borderWidth: 2,
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            interaction: {
                intersect: false,
                mode: 'index',
            },
            scales: {
                y: {
                    beginAtZero: true,
                    grid: { color: 'rgba(30, 41, 59, 0.8)', drawBorder: false },
                    ticks: {
                        color: '#64748b',
                        font: { family: "'JetBrains Mono', monospace", size: 11 },
                        callback: function(v) { return v + ' ms'; },
                    },
                    border: { display: false },
                },
                x: {
                    grid: { display: false },
                    ticks: {
                        color: '#64748b',
                        font: { family: "'JetBrains Mono', monospace", size: 10 },
                        maxTicksLimit: 8,
                    },
                    border: { display: false },
                }
            },
            plugins: {
                legend: { display: false },
                tooltip: {
                    backgroundColor: '#1e2740',
                    titleColor: '#f1f5f9',
                    bodyColor: '#94a3b8',
                    borderColor: '#334155',
                    borderWidth: 1,
                    padding: 10,
                    titleFont: { family: "'JetBrains Mono', monospace", size: 12 },
                    bodyFont: { family: "'JetBrains Mono', monospace", size: 11 },
                    displayColors: false,
                    callbacks: {
                        label: function(ctx) { return Math.round(ctx.parsed.y) + ' ms'; }
                    }
                }
            }
        }
    });
}
