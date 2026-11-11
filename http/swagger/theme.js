(function () {
  var STORAGE_KEY = 'swagger-ui-theme';
  var STYLE_ID = 'swagger-ui-theme-style';
  var TOGGLE_ID = 'swagger-ui-theme-toggle';

  var css = `
#swagger-ui-theme-toggle {
  position: fixed;
  right: 20px;
  bottom: 20px;
  z-index: 9999;
  padding: 8px 14px;
  border-radius: 999px;
  border: 1px solid #d0d7de;
  background: #ffffff;
  color: #3b4151;
  font: 500 13px/1.2 -apple-system, "Segoe UI", sans-serif;
  cursor: pointer;
  opacity: .8;
  box-shadow: 0 4px 14px rgba(0, 0, 0, .16);
  transition: opacity .15s ease;
}
#swagger-ui-theme-toggle:hover { opacity: 1; }

html[data-swagger-theme="dark"] body { background: #1c2128; color: #adbac7; }
html[data-swagger-theme="dark"] #swagger-ui-theme-toggle {
  border-color: #444c56;
  background: #22272e;
  color: #cdd9e5;
  box-shadow: 0 4px 14px rgba(0, 0, 0, .5);
}
html[data-swagger-theme="dark"] ::-webkit-scrollbar { width: 12px; height: 12px; }
html[data-swagger-theme="dark"] ::-webkit-scrollbar-track { background: #161b22; }
html[data-swagger-theme="dark"] ::-webkit-scrollbar-thumb { background: #3d444d; border-radius: 6px; }

html[data-swagger-theme="dark"] .swagger-ui { color: #adbac7; }
html[data-swagger-theme="dark"] .swagger-ui .topbar { background-color: #161b22; border-bottom: 1px solid #373e47; }
html[data-swagger-theme="dark"] .swagger-ui .topbar .download-url-wrapper .select-label { color: #adbac7; }
html[data-swagger-theme="dark"] .swagger-ui .info .title { color: #cdd9e5; }
html[data-swagger-theme="dark"] .swagger-ui .info .title small { background: #22272e; color: #adbac7; }
html[data-swagger-theme="dark"] .swagger-ui .info .title small pre { background: #2d333b; border: 1px solid #373e47; color: #adbac7; }
html[data-swagger-theme="dark"] .swagger-ui .info .base-url { color: #768390; }
html[data-swagger-theme="dark"] .swagger-ui .info p,
html[data-swagger-theme="dark"] .swagger-ui .info li,
html[data-swagger-theme="dark"] .swagger-ui .info table,
html[data-swagger-theme="dark"] .swagger-ui .info .markdown p { color: #adbac7; }
html[data-swagger-theme="dark"] .swagger-ui .info a,
html[data-swagger-theme="dark"] .swagger-ui .info .markdown a { color: #539bf5; }
html[data-swagger-theme="dark"] .swagger-ui .info h1,
html[data-swagger-theme="dark"] .swagger-ui .info h2,
html[data-swagger-theme="dark"] .swagger-ui .info h3,
html[data-swagger-theme="dark"] .swagger-ui .info h4,
html[data-swagger-theme="dark"] .swagger-ui .info h5 { color: #cdd9e5; }

html[data-swagger-theme="dark"] .swagger-ui .scheme-container { background: #22272e; box-shadow: 0 1px 2px 0 rgba(0, 0, 0, .5); }
html[data-swagger-theme="dark"] .swagger-ui .servers-title { color: #adbac7; }

html[data-swagger-theme="dark"] .swagger-ui select,
html[data-swagger-theme="dark"] .swagger-ui input[type="text"],
html[data-swagger-theme="dark"] .swagger-ui input[type="password"],
html[data-swagger-theme="dark"] .swagger-ui input[type="email"],
html[data-swagger-theme="dark"] .swagger-ui input[type="search"],
html[data-swagger-theme="dark"] .swagger-ui textarea {
  background: #2d333b;
  color: #cdd9e5;
  border: 1px solid #444c56;
}
html[data-swagger-theme="dark"] .swagger-ui .btn { background: transparent; color: #adbac7; border-color: #444c56; }
html[data-swagger-theme="dark"] .swagger-ui .btn.authorize { color: #57ab5a; border-color: #57ab5a; }
html[data-swagger-theme="dark"] .swagger-ui .btn.authorize svg { fill: #57ab5a; }
html[data-swagger-theme="dark"] .swagger-ui .btn.authorize.locked { color: #e5534b; border-color: #e5534b; }
html[data-swagger-theme="dark"] .swagger-ui .btn.authorize.locked svg { fill: #e5534b; }

html[data-swagger-theme="dark"] .swagger-ui .opblock-tag { color: #cdd9e5; border-bottom: 1px solid #373e47; }
html[data-swagger-theme="dark"] .swagger-ui .opblock-tag small { color: #768390; }
html[data-swagger-theme="dark"] .swagger-ui .opblock-tag svg,
html[data-swagger-theme="dark"] .swagger-ui .expand-operation svg,
html[data-swagger-theme="dark"] .swagger-ui .models-control svg { fill: #768390; }
html[data-swagger-theme="dark"] .swagger-ui .opblock .opblock-summary-description { color: #adbac7; }
html[data-swagger-theme="dark"] .swagger-ui .opblock .opblock-summary-path,
html[data-swagger-theme="dark"] .swagger-ui .opblock .opblock-summary-path a,
html[data-swagger-theme="dark"] .swagger-ui .opblock .opblock-summary-operation-id { color: #cdd9e5; }
html[data-swagger-theme="dark"] .swagger-ui .opblock .opblock-summary { border-bottom: 1px solid rgba(255, 255, 255, .08); }
html[data-swagger-theme="dark"] .swagger-ui .opblock .opblock-section-header { background: #2d333b; box-shadow: none; }
html[data-swagger-theme="dark"] .swagger-ui .opblock .opblock-section-header h4,
html[data-swagger-theme="dark"] .swagger-ui .opblock .opblock-section-header label { color: #cdd9e5; }
html[data-swagger-theme="dark"] .swagger-ui .opblock-description-wrapper p,
html[data-swagger-theme="dark"] .swagger-ui .opblock-external-docs-wrapper p,
html[data-swagger-theme="dark"] .swagger-ui .opblock-title_normal p { color: #adbac7; }
html[data-swagger-theme="dark"] .swagger-ui .opblock-body { background: #22272e; }
html[data-swagger-theme="dark"] .swagger-ui .opblock.opblock-get { background: rgba(97, 175, 254, .07); }
html[data-swagger-theme="dark"] .swagger-ui .opblock.opblock-post { background: rgba(73, 204, 144, .07); }
html[data-swagger-theme="dark"] .swagger-ui .opblock.opblock-put { background: rgba(252, 161, 48, .07); }
html[data-swagger-theme="dark"] .swagger-ui .opblock.opblock-delete { background: rgba(249, 62, 62, .07); }
html[data-swagger-theme="dark"] .swagger-ui .opblock.opblock-patch { background: rgba(80, 227, 194, .07); }
html[data-swagger-theme="dark"] .swagger-ui .opblock.opblock-head { background: rgba(144, 18, 254, .07); }
html[data-swagger-theme="dark"] .swagger-ui .opblock.opblock-options { background: rgba(13, 90, 167, .07); }

html[data-swagger-theme="dark"] .swagger-ui table thead tr td,
html[data-swagger-theme="dark"] .swagger-ui table thead tr th { color: #cdd9e5; border-bottom: 1px solid #373e47; }
html[data-swagger-theme="dark"] .swagger-ui .parameters-col_description p,
html[data-swagger-theme="dark"] .swagger-ui .response-col_description p,
html[data-swagger-theme="dark"] .swagger-ui .parameter__extension,
html[data-swagger-theme="dark"] .swagger-ui .parameter__deprecated { color: #adbac7; }
html[data-swagger-theme="dark"] .swagger-ui .parameter__name { color: #cdd9e5; }
html[data-swagger-theme="dark"] .swagger-ui .parameter__type { color: #768390; }
html[data-swagger-theme="dark"] .swagger-ui .parameter__in { color: #768390; }
html[data-swagger-theme="dark"] .swagger-ui .response-col_status { color: #cdd9e5; }
html[data-swagger-theme="dark"] .swagger-ui .response-col_links { color: #768390; }
html[data-swagger-theme="dark"] .swagger-ui .responses-inner h4,
html[data-swagger-theme="dark"] .swagger-ui .responses-inner h5 { color: #cdd9e5; }
html[data-swagger-theme="dark"] .swagger-ui .tab li button { color: #adbac7; }
html[data-swagger-theme="dark"] .swagger-ui .highlight-code > .microlight,
html[data-swagger-theme="dark"] .swagger-ui .opblock-body pre.microlight,
html[data-swagger-theme="dark"] .swagger-ui .model-example .highlight-code { background: #161b22 !important; color: #cdd9e5; }
html[data-swagger-theme="dark"] .swagger-ui .microlight .token.string,
html[data-swagger-theme="dark"] .swagger-ui .microlight .token.attr-value { color: #96d0ff; }
html[data-swagger-theme="dark"] .swagger-ui .microlight .token.number,
html[data-swagger-theme="dark"] .swagger-ui .microlight .token.boolean { color: #f69d50; }
html[data-swagger-theme="dark"] .swagger-ui .microlight .token.property,
html[data-swagger-theme="dark"] .swagger-ui .microlight .token.attr-name,
html[data-swagger-theme="dark"] .swagger-ui .microlight .token.key { color: #8ddb8c; }
html[data-swagger-theme="dark"] .swagger-ui .copy-to-clipboard { background: #2d333b; border: 1px solid #373e47; }
html[data-swagger-theme="dark"] .swagger-ui .copy-to-clipboard button { background: transparent; }
html[data-swagger-theme="dark"] .swagger-ui .copy-to-clipboard svg { fill: #adbac7; }

html[data-swagger-theme="dark"] .swagger-ui section.models { border: 1px solid #373e47; }
html[data-swagger-theme="dark"] .swagger-ui section.models.is-open h4 { border-bottom: 1px solid #373e47; }
html[data-swagger-theme="dark"] .swagger-ui section.models h4 { color: #cdd9e5; }
html[data-swagger-theme="dark"] .swagger-ui section.models .model-container { background: #22272e; }
html[data-swagger-theme="dark"] .swagger-ui .model-box { background: #22272e; }
html[data-swagger-theme="dark"] .swagger-ui .model,
html[data-swagger-theme="dark"] .swagger-ui .model-title,
html[data-swagger-theme="dark"] .swagger-ui .model-title__text { color: #cdd9e5; }
html[data-swagger-theme="dark"] .swagger-ui .model .property { color: #768390; }
html[data-swagger-theme="dark"] .swagger-ui .model .property.primitive { color: #8ddb8c; }
html[data-swagger-theme="dark"] .swagger-ui .prop-type { color: #7aa2f7; }
html[data-swagger-theme="dark"] .swagger-ui .prop-format,
html[data-swagger-theme="dark"] .swagger-ui .model-hint { color: #768390; }
html[data-swagger-theme="dark"] .swagger-ui .model-hint { background: #2d333b; }
html[data-swagger-theme="dark"] .swagger-ui .model-toggle:after { filter: invert(1) brightness(1.4); }

html[data-swagger-theme="dark"] .swagger-ui .dialog-ux .backdrop-ux { background: rgba(0, 0, 0, .7); }
html[data-swagger-theme="dark"] .swagger-ui .dialog-ux .modal-ux { background: #22272e; border: 1px solid #373e47; box-shadow: 0 10px 30px rgba(0, 0, 0, .6); }
html[data-swagger-theme="dark"] .swagger-ui .dialog-ux .modal-ux-header { border-bottom: 1px solid #373e47; }
html[data-swagger-theme="dark"] .swagger-ui .dialog-ux .modal-ux-header h3 { color: #cdd9e5; }
html[data-swagger-theme="dark"] .swagger-ui .dialog-ux .modal-ux-content p,
html[data-swagger-theme="dark"] .swagger-ui .dialog-ux .modal-ux-content h4,
html[data-swagger-theme="dark"] .swagger-ui .dialog-ux .modal-ux-content label { color: #adbac7; }
html[data-swagger-theme="dark"] .swagger-ui .auth-container { border-bottom: 1px solid #373e47; }
html[data-swagger-theme="dark"] .swagger-ui .scopes h2 { color: #cdd9e5; }
html[data-swagger-theme="dark"] .swagger-ui .errors-wrapper { background: rgba(229, 83, 75, .12); border: 2px solid #e5534b; }
html[data-swagger-theme="dark"] .swagger-ui .errors-wrapper .errors h4,
html[data-swagger-theme="dark"] .swagger-ui .errors-wrapper hgroup h4 { color: #e5534b; }
html[data-swagger-theme="dark"] .swagger-ui .markdown code,
html[data-swagger-theme="dark"] .swagger-ui .renderedMarkdown code { background: rgba(255, 255, 255, .08); color: #cdd9e5; }
html[data-swagger-theme="dark"] .swagger-ui .markdown pre,
html[data-swagger-theme="dark"] .swagger-ui .renderedMarkdown pre { background: #161b22; color: #cdd9e5; }
`;

  var MODES = ['system', 'dark', 'light'];

  var LABELS = { system: '🖥 系统', dark: '🌙 深色', light: '☀ 浅色' };

  var TITLES = {
    system: '跟随系统，点击切换到深色',
    dark: '深色模式，点击切换到浅色',
    light: '浅色模式，点击切换到跟随系统',
  };

  var media = window.matchMedia ? window.matchMedia('(prefers-color-scheme: dark)') : null;

  // mode 返回当前模式；未写入 localStorage 时视为跟随系统。
  function mode() {
    try {
      var value = localStorage.getItem(STORAGE_KEY);
      return value === 'dark' || value === 'light' ? value : 'system';
    } catch (error) {
      return 'system';
    }
  }

  // resolve 把模式解析成实际生效的主题，跟随系统时取系统偏好。
  function resolve(current) {
    if (current !== 'system') {
      return current;
    }

    return media && media.matches ? 'dark' : 'light';
  }

  var style = document.getElementById(STYLE_ID);

  if (!style) {
    style = document.createElement('style');
    style.id = STYLE_ID;
    style.textContent = css;
    document.head.appendChild(style);
  }

  function apply() {
    var current = mode();

    document.documentElement.setAttribute('data-swagger-theme', resolve(current));
    document.documentElement.setAttribute('data-swagger-mode', current);

    var toggle = document.getElementById(TOGGLE_ID);

    if (toggle) {
      toggle.textContent = LABELS[current];
      toggle.title = TITLES[current];
    }
  }

  apply();

  var toggle = document.getElementById(TOGGLE_ID);

  if (!toggle) {
    toggle = document.createElement('button');
    toggle.id = TOGGLE_ID;
    toggle.type = 'button';
    toggle.addEventListener('click', function () {
      var next = MODES[(MODES.indexOf(mode()) + 1) % MODES.length];

      try {
        if (next === 'system') {
          localStorage.removeItem(STORAGE_KEY);
        } else {
          localStorage.setItem(STORAGE_KEY, next);
        }
      } catch (error) {
        // 隐私模式下无法持久化，忽略即可。
      }

      apply();
    });
    document.body.appendChild(toggle);
  }

  if (media) {
    var onChange = function () {
      if (mode() === 'system') {
        apply();
      }
    };

    if (media.addEventListener) {
      media.addEventListener('change', onChange);
    } else if (media.addListener) {
      media.addListener(onChange);
    }
  }

  apply();
})();
