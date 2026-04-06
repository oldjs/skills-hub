function refreshCaptchaImage() {
    const captchaImg = document.getElementById('captchaImg');
    const captchaInput = document.getElementById('captcha');

    if (captchaImg) {
        captchaImg.src = `/captcha?ts=${Date.now()}`;
    }
    if (captchaInput) {
        captchaInput.value = '';
        captchaInput.focus();
    }
}

function setAuthMessage(message, isError) {
    const box = document.getElementById('auth-message');
    if (!box) {
        return;
    }

    box.textContent = message;
    box.classList.remove('hidden', 'border-red-200', 'bg-red-50', 'text-red-600', 'border-emerald-200', 'bg-emerald-50', 'text-emerald-600');
    if (isError) {
        box.classList.add('border-red-200', 'bg-red-50', 'text-red-600');
    } else {
        box.classList.add('border-emerald-200', 'bg-emerald-50', 'text-emerald-600');
    }
}

function bindAuthPage() {
    const sendCodeBtn = document.getElementById('sendCodeBtn');
    const authForm = document.getElementById('auth-form');
    const captchaImg = document.getElementById('captchaImg');
    const captchaInput = document.getElementById('captcha');

    if (captchaImg) {
        captchaImg.addEventListener('click', refreshCaptchaImage);
    }

    if (captchaInput) {
        captchaInput.addEventListener('input', () => {
            captchaInput.value = captchaInput.value.toUpperCase();
        });
    }

    if (!sendCodeBtn || !authForm) {
        return;
    }

    sendCodeBtn.addEventListener('click', async () => {
        const emailInput = document.getElementById('email');
        const csrfInput = authForm.querySelector('input[name="csrf_token"]');
        const purpose = authForm.dataset.purpose || 'login';
        const email = emailInput ? emailInput.value.trim() : '';
        const captcha = captchaInput ? captchaInput.value.trim() : '';

        if (!email) {
            setAuthMessage('请先输入邮箱', true);
            return;
        }
        if (!captcha) {
            setAuthMessage('请先输入图形验证码', true);
            return;
        }

        const formData = new FormData();
        formData.append('email', email);
        formData.append('captcha', captcha);
        formData.append('purpose', purpose);
        if (csrfInput) {
            formData.append('csrf_token', csrfInput.value);
        }

        sendCodeBtn.disabled = true;
        sendCodeBtn.textContent = '发送中...';

        try {
            const response = await fetch('/send-code', { method: 'POST', body: formData });
            const data = await response.json();
            refreshCaptchaImage();

            if (data.error) {
                setAuthMessage(data.error, true);
                sendCodeBtn.disabled = false;
                sendCodeBtn.textContent = '发送验证码';
                return;
            }

            setAuthMessage('验证码已发送，请检查邮箱', false);
            let seconds = 60;
            sendCodeBtn.textContent = `${seconds}s`;
            const timer = window.setInterval(() => {
                seconds -= 1;
                sendCodeBtn.textContent = `${seconds}s`;
                if (seconds <= 0) {
                    window.clearInterval(timer);
                    sendCodeBtn.disabled = false;
                    sendCodeBtn.textContent = '发送验证码';
                }
            }, 1000);

            const codeInput = document.getElementById('code');
            if (codeInput) {
                codeInput.focus();
            }
        } catch (error) {
            refreshCaptchaImage();
            setAuthMessage('网络错误，请稍后重试', true);
            sendCodeBtn.disabled = false;
            sendCodeBtn.textContent = '发送验证码';
        }
    });
}

if (typeof window !== 'undefined') {
    window.addEventListener('DOMContentLoaded', bindAuthPage);
}
