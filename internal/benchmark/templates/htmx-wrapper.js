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
})();
