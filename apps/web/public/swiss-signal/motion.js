/* Shared 2D motion choreography. Native animations, no remote dependencies. */
(() => {
  'use strict';
  const reduced = matchMedia('(prefers-reduced-motion: reduce)');
  const finePointer = matchMedia('(hover: hover) and (pointer: fine)');
  const active = new Set();
  const ease = 'cubic-bezier(.22,1,.36,1)';
  const enabled = () => !reduced.matches && !document.hidden && !document.body.classList.contains('motion-paused');
  let pointerFrame;
  let scrollFrame;
  let scrollRunning = false;
  let pointerX = 0, pointerY = 0;
  let smoothX = 0, smoothY = 0;

  function animate(element, frames, options = {}, final = null) {
    if (!element || !enabled()) { if (element && final) Object.assign(element.style, final); return Promise.resolve(); }
    const animation = element.animate(frames, { duration: 700, easing: ease, fill: 'both', ...options });
    active.add(animation);
    // Background tabs and suspended browser rendering must not hold UI transitions.
    const deadline = setTimeout(() => {
      if (!active.has(animation)) return;
      try { animation.finish(); } catch { animation.cancel(); }
    }, Number(options.duration || 700) + Number(options.delay || 0) + 350);
    return animation.finished.catch(() => {}).then(() => {
      clearTimeout(deadline);
      if (final) Object.assign(element.style, final);
      active.delete(animation);
      animation.cancel();
    });
  }
  function stagger(elements, distance = 24, delay = 0) {
    [...elements].forEach((element, index) => animate(element, [
      { opacity: 0, transform: `translateY(${distance}px)` },
      { opacity: 1, transform: 'translateY(0)' },
    ], { duration: 650, delay: delay + index * 45 }));
  }
  document.querySelectorAll('.auth-poster h1, .home-poster h1').forEach(heading => {
    heading.innerHTML = heading.innerHTML.split(/<br\s*\/?\s*>/i).map(line => `<span class="title-mask"><span class="title-line">${line}</span></span>`).join('');
  });
  document.querySelectorAll('.auth-poster, .home-poster').forEach(poster => {
    const scanner = document.createElement('div'); scanner.className = 'signal-scanner'; scanner.setAttribute('aria-hidden', 'true'); poster.append(scanner);
    const halo = document.createElement('div'); halo.className = 'signal-halo'; halo.setAttribute('aria-hidden', 'true'); poster.append(halo);
  });
  const wipe = document.createElement('div');
  wipe.className = 'page-wipe'; wipe.hidden = true; wipe.setAttribute('aria-hidden', 'true');
  wipe.innerHTML = '<i></i><i></i><i></i><span class="wipe-word">CONNECTING<br>IDEAS<span>↗</span></span>';
  document.body.append(wipe);
  const strips = [...wipe.querySelectorAll('i')];
  async function transition(change) {
    if (!enabled()) { change(); return; }
    wipe.hidden = false;
    try {
      animate(wipe.querySelector('.wipe-word'), [{ opacity: 0, transform: 'translateY(12px)' }, { opacity: 1, transform: 'translateY(0)' }], { duration: 180, delay: 140 }, { opacity: '1' });
      await Promise.all(strips.map((strip, index) => animate(strip, [
        { transform: 'translateY(105%)' }, { transform: 'translateY(0)' },
      ], { duration: 330, delay: index * 38 }, { transform: 'translateY(0)' })));
      change();
      document.body.classList.add('wipe-revealing');
      animate(wipe.querySelector('.wipe-word'), [{ opacity: 1 }, { opacity: 0 }], { duration: 140 }, { opacity: '0' });
      await Promise.all(strips.map((strip, index) => animate(strip, [
        { transform: 'translateY(0)' }, { transform: 'translateY(-105%)' },
      ], { duration: 540, delay: index * 38 }, { transform: 'translateY(105%)' })));
    } finally { wipe.hidden = true; document.body.classList.remove('wipe-revealing'); }
  }
  function enter(view, initial = false) {
    const root = document.querySelector(view === 'home' ? '.home-poster' : '.auth-poster');
    stagger(root.querySelectorAll('.title-line'), 100, 55);
    stagger(root.querySelectorAll('.poster-top, .poster-copy, .poster-foot, .home-poster-top, .home-poster-bottom'), 18, 160);
    const mark = root.querySelector('.signal-mark');
    const markOpacity = getComputedStyle(mark).opacity;
    animate(mark, [{ opacity: 0, scale: '.6' }, { opacity: markOpacity, scale: '1' }], { duration: 1100, delay: 90 });
    if (view !== 'home') {
      stagger(document.querySelectorAll(`#${view}-pane > *, #${view}-form > label, #${view}-form > input, #${view}-form > .password-field`), 18, initial ? 220 : 100);
    } else stagger(document.querySelectorAll('.space-stats > div'), 20, 240);
  }
  function form(view) {
    stagger(document.querySelectorAll(`#${view}-pane > h2, #${view}-pane > .form-intro, #${view}-form > label, #${view}-form > input, #${view}-form > .password-field, #${view}-form > .action-button`), 18, 100);
  }
  function dialogOpen(dialog) {
    animate(dialog, [{ opacity: 0, transform: 'translateY(28px) scale(.97)' }, { opacity: 1, transform: 'translateY(0) scale(1)' }], { duration: 430 });
    stagger(dialog.querySelectorAll('h2, .dialog-description, .direction-facts > div, .detail-steps li, form > label, form > input, .action-button'), 14, 70);
  }
  function dialogClose(dialog) {
    return animate(dialog, [{ opacity: 1, transform: 'translateY(0) scale(1)' }, { opacity: 0, transform: 'translateY(15px) scale(.98)' }], { duration: 190 });
  }
  function finishAnimations() { [...active].forEach(animation => { try { animation.finish(); } catch { animation.cancel(); } }); }
  function resetDecorations() {
    cancelAnimationFrame(pointerFrame); cancelAnimationFrame(scrollFrame); scrollRunning = false;
    smoothX = smoothY = pointerX = pointerY = 0;
    document.body.style.setProperty('--drift-x', '0px'); document.body.style.setProperty('--drift-y', '0px');
    document.querySelectorAll('[data-magnetic]').forEach(element => element.style.removeProperty('translate'));
    document.querySelectorAll('.direction-panel').forEach(element => { element.style.removeProperty('--chapter-drift'); element.style.removeProperty('--symbol-drift'); });
  }
  function sync() {
    if (!enabled()) { finishAnimations(); resetDecorations(); }
    else requestScroll();
  }
  function followPointer() {
    if (!enabled() || document.hidden) return;
    smoothX += (pointerX - smoothX) * .075; smoothY += (pointerY - smoothY) * .075;
    document.body.style.setProperty('--drift-x', smoothX.toFixed(2) + 'px');
    document.body.style.setProperty('--drift-y', smoothY.toFixed(2) + 'px');
    if (Math.abs(pointerX - smoothX) + Math.abs(pointerY - smoothY) > .05) pointerFrame = requestAnimationFrame(followPointer);
  }
  document.addEventListener('pointermove', event => {
    if (!enabled() || !finePointer.matches || event.pointerType === 'touch') return;
    pointerX = (event.clientX / innerWidth - .5) * 22; pointerY = (event.clientY / innerHeight - .5) * 14;
    cancelAnimationFrame(pointerFrame); pointerFrame = requestAnimationFrame(followPointer);
  }, { passive: true });
  function bindButtons(root = document) {
    root.querySelectorAll('.action-button, .panel-button, .rail-controls button').forEach(button => {
      if (button.dataset.magnetic) return;
      button.dataset.magnetic = 'true';
      button.addEventListener('pointermove', event => {
        if (!enabled() || !finePointer.matches || button.disabled) return;
        const rect = button.getBoundingClientRect();
        const x = Math.max(-5, Math.min(5, (event.clientX - rect.left - rect.width / 2) * .07));
        const y = Math.max(-4, Math.min(4, (event.clientY - rect.top - rect.height / 2) * .1));
        button.style.translate = `${x}px ${y}px`;
      });
      button.addEventListener('pointerleave', () => button.style.translate = '0px 0px');
      button.addEventListener('click', () => {
        if (!enabled() || button.disabled) return;
        animate(button, [{ scale: '1' }, { scale: '.975', offset: .35 }, { scale: '1' }], { duration: 300 });
      });
    });
  }
  bindButtons();
  function requestScroll() {
    if (!enabled() || document.hidden || scrollRunning) return;
    scrollRunning = true;
    scrollFrame = requestAnimationFrame(() => {
      scrollRunning = false;
      const viewport = innerHeight;
      const hero = document.querySelector('.home-poster');
      if (!document.querySelector('#home-view').hidden) {
        const rect = hero.getBoundingClientRect();
        hero.style.setProperty('--scroll-drift', `${Math.max(-25, Math.min(35, -rect.top * .09))}px`);
      }
      const rail = document.querySelector('#direction-rail');
      if (rail.getBoundingClientRect().bottom > 0 && rail.getBoundingClientRect().top < viewport) {
        const center = rail.getBoundingClientRect().left + rail.clientWidth / 2;
        rail.querySelectorAll('.direction-panel').forEach(panel => {
          const rect = panel.getBoundingClientRect();
          const delta = (rect.left + rect.width / 2 - center) / rail.clientWidth;
          panel.style.setProperty('--chapter-drift', `${Math.max(-30, Math.min(30, -delta * 32))}px`);
          panel.style.setProperty('--symbol-drift', `${Math.max(-16, Math.min(16, delta * 22))}px`);
        });
      }
    });
  }
  document.addEventListener('scroll', requestScroll, { passive: true });
  document.querySelector('#direction-rail').addEventListener('scroll', requestScroll, { passive: true });
  window.addEventListener('resize', requestScroll);
  document.addEventListener('visibilitychange', () => { if (document.hidden) { finishAnimations(); resetDecorations(); } else requestScroll(); });
  reduced.addEventListener('change', sync);
  const observed = new IntersectionObserver(entries => entries.forEach(entry => {
    if (!entry.isIntersecting) return;
    observed.unobserve(entry.target);
    if (entry.target.classList.contains('journey-sheet')) stagger(entry.target.querySelectorAll('.journey-steps li'), 18, 120);
    if (entry.target.classList.contains('space-stats')) {
      ['#application-count'].forEach(selector => count(document.querySelector(selector), Number(document.querySelector(selector).dataset.targetCount || document.querySelector(selector).textContent)));
      const number = document.querySelector('.space-stats > div:first-child strong');
      const node = number.firstChild;
      if (enabled()) animateNumber(3, value => { node.textContent = String(value).padStart(2, '0'); });
    }
  }), { threshold: .3 });
  document.querySelectorAll('.journey-sheet, .space-stats').forEach(element => observed.observe(element));
  function animateNumber(target, render) {
    if (!enabled()) { render(target); return; }
    const clock = document.createElement('i');
    const animation = clock.animate([{ opacity: 0 }, { opacity: 1 }], { duration: 850, easing: 'linear' });
    active.add(animation);
    const tick = () => {
      if (animation.playState === 'finished' || animation.playState === 'idle' || !enabled()) { render(target); active.delete(animation); animation.cancel(); return; }
      const progress = Math.min(1, Number(animation.currentTime || 0) / 850);
      render(Math.round(target * (1 - Math.pow(1 - progress, 3))));
      requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  }
  function count(element, value) {
    element.dataset.targetCount = String(value);
    const version = Number(element.dataset.countVersion || 0) + 1;
    element.dataset.countVersion = String(version);
    animateNumber(value, current => { if (element.dataset.countVersion === String(version)) element.textContent = String(current).padStart(2, '0'); });
  }
  window.SignalMotion = { enter, form, transition, dialogOpen, dialogClose, sync, bindButtons, count, enabled };
})();
