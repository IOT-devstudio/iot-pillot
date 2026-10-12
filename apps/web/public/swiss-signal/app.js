/* Local visual prototype. No requests, credential storage or real submissions. */
(() => {
  'use strict';
  const $ = selector => document.querySelector(selector);
  const reducedMotion = matchMedia('(prefers-reduced-motion: reduce)');
  const profile = { name: '探索者', interest: '还在探索', bio: '' };
  const intents = new Set();
  const directions = {
    frontend: { name: '前端开发', index: '01', tools: 'Vue / TypeScript / 交互体验', desc: '把想法变成可以触摸的界面。从组件、动效到完整流程，让人愿意使用你的作品。', task: '从一个页面开始，参与设计、实现与联调。' },
    backend: { name: '后端开发', index: '02', tools: 'Go / Gin / PostgreSQL', desc: '连接数据、接口与服务。用清晰的逻辑和可靠的实现，让一个想法成为运行中的系统。', task: '从一个接口开始，探索数据建模、测试与部署。' },
    agent: { name: 'Agent 开发', index: '03', tools: 'LLM / 工具调用 / 工作流', desc: '让模型与工具一起工作。探索智能体如何理解问题、执行任务，以及融入真实的产品流程。', task: '从一个工作流开始，让智能真正参与解决问题。' },
  };
  let view = 'login';
  let railFrame;
  let toastTimer;
  let lastOpener;
  let switching = false;
  let pendingView = null;
  const motion = window.SignalMotion;
  const hasMotion = () => !reducedMotion.matches && !document.body.classList.contains('motion-paused');

  function toast(message) {
    $('#toast').textContent = message;
    $('#toast').classList.add('visible');
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => $('#toast').classList.remove('visible'), 3500);
  }
  function updateProfile() {
    $('#nav-name').textContent = profile.name;
    $('#welcome-name').textContent = profile.name;
  }
  function applyView(next, focus = true) {
    if (!['login', 'register', 'home'].includes(next)) next = 'login';
    view = next;
    const home = next === 'home';
    const register = next === 'register';
    $('#auth-view').hidden = home;
    $('#home-view').hidden = !home;
    document.body.dataset.view = next;
    $('#form-index').textContent = register ? 'NEW CHAPTER / 02' : 'ACCESS / 01';
    $('#login-pane').inert = register;
    $('#register-pane').inert = !register;
    $('#login-pane').setAttribute('aria-hidden', String(register));
    $('#register-pane').setAttribute('aria-hidden', String(!register));
    document.querySelectorAll('[data-view]').forEach(button => {
      if (button.closest('.view-nav, .form-tabs')) button.setAttribute('aria-pressed', String(button.dataset.view === next));
    });
    if (location.hash !== '#' + next) history.replaceState(null, '', '#' + next);
    updateProfile();
    if (home) {
      updateIntents();
      requestAnimationFrame(updateRail);
      if (focus) { window.scrollTo({ top: 0, behavior: 'instant' }); $('.home-poster h1').focus({ preventScroll: true }); }
    } else if (focus) {
      window.scrollTo({ top: 0, behavior: 'instant' });
      $(`.form-tabs [data-view=${next}]`).focus({ preventScroll: true });
    }
  }
  async function showView(next, focus = true) {
    if (!['login', 'register', 'home'].includes(next)) next = 'login';
    if (switching) { pendingView = { next, focus }; return; }
    if (next === view) return;
    switching = true;
    const previous = view;
    try {
      if ((next === 'home') !== (previous === 'home')) {
        await motion.transition(() => { applyView(next, focus); motion.enter(next); });
      } else { applyView(next, focus); motion.form(next); }
    } finally {
      switching = false;
      if (pendingView) { const pending = pendingView; pendingView = null; showView(pending.next, pending.focus); }
    }
  }
  document.querySelectorAll('[data-view]').forEach(button => button.addEventListener('click', () => showView(button.dataset.view)));
  window.addEventListener('hashchange', () => {
    const target = location.hash.slice(1);
    if (['login', 'register', 'home'].includes(target)) showView(target);
    else if (['directions', 'journey'].includes(target)) {
      if (view !== 'home') showView('home', false);
      history.replaceState(null, '', '#' + target);
      $('#' + target).scrollIntoView({ behavior: hasMotion() ? 'smooth' : 'instant' });
    }
  });
  document.querySelectorAll('[data-password]').forEach(button => button.addEventListener('click', () => {
    const input = document.getElementById(button.dataset.password);
    const visible = input.type === 'password';
    input.type = visible ? 'text' : 'password';
    button.textContent = visible ? '隐藏' : '显示';
    button.setAttribute('aria-pressed', String(visible));
    button.setAttribute('aria-label', (visible ? '隐藏' : '显示') + (button.dataset.password.startsWith('login') ? '登录' : '注册') + '密码');
  }));
  $('#forgot').addEventListener('click', () => toast('这里是视觉演示，密码找回将在正式功能中接入。'));
  $('#login-form').addEventListener('submit', event => {
    event.preventDefault();
    const form = event.currentTarget;
    if (!form.reportValidity()) return;
    if (form.elements.password.value.length < 8) { $('#login-error').textContent = '密码需要至少 8 位字符。'; return; }
    showView('home');
    form.reset();
    toast('欢迎进入演示空间，未进行真实登录。');
  });
  $('#register-confirm').addEventListener('input', () => $('#register-confirm').setCustomValidity(''));
  $('#register-password').addEventListener('input', () => $('#register-confirm').setCustomValidity(''));
  $('#register-form').addEventListener('submit', event => {
    event.preventDefault();
    const form = event.currentTarget;
    const name = form.elements.name.value.trim();
    $('#register-error').textContent = '';
    if (!name) { $('#register-error').textContent = '请填写你的称呼，不能只输入空格。'; return; }
    if (form.elements.password.value !== form.elements.confirm.value) {
      $('#register-error').textContent = '两次密码不一致，请再确认一下。';
      $('#register-confirm').setCustomValidity('两次密码不一致');
      $('#register-confirm').reportValidity();
      return;
    }
    profile.name = name;
    showView('home');
    form.reset();
    toast('已进入注册后的演示空间，未创建真实账号。');
  });
  const rail = $('#direction-rail');
  const panels = [...rail.children];
  function currentPanel() {
    return panels.reduce((best, panel, index) => Math.abs(panel.offsetLeft - rail.scrollLeft) < Math.abs(panels[best].offsetLeft - rail.scrollLeft) ? index : best, 0);
  }
  function updateRail() {
    const index = currentPanel();
    $('#rail-index').textContent = String(index + 1).padStart(2, '0');
    $('#rail-prev').disabled = index === 0;
    $('#rail-next').disabled = index === panels.length - 1;
  }
  function goPanel(delta) {
    const index = Math.max(0, Math.min(panels.length - 1, currentPanel() + delta));
    rail.scrollTo({ left: panels[index].offsetLeft, behavior: hasMotion() ? 'smooth' : 'instant' });
  }
  $('#rail-prev').addEventListener('click', () => goPanel(-1));
  $('#rail-next').addEventListener('click', () => goPanel(1));
  rail.addEventListener('scroll', () => { cancelAnimationFrame(railFrame); railFrame = requestAnimationFrame(updateRail); }, { passive: true });
  rail.addEventListener('keydown', event => {
    if (event.target !== rail) return;
    if (['ArrowLeft', 'ArrowRight'].includes(event.key)) { event.preventDefault(); goPanel(event.key === 'ArrowLeft' ? -1 : 1); }
  });
  window.addEventListener('resize', updateRail);
  function openDialog(dialog, opener) { lastOpener = opener; dialog.showModal(); motion.dialogOpen(dialog); motion.bindButtons(dialog); }
  const closingDialogs = new WeakSet();
  async function closeDialog(dialog) {
    if (!dialog.open || closingDialogs.has(dialog)) return;
    closingDialogs.add(dialog);
    await motion.dialogClose(dialog);
    dialog.close(); closingDialogs.delete(dialog);
  }
  document.querySelectorAll('dialog').forEach(dialog => {
    dialog.querySelector('.dialog-close').addEventListener('click', () => closeDialog(dialog));
    dialog.addEventListener('cancel', event => { event.preventDefault(); closeDialog(dialog); });
    dialog.addEventListener('click', event => {
      if (event.target !== dialog) return;
      const rect = dialog.getBoundingClientRect();
      if (event.clientX < rect.left || event.clientX > rect.right || event.clientY < rect.top || event.clientY > rect.bottom) closeDialog(dialog);
    });
    dialog.addEventListener('close', () => lastOpener?.focus({ preventScroll: true }));
  });
  function updateIntents() {
    motion.count($('#application-count'), 1 + intents.size);
    const container = $('#new-intents');
    container.hidden = intents.size === 0;
    container.replaceChildren();
    [...intents].forEach(id => {
      const line = document.createElement('p');
      line.textContent = `${directions[id].name} / 本次体验的报名意向 · 刷新后重置`;
      container.append(line);
    });
  }
  document.querySelectorAll('[data-direction]').forEach(button => button.addEventListener('click', () => {
    const id = button.dataset.direction;
    const direction = directions[id];
    $('#detail-content').innerHTML = `<span class="mono">DIRECTION / ${direction.index}</span><h2>${direction.name}</h2><p class="dialog-description">${direction.desc}</p><dl class="direction-facts"><div><dt>探索工具</dt><dd>${direction.tools}</dd></div><div><dt>你的第一步</dt><dd>${direction.task}</dd></div></dl><p class="dialog-note">不必已经精通。带上热情，我们从第一步开始。<br>报名意向只保留在本次体验中，不提交到工作室。</p><button class="action-button" id="apply-intent" ${intents.has(id) ? 'disabled' : ''}><span>${intents.has(id) ? '已记录本次意向' : '体验报名意向'}</span><b>↗</b></button>`;
    $('#apply-intent').addEventListener('click', () => {
      intents.add(id); updateIntents(); closeDialog($('#detail-dialog'));
      toast(`已记录「${direction.name}」的演示意向，未提交真实申请。`);
    });
    openDialog($('#detail-dialog'), button);
  }));
  $('#journey-open').addEventListener('click', event => {
    $('#detail-content').innerHTML = '<span class="mono">APPLICATION / 001 / DEMO</span><h2>你的下一步。</h2><p class="dialog-description">前端开发 · 秋季招新</p><ol class="detail-steps"><li><b>01</b><div>提交申请<small>已完成 · 演示记录</small></div></li><li><b>02</b><div>初步审核<small>正在进行 · 等待团队了解你</small></div></li><li><b>03</b><div>面试交流<small>待安排 · 聊聊你的想法</small></div></li></ol><p class="dialog-note">此处为示例进度，不代表真实申请或审核结果。</p>';
    openDialog($('#detail-dialog'), event.currentTarget);
  });
  function showProfile(event) {
    $('#profile-name').value = profile.name;
    $('#profile-interest').value = profile.interest;
    $('#profile-bio').value = profile.bio;
    $('#profile-error').textContent = '';
    openDialog($('#profile-dialog'), event.currentTarget);
  }
  $('#profile-open').addEventListener('click', showProfile);
  $('#profile-open-bottom').addEventListener('click', showProfile);
  $('#profile-form').addEventListener('submit', event => {
    event.preventDefault();
    const form = event.currentTarget;
    const name = form.elements.name.value.trim();
    if (!name) { $('#profile-error').textContent = '称呼不能只输入空格。'; return; }
    profile.name = name; profile.interest = form.elements.interest.value; profile.bio = form.elements.bio.value.trim();
    updateProfile(); closeDialog($('#profile-dialog')); toast('本次体验资料已更新，刷新后重置。');
  });
  $('#motion-toggle').addEventListener('click', event => {
    const paused = document.body.classList.toggle('motion-paused');
    event.currentTarget.setAttribute('aria-pressed', String(paused));
    event.currentTarget.textContent = paused ? '恢复动效 ▶' : '暂停动效 ⏸';
    motion.sync();
  });
  const observer = new IntersectionObserver(entries => entries.forEach(entry => { if (entry.isIntersecting) { entry.target.classList.add('revealed'); observer.unobserve(entry.target); } }), { threshold: .08 });
  document.querySelectorAll('.section-top, .journey-sheet, .closing-note').forEach(element => { element.classList.add('reveal'); observer.observe(element); });
  applyView(['login', 'register', 'home'].includes(location.hash.slice(1)) ? location.hash.slice(1) : 'login', false);
  motion.enter(view, true);
})();
