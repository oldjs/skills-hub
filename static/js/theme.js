// 暗色模式切换，用 localStorage 持久化
(function() {
    var KEY = 'skills-hub-theme';

    // 绑定所有切换按钮
    document.addEventListener('DOMContentLoaded', function() {
        document.querySelectorAll('.theme-toggle-btn').forEach(function(btn) {
            btn.addEventListener('click', function() {
                var isDark = document.documentElement.classList.contains('dark');
                var next = isDark ? 'light' : 'dark';
                localStorage.setItem(KEY, next);
                document.documentElement.classList.toggle('dark', next === 'dark');
            });
        });
    });

    // 系统偏好变化时跟随（仅未手动选择时）
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', function(e) {
        if (!localStorage.getItem(KEY)) {
            document.documentElement.classList.toggle('dark', e.matches);
        }
    });
})();
