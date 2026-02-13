/**
 * ForgeCommerce Admin — Client-side enhancements
 * Keyboard shortcuts, confirmations, HTMX helpers, search
 */

(function () {
  'use strict';

  // ─── Keyboard Shortcuts ─────────────────────────────────────────
  var shortcuts = {
    'g d': '/admin/dashboard',
    'g p': '/admin/products',
    'g c': '/admin/categories',
    'g r': '/admin/inventory/raw-materials',
    'g o': '/admin/orders',
    'g s': '/admin/settings/vat',
    'g u': '/admin/users',
    'g m': '/admin/reports/sales',
  };

  var keyBuffer = '';
  var keyTimeout = null;

  document.addEventListener('keydown', function (e) {
    // Don't trigger shortcuts when typing in inputs
    var tag = e.target.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || e.target.isContentEditable) return;

    // Escape to close modals/dialogs
    if (e.key === 'Escape') {
      var modal = document.querySelector('.modal-overlay.active');
      if (modal) {
        modal.classList.remove('active');
        return;
      }
    }

    // "?" to show shortcut help
    if (e.key === '?' && !e.ctrlKey && !e.metaKey) {
      e.preventDefault();
      toggleShortcutHelp();
      return;
    }

    // "/" to focus search
    if (e.key === '/' && !e.ctrlKey && !e.metaKey) {
      var search = document.querySelector('#admin-search');
      if (search) {
        e.preventDefault();
        search.focus();
        return;
      }
    }

    // "n" to create new (context-sensitive)
    if (e.key === 'n' && !e.ctrlKey && !e.metaKey) {
      var newBtn = document.querySelector('[data-shortcut="new"]');
      if (newBtn) {
        e.preventDefault();
        newBtn.click();
        return;
      }
    }

    // Multi-key shortcuts (e.g., "g d" for dashboard)
    clearTimeout(keyTimeout);
    keyBuffer += e.key;

    if (keyBuffer.length > 1) {
      var target = shortcuts[keyBuffer];
      if (target) {
        e.preventDefault();
        window.location.href = target;
      }
      keyBuffer = '';
    } else {
      keyTimeout = setTimeout(function () { keyBuffer = ''; }, 500);
    }
  });

  // ─── Shortcut Help Panel (built with safe DOM methods) ──────────
  function createEl(tag, attrs, children) {
    var el = document.createElement(tag);
    if (attrs) {
      Object.keys(attrs).forEach(function (k) {
        if (k === 'className') el.className = attrs[k];
        else if (k === 'textContent') el.textContent = attrs[k];
        else if (k.startsWith('on')) el.addEventListener(k.slice(2).toLowerCase(), attrs[k]);
        else el.setAttribute(k, attrs[k]);
      });
    }
    if (children) {
      children.forEach(function (c) {
        if (typeof c === 'string') el.appendChild(document.createTextNode(c));
        else if (c) el.appendChild(c);
      });
    }
    return el;
  }

  function shortcutRow(keys, desc) {
    return createEl('tr', null, [
      createEl('td', null, [createEl('kbd', { textContent: keys })]),
      createEl('td', { textContent: desc })
    ]);
  }

  function sectionRow(label) {
    var td = createEl('td', {
      colspan: '2',
      style: 'padding-top:12px;font-weight:600;color:#6b7280',
      textContent: label
    });
    return createEl('tr', null, [td]);
  }

  function toggleShortcutHelp() {
    var panel = document.getElementById('shortcut-help');
    if (panel) {
      panel.classList.toggle('active');
      return;
    }

    var tbody = createEl('tbody', null, [
      shortcutRow('?', 'Show this help'),
      shortcutRow('/', 'Focus search'),
      shortcutRow('n', 'Create new (context)'),
      shortcutRow('Esc', 'Close modal / cancel'),
      sectionRow('Navigation (g + key)'),
      shortcutRow('g d', 'Dashboard'),
      shortcutRow('g p', 'Products'),
      shortcutRow('g c', 'Categories'),
      shortcutRow('g r', 'Raw Materials'),
      shortcutRow('g o', 'Orders'),
      shortcutRow('g s', 'Settings'),
      shortcutRow('g u', 'Admin Users'),
      shortcutRow('g m', 'Reports'),
    ]);

    var table = createEl('table', { style: 'width:100%' }, [tbody]);

    var closeBtn = createEl('button', {
      className: 'btn-icon',
      textContent: '\u00D7',
      style: 'font-size:24px'
    });

    var header = createEl('div', { className: 'modal-header' }, [
      createEl('h3', { textContent: 'Keyboard Shortcuts' }),
      closeBtn
    ]);

    var body = createEl('div', { className: 'modal-body', style: 'padding:16px' }, [table]);
    var content = createEl('div', { className: 'modal-content', style: 'max-width:420px' }, [header, body]);

    panel = createEl('div', { id: 'shortcut-help', className: 'modal-overlay active' }, [content]);

    closeBtn.addEventListener('click', function () { panel.classList.remove('active'); });
    panel.addEventListener('click', function (e) {
      if (e.target === panel) panel.classList.remove('active');
    });

    document.body.appendChild(panel);
  }

  // ─── Delete Confirmations ───────────────────────────────────────
  document.addEventListener('click', function (e) {
    var btn = e.target.closest('[data-confirm]');
    if (!btn) return;

    var message = btn.getAttribute('data-confirm') || 'Are you sure? This action cannot be undone.';
    if (!confirm(message)) {
      e.preventDefault();
      e.stopImmediatePropagation();
    }
  });

  // ─── HTMX: Auto-dismiss Flash Messages ─────────────────────────
  document.addEventListener('htmx:afterSwap', function (e) {
    var flashes = e.detail.target.querySelectorAll('.alert[data-auto-dismiss]');
    flashes.forEach(function (flash) {
      setTimeout(function () {
        flash.style.transition = 'opacity 0.3s';
        flash.style.opacity = '0';
        setTimeout(function () { flash.remove(); }, 300);
      }, 4000);
    });
  });

  // ─── HTMX: Loading Indicators ──────────────────────────────────
  document.addEventListener('htmx:beforeRequest', function (e) {
    var indicator = e.detail.elt.querySelector('.htmx-indicator');
    if (indicator) indicator.style.display = 'inline-block';
  });

  document.addEventListener('htmx:afterRequest', function (e) {
    var indicator = e.detail.elt.querySelector('.htmx-indicator');
    if (indicator) indicator.style.display = 'none';
  });

  // ─── Form: Unsaved Changes Warning ─────────────────────────────
  var formDirty = false;

  document.addEventListener('input', function (e) {
    if (e.target.closest('form[data-track-changes]')) {
      formDirty = true;
    }
  });

  document.addEventListener('submit', function () {
    formDirty = false;
  });

  window.addEventListener('beforeunload', function (e) {
    if (formDirty) {
      e.preventDefault();
      e.returnValue = '';
    }
  });

  // ─── Batch Select All ──────────────────────────────────────────
  document.addEventListener('change', function (e) {
    if (e.target.id === 'select-all') {
      var checkboxes = document.querySelectorAll('.row-select');
      checkboxes.forEach(function (cb) { cb.checked = e.target.checked; });
      updateBatchActions();
    }
    if (e.target.classList.contains('row-select')) {
      updateBatchActions();
    }
  });

  function updateBatchActions() {
    var selected = document.querySelectorAll('.row-select:checked').length;
    var batchBar = document.getElementById('batch-actions');
    if (batchBar) {
      batchBar.style.display = selected > 0 ? 'flex' : 'none';
      var count = batchBar.querySelector('.batch-count');
      if (count) count.textContent = selected + ' selected';
    }
  }

  // ─── Toast Notifications ───────────────────────────────────────
  window.ForgeToast = function (message, type) {
    type = type || 'success';
    var toast = document.createElement('div');
    toast.className = 'toast toast-' + type;
    toast.textContent = message;
    document.body.appendChild(toast);

    requestAnimationFrame(function () { toast.classList.add('toast-visible'); });
    setTimeout(function () {
      toast.classList.remove('toast-visible');
      setTimeout(function () { toast.remove(); }, 300);
    }, 3000);
  };

  // Listen for HTMX success responses to show toasts
  document.addEventListener('htmx:afterRequest', function (e) {
    var trigger = e.detail.xhr && e.detail.xhr.getResponseHeader('HX-Trigger');
    if (trigger) {
      try {
        var data = JSON.parse(trigger);
        if (data.showToast) {
          ForgeToast(data.showToast.message, data.showToast.type);
        }
      } catch (_) { /* not JSON trigger, ignore */ }
    }
  });

})();
