let searchTimeout = null;
let searchAbortController = null;
const searchInput = document.getElementById('search-input');
const suggestionsEl = document.getElementById('search-suggestions');

if (searchInput && suggestionsEl) {
    searchInput.addEventListener('input', (e) => {
        const query = e.target.value.trim();
        
        if (searchTimeout) {
            clearTimeout(searchTimeout);
        }
        
        if (query.length < 2) {
            if (searchAbortController) {
                searchAbortController.abort();
            }
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

    if (searchAbortController) {
        searchAbortController.abort();
    }
    searchAbortController = new AbortController();

    setSuggestionsMessage('搜索中...');

    try {
        const response = await fetch(`/api/search?q=${encodeURIComponent(query)}&format=json`, {
            signal: searchAbortController.signal,
        });
        const data = await response.json();
        
        if (data.skills && data.skills.length > 0) {
            renderSuggestions(data.skills.slice(0, 8));
        } else {
            setSuggestionsMessage('未找到匹配结果');
        }
    } catch (err) {
        if (err.name === 'AbortError') {
            return;
        }
        console.error('Search error:', err);
        setSuggestionsMessage('搜索建议加载失败');
    }
}

function renderSuggestions(skills) {
    if (!skills || skills.length === 0) {
        suggestionsEl.classList.add('hidden');
        return;
    }
    
    suggestionsEl.innerHTML = '';
    skills.forEach(skill => {
        const item = document.createElement('a');
        item.href = `/skill?slug=${encodeURIComponent(skill.slug)}`;
        item.className = 'flex items-center p-3 hover:bg-gray-50 transition border-b border-gray-50 last:border-0';

        const iconWrap = document.createElement('div');
        iconWrap.className = 'w-8 h-8 bg-indigo-100 rounded-lg flex items-center justify-center mr-3 flex-shrink-0';
        const icon = document.createElement('i');
        icon.className = 'fas fa-puzzle-piece text-indigo-500 text-xs';
        iconWrap.appendChild(icon);

        const content = document.createElement('div');
        content.className = 'flex-1 min-w-0';

        const title = document.createElement('div');
        title.className = 'font-medium text-gray-900 truncate';
        title.textContent = skill.displayName || skill.slug;

        const summary = document.createElement('div');
        summary.className = 'text-xs text-gray-500 truncate';
        summary.textContent = skill.summary || '暂无描述';

        content.appendChild(title);
        content.appendChild(summary);
        item.appendChild(iconWrap);
        item.appendChild(content);
        suggestionsEl.appendChild(item);
    });
    
    suggestionsEl.classList.remove('hidden');
}

function setSuggestionsMessage(message) {
    if (!suggestionsEl) {
        return;
    }
    suggestionsEl.innerHTML = '';
    const empty = document.createElement('div');
    empty.className = 'p-3 text-sm text-gray-500';
    empty.textContent = message;
    suggestionsEl.appendChild(empty);
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

    const icon = document.createElement('i');
    icon.className = 'fas fa-check-circle mr-2 text-green-400';
    const text = document.createElement('span');
    text.textContent = message;

    toast.appendChild(icon);
    toast.appendChild(text);
    document.body.appendChild(toast);
    
    setTimeout(() => {
        toast.style.opacity = '0';
        toast.style.transition = 'opacity 0.3s ease';
        setTimeout(() => toast.remove(), 300);
    }, 2000);
}

if (typeof window !== 'undefined') {
    window.copyInstall = copyInstall;
}
