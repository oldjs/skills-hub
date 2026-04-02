let searchTimeout = null;
const searchInput = document.getElementById('search-input');
const suggestionsEl = document.getElementById('search-suggestions');

if (searchInput && suggestionsEl) {
    searchInput.addEventListener('input', (e) => {
        const query = e.target.value.trim();
        
        if (searchTimeout) {
            clearTimeout(searchTimeout);
        }
        
        if (query.length < 2) {
            suggestionsEl.classList.add('hidden');
            return;
        }
        
        searchTimeout = setTimeout(() => {
            fetchSuggestions(query);
        }, 300);
    });
    
    searchInput.addEventListener('focus', () => {
        if (searchInput.value.length >= 2) {
            suggestionsEl.classList.remove('hidden');
        }
    });
    
    document.addEventListener('click', (e) => {
        if (!searchInput.contains(e.target) && !suggestionsEl.contains(e.target)) {
            suggestionsEl.classList.add('hidden');
        }
    });
}

async function fetchSuggestions(query) {
    if (!suggestionsEl) {
        return;
    }

    try {
        const response = await fetch(`/api/search?q=${encodeURIComponent(query)}&format=json`);
        const data = await response.json();
        
        if (data.skills && data.skills.length > 0) {
            renderSuggestions(data.skills.slice(0, 8));
        } else {
            suggestionsEl.innerHTML = '<div class="p-3 text-gray-500 text-sm">未找到匹配结果</div>';
            suggestionsEl.classList.remove('hidden');
        }
    } catch (err) {
        console.error('Search error:', err);
    }
}

function renderSuggestions(skills) {
    if (!skills || skills.length === 0) {
        suggestionsEl.classList.add('hidden');
        return;
    }
    
    suggestionsEl.innerHTML = skills.map(skill => `
        <a href="/skill?slug=${skill.slug}" class="flex items-center p-3 hover:bg-gray-50 transition border-b border-gray-50 last:border-0">
            <div class="w-8 h-8 bg-indigo-100 rounded-lg flex items-center justify-center mr-3 flex-shrink-0">
                <i class="fas fa-puzzle-piece text-indigo-500 text-xs"></i>
            </div>
            <div class="flex-1 min-w-0">
                <div class="font-medium text-gray-900 truncate">${skill.displayName}</div>
                <div class="text-xs text-gray-500 truncate">${skill.summary || ''}</div>
            </div>
        </a>
    `).join('');
    
    suggestionsEl.classList.remove('hidden');
}

async function copyInstall(slug) {
    const command = `openclaw skills install ${slug}`;
    
    try {
        await navigator.clipboard.writeText(command);
        showToast('安装命令已复制到剪贴板!');
    } catch (err) {
        const textarea = document.createElement('textarea');
        textarea.value = command;
        textarea.style.position = 'fixed';
        textarea.style.opacity = '0';
        document.body.appendChild(textarea);
        textarea.select();
        document.execCommand('copy');
        document.body.removeChild(textarea);
        showToast('安装命令已复制到剪贴板!');
    }
}

function showToast(message) {
    const existing = document.querySelector('.toast-notification');
    if (existing) existing.remove();
    
    const toast = document.createElement('div');
    toast.className = 'toast-notification fixed bottom-4 right-4 px-6 py-3 bg-gray-900 text-white rounded-lg shadow-lg z-50 fade-in';
    toast.innerHTML = `<i class="fas fa-check-circle mr-2 text-green-400"></i>${message}`;
    document.body.appendChild(toast);
    
    setTimeout(() => {
        toast.style.opacity = '0';
        toast.style.transition = 'opacity 0.3s ease';
        setTimeout(() => toast.remove(), 300);
    }, 2000);
}

async function triggerSync() {
    const source = arguments[0];
    const btn = source?.currentTarget || source?.target || source;
    if (!btn || !btn.innerHTML) {
        return;
    }

    const originalText = btn.innerHTML;
    btn.disabled = true;
    btn.innerHTML = '<i class="fas fa-spinner fa-spin mr-2"></i>同步中...';
    
    try {
        const response = await fetch('/sync', { method: 'POST' });
        const data = await response.json();
        
        if (data.status === 'started') {
            showToast('数据同步已启动，请稍后刷新页面');
            setTimeout(() => location.reload(), 3000);
        } else if (data.status === 'running') {
            showToast('同步正在进行中，请稍后查看');
        } else {
            showToast('同步失败: ' + (data.message || '未知错误'));
        }
    } catch (err) {
        showToast('同步请求失败');
    } finally {
        btn.disabled = false;
        btn.innerHTML = originalText;
    }
}

if (typeof window !== 'undefined') {
    window.copyInstall = copyInstall;
    window.triggerSync = triggerSync;
}
