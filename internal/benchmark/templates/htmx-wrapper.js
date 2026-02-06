// HTMX wrapper - must load BEFORE SSE extension
// This intercepts htmx.defineExtension to track registered extensions
(function() {
    if (typeof htmx === 'undefined') {
        console.error('HTMX not loaded yet');
        return;
    }
    
    window.registeredExtensions = {};
    
    const originalDefineExtension = htmx.defineExtension.bind(htmx);
    htmx.defineExtension = function(name, extension) {
        const result = originalDefineExtension(name, extension);
        window.registeredExtensions[name] = true;
        console.log('[EXT] Registered:', name);
        return result;
    };
    
    console.log('HTMX wrapper loaded, waiting for SSE extension...');

    // Memory estimation logic
    document.body.addEventListener('change', function(e) {
        if (e.target.tagName === 'SELECT' && (e.target.name === 'context_size' || e.target.name === 'ngl')) {
            const form = e.target.closest('form');
            if (!form || form.getAttribute('hx-post') !== '/api/servers/start') return;

            const estimateDiv = form.querySelector('.memory-estimate');
            if (!estimateDiv) return;

            const modelSize = parseInt(form.dataset.modelSize) || 0;
            const contextSize = parseInt(form.querySelector('[name="context_size"]').value) || 131072;
            const ngl = parseInt(form.querySelector('[name="ngl"]').value);
            
            // Simplified estimation logic:
            // - Weights take up modelSize
            // - KV cache takes up proportional to contextSize
            // - NGL affects how much is in VRAM
            
            // Very rough KV cache estimate: 0.5MB per 1K context for small models, up to 2MB for large ones
            const kvCachePer1K = modelSize > 10 * 1024 * 1024 * 1024 ? 2 : 0.5;
            const kvCacheBytes = (contextSize / 1024) * kvCachePer1K * 1024 * 1024;
            
            const totalEstimated = modelSize + kvCacheBytes;
            const vramRatio = (ngl === 999) ? 1.0 : (ngl / 80); // Assuming 80 layers max
            const estimatedVRAM = totalEstimated * Math.min(1, vramRatio);
            
            function formatBytes(bytes) {
                if (bytes === 0) return '0 B';
                const k = 1024;
                const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
                const i = Math.floor(Math.log(bytes) / Math.log(k));
                return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
            }

            estimateDiv.innerHTML = `<span style="color: #94a3b8; font-size: 11px;">Est. Memory: <b>${formatBytes(totalEstimated)}</b> (~${formatBytes(estimatedVRAM)} VRAM)</span>`;
        }
    });
})();
