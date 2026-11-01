// web.js is the page's half of pkg/platform/web: the DOM the platform
// builds (windows as canvases, menus, dialogs, a console) and the events
// it hands to Go. Go evaluates this file as the body of a function and
// uses what it returns (the object W at the end).
//
// Rules the two halves keep to:
//  - A callback into Go never waits for Go: Go's callbacks only queue
//    work for its event goroutine (a Go function called from JavaScript
//    that blocks would deadlock the page). The only values a callback
//    returns are decided without the application: whether a key event is
//    kept from the browser.
//  - Go owns every state of the application; this file owns the
//    presentation: open menus, focus, dialogs' widgets, scroll boxes.
"use strict";
const doc = document;

function h(tag, cls, text) {
	const e = doc.createElement(tag);
	if (cls) e.className = cls;
	if (text != null) e.textContent = text;
	return e;
}

// mnemonic splits a label at its '&': the text without it, the index of
// the character it marked (-1 for none) and that character in lower
// case. "&&" is a literal '&'.
function mnemonic(label) {
	let text = "", at = -1;
	for (let i = 0; i < label.length; i++) {
		const c = label[i];
		if (c === "&" && i + 1 < label.length) {
			i++;
			if (label[i] !== "&" && at < 0) at = text.length;
			text += label[i];
		} else {
			text += c;
		}
	}
	return { text, at, key: at >= 0 ? text[at].toLowerCase() : "" };
}

// labelSpan is a span with the label and its marked character underlined.
function labelSpan(label, cls) {
	const m = mnemonic(label), s = h("span", cls);
	if (m.at < 0) {
		s.textContent = m.text;
	} else {
		s.append(m.text.slice(0, m.at), h("u", "", m.text[m.at]), m.text.slice(m.at + 1));
	}
	s.mnemonic = m.key;
	return s;
}

// ---------------------------------------------------------------- layout

let root, menubarSlot, bodyEl, mainEl, splitEl, sideEl, consoleSlot, filesBtn;
const toolWindows = new Set();
let modalDepth = 0;

function init() {
	root = h("div", "t-root");
	menubarSlot = h("div", "t-menubar");
	bodyEl = h("div", "t-body");
	mainEl = h("div", "t-main");
	splitEl = h("div", "t-split");
	sideEl = h("div", "t-side");
	consoleSlot = h("div", "t-console-slot");
	splitEl.style.display = sideEl.style.display = "none";
	bodyEl.append(mainEl, splitEl, sideEl);
	root.append(menubarSlot, bodyEl, consoleSlot);
	doc.body.append(root);
	filesBtn = h("button", "t-files-btn", "Files");
	filesBtn.title = "Files in the browser's storage: download, delete, upload";
	filesBtn.addEventListener("click", showFiles);
	menubarSlot.append(h("div", "t-bar"), filesBtn);

	// The splitter moves the side column's width.
	splitEl.addEventListener("pointerdown", (e) => {
		e.preventDefault();
		splitEl.setPointerCapture(e.pointerId);
		const move = (m) => {
			const r = bodyEl.getBoundingClientRect();
			sideEl.style.width = Math.max(200, Math.min(r.right - m.clientX, r.width - 300)) + "px";
		};
		const up = () => { splitEl.removeEventListener("pointermove", move); splitEl.removeEventListener("pointerup", up); };
		splitEl.addEventListener("pointermove", move);
		splitEl.addEventListener("pointerup", up);
	});
	doc.addEventListener("dragover", (e) => { if (e.dataTransfer && [...e.dataTransfer.types].includes("Files")) e.preventDefault(); });
	doc.addEventListener("drop", (e) => {
		if (!e.dataTransfer || !e.dataTransfer.files.length) return;
		e.preventDefault();
		if (W.onDropFiles) putFiles([...e.dataTransfer.files]).then((paths) => W.onDropFiles(JSON.stringify(paths)));
	});
}

function updateSide() {
	const any = [...toolWindows].some((w) => w.visible);
	sideEl.style.display = splitEl.style.display = any ? "flex" : "none";
}

// putFiles reads files into the in-memory file system, in the home
// folder, and gives the paths.
async function putFiles(files) {
	const paths = [];
	for (const f of files) {
		const path = "/home/user/" + f.name;
		memfs.put(path, new Uint8Array(await f.arrayBuffer()));
		paths.push(path);
	}
	return paths;
}

// ---------------------------------------------------------------- windows

// createWindow makes a window: the main one fills the page's left, a tool
// window is a panel in the column at the right. cb has the functions
// Go gave: mouse, resize, scroll, editDone, editEscape, close,
// fullscreen.
function createWindow(isTool, cb) {
	const wnd = { cb, visible: !isTool, minW: 0, minH: 0, tip: null, tipTimer: 0, pointer: null, frame: null, spacer: 1, sbLast: -1, lo: 0, hi: 1, page: 1 };
	const box = h("div", isTool ? "t-panel" : "t-win");
	const content = h("div", "t-content");
	const wrap = h("div", "t-wrap");
	wrap.tabIndex = 0;
	const sb = h("div", "t-sb");
	const spacer = h("div", "t-sb-spacer");
	sb.append(spacer);
	sb.style.display = "none";
	content.append(wrap, sb);
	if (isTool) {
		const bar = h("div", "t-panel-bar");
		wnd.titleEl = h("span", "t-panel-title", "");
		const close = h("button", "t-x", "✕");
		close.title = "Close";
		close.addEventListener("click", () => cb.close());
		bar.append(wnd.titleEl, close);
		box.append(bar, content);
		box.style.display = "none";
		sideEl.append(box);
		toolWindows.add(wnd);
	} else {
		box.append(content);
		mainEl.append(box);
	}
	Object.assign(wnd, { box, content, wrap, sb, spacerEl: spacer });
	wnd.canvas = null;
	resetCanvas(wnd);

	const pos = (e) => {
		const r = wrap.getBoundingClientRect();
		return [e.clientX - r.left, e.clientY - r.top];
	};
	const send = (type, e, button, delta, mode) => {
		const [x, y] = pos(e);
		cb.mouse(type, x, y, e.shiftKey, e.ctrlKey || e.metaKey, button, delta, e.clientX, e.clientY, e.timeStamp, mode);
	};
	wrap.addEventListener("pointerdown", (e) => {
		if (e.target !== wnd.canvas) return;
		e.preventDefault();
		const a = doc.activeElement;
		if (a && a !== doc.body && a !== wrap && a.blur) a.blur();
		wrap.focus({ preventScroll: true });
		wrap.setPointerCapture(e.pointerId);
		hideTip(wnd);
		send(1, e, e.button, 0, 0);
	});
	wrap.addEventListener("pointerup", (e) => {
		if (!wrap.hasPointerCapture(e.pointerId)) return;
		wrap.releasePointerCapture(e.pointerId);
		send(3, e, e.button, 0, 0);
	});
	wrap.addEventListener("pointermove", (e) => {
		if (e.target !== wnd.canvas && !wrap.hasPointerCapture(e.pointerId)) return;
		wnd.pointer = pos(e);
		tipMove(wnd, e);
		send(0, e, e.buttons, 0, 0);
	});
	wrap.addEventListener("pointerleave", (e) => {
		if (wrap.hasPointerCapture(e.pointerId)) return;
		hideTip(wnd);
		send(5, e, 0, 0, 0);
	});
	wrap.addEventListener("wheel", (e) => {
		if (e.target !== wnd.canvas) return;
		e.preventDefault();
		hideTip(wnd);
		send(4, e, 0, e.deltaY, e.deltaMode);
	}, { passive: false });
	wrap.addEventListener("contextmenu", (e) => e.preventDefault());
	wrap.addEventListener("focus", () => cb.focus());

	new ResizeObserver(() => cb.resize()).observe(wrap);
	sb.addEventListener("scroll", () => {
		if (sb.scrollTop === wnd.sbLast) return; // our own doing
		wnd.sbLast = sb.scrollTop;
		cb.scroll(sb.scrollTop, wnd.spacer, wnd.lo, wnd.hi);
	});
	return wnd;
}

// resetCanvas puts a new canvas in the window: a canvas has one kind of
// context for good, so changing between 2D and WebGL needs another.
function resetCanvas(wnd) {
	const c = h("canvas", "t-canvas");
	if (wnd.canvas) wnd.canvas.replaceWith(c); else wnd.wrap.prepend(c);
	wnd.canvas = c;
	wnd.frame = null;
	wnd.ctx = null;
	return c;
}

const windowMethods = {
	getCanvas() { return this.canvas; },
	newCanvas() { return resetCanvas(this); },
	setTitle(t) { if (this.titleEl) this.titleEl.textContent = t; },
	setVisible(v) {
		this.visible = v;
		this.box.style.display = v ? "flex" : "none";
		if (!v) hideTip(this);
		updateSide();
	},
	focus() { this.wrap.focus({ preventScroll: true }); },
	// size is the drawing area in CSS pixels; a window that is not shown
	// has none.
	size() { return [this.wrap.clientWidth, this.wrap.clientHeight]; },
	setMinSize(w, h2) {
		this.minW = w; this.minH = h2;
		this.wrap.style.minWidth = w + "px";
		this.wrap.style.minHeight = h2 + "px";
		if (this.titleEl) { // a panel is at least as wide as its content
			this.box.style.minWidth = w + "px";
			if (!sideEl.style.width) sideEl.style.width = Math.round(Math.min(Math.max(w + 40, 560), innerWidth * 0.45)) + "px";
		}
	},
	setColumnWidth(w) { sideEl.style.width = w + "px"; },
	setCursor(name) { this.canvas.style.cursor = name; },
	setBacking(w, h2) { this.canvas.width = w; this.canvas.height = h2; },
	// frameBuffer is the bytes of the next frame, which Go fills; putFrame
	// shows them.
	frameBuffer(w, h2) {
		if (!this.frame || this.frame.width !== w || this.frame.height !== h2) {
			if (!this.ctx) this.ctx = this.canvas.getContext("2d");
			if (this.canvas.width !== w) this.canvas.width = w;
			if (this.canvas.height !== h2) this.canvas.height = h2;
			this.frame = this.ctx.createImageData(w, h2);
		}
		return this.frame.data;
	},
	putFrame() { this.ctx.putImageData(this.frame, 0, 0); },
	setTooltip(text, x, y, w, h2) {
		hideTip(this);
		this.tip = text ? { text, x, y, w, h: h2 } : null;
	},
	// The editor is a text box over the drawing area.
	showEditor(x, y, fontHeight, minWidth, mono, text) {
		let ed = this.editor;
		if (!ed) {
			ed = this.editor = h("input", "t-editor");
			ed.type = "text";
			ed.spellcheck = false;
			ed.autocomplete = "off";
			ed.addEventListener("keydown", (e) => {
				if (e.key === "Enter") { e.preventDefault(); this.cb.editDone(ed.value); }
				else if (e.key === "Escape") { e.preventDefault(); this.cb.editEscape(); }
				e.stopPropagation();
			});
			this.wrap.append(ed);
		}
		const font = fontHeight + "px " + (mono ? "ui-monospace, Menlo, Consolas, monospace" : "system-ui, sans-serif");
		ed.style.font = font;
		ed.value = text;
		const m = measureText(text + " ", font);
		ed.style.width = Math.max(minWidth, m + 8) + "px";
		// x, y is the baseline of the text.
		ed.style.left = (x - 3) + "px";
		ed.style.top = (y - fontHeight * 0.95 - 3) + "px";
		ed.style.height = (fontHeight * 1.2 + 4) + "px";
		ed.style.display = "block";
		ed.focus();
		ed.select();
	},
	hideEditor() {
		if (!this.editor || this.editor.style.display === "none") return;
		this.editor.style.display = "none";
		this.wrap.focus({ preventScroll: true });
	},
	editorVisible() { return !!this.editor && this.editor.style.display !== "none"; },
	setScrollbarVisible(v) { this.sb.style.display = v ? "block" : "none"; },
	// configureScrollbar sets the range of values; it keeps the position
	// by value, which Go gives back with setScrollbarPosition.
	configureScrollbar(lo, hi, page, value) {
		this.lo = lo; this.hi = hi; this.page = page;
		const track = this.sb.clientHeight;
		const span = hi - lo;
		this.spacer = span <= page || page <= 0 ? track : track * span / page;
		this.spacerEl.style.height = this.spacer + "px";
		this.setScrollbarPosition(value);
	},
	setScrollbarPosition(value) {
		const span = this.hi - this.lo;
		const top = span > 0 ? (value - this.lo) * this.spacer / span : 0;
		this.sb.scrollTop = top;
		this.sbLast = this.sb.scrollTop;
	},
	setFullScreen(on) {
		if (on && !doc.fullscreenElement) doc.documentElement.requestFullscreen?.().catch(() => {});
		else if (!on && doc.fullscreenElement) doc.exitFullscreen().catch(() => {});
	},
};
// windowObject gives the methods to a window made by createWindow.
function makeWindow(isTool, cb) {
	const wnd = createWindow(isTool, cb);
	return Object.assign(wnd, windowMethods);
}
doc.addEventListener("fullscreenchange", () => { for (const f of fullscreenListeners) f(!!doc.fullscreenElement); });
const fullscreenListeners = new Set();

let measureCtx = null;
function measureText(text, font) {
	if (!measureCtx) measureCtx = h("canvas").getContext("2d");
	measureCtx.font = font;
	return measureCtx.measureText(text).width;
}

// tooltips: shown under the pointer after it rests in the area for half
// a second.
let tipEl = null;
function hideTip(wnd) {
	clearTimeout(wnd.tipTimer);
	wnd.tipTimer = 0;
	if (tipEl) tipEl.style.display = "none";
}
function tipMove(wnd, e) {
	const t = wnd.tip;
	if (!t) return;
	const [x, y] = wnd.pointer;
	const inside = x >= t.x && x < t.x + t.w && y >= t.y && y < t.y + t.h;
	if (!inside) { hideTip(wnd); return; }
	if (tipEl && tipEl.style.display !== "none") return;
	clearTimeout(wnd.tipTimer);
	wnd.tipTimer = setTimeout(() => {
		if (!tipEl) { tipEl = h("div", "t-tooltip"); doc.body.append(tipEl); }
		tipEl.textContent = t.text;
		tipEl.style.display = "block";
		const w = tipEl.offsetWidth;
		tipEl.style.left = Math.max(0, Math.min(e.clientX + 12, innerWidth - w - 4)) + "px";
		tipEl.style.top = (e.clientY + 20 + 24 > innerHeight ? e.clientY - 30 : e.clientY + 20) + "px";
	}, 500);
}

// ---------------------------------------------------------------- menus

// One menu is open at a time: path[0] is a bar menu or a popup, and
// each next entry the submenu open in the one before. Menus are lists;
// an item that has a submenu holds its list.
let barEl = null;
const bar = { menus: [] };
let path = [];
let popup = null; // {ul, backdrop, done}

function menuObject(isBar) {
	const ul = h("ul", "t-menu");
	const m = { ul, items: [], isBar };
	return Object.assign(m, menuMethods);
}

const menuMethods = {
	addItem(label, mnemonics, onTrigger) {
		const li = h("li", "t-mi");
		const check = h("span", "t-mi-ind");
		const text = mnemonics ? labelSpan(label, "t-mi-label") : h("span", "t-mi-label", label);
		const accel = h("span", "t-mi-accel");
		li.append(check, text, accel);
		li.mnemonic = text.mnemonic || "";
		li.addEventListener("click", (e) => {
			e.stopPropagation();
			if (li.classList.contains("dis")) return;
			closeMenus();
			onTrigger();
		});
		this.ul.append(li);
		const item = { li, accel, check };
		this.items.push(item);
		return Object.assign(item, itemMethods);
	},
	addSub(label) {
		const li = h("li", "t-mi t-sub");
		li.append(h("span", "t-mi-ind"), labelSpan(label, "t-mi-label"), h("span", "t-mi-accel"), h("span", "t-mi-arrow", "▸"));
		li.mnemonic = li.querySelector(".t-mi-label").mnemonic;
		const sub = menuObject(false);
		sub.ul.classList.add("t-submenu");
		li.append(sub.ul);
		li.addEventListener("click", (e) => { e.stopPropagation(); li.classList.toggle("open"); });
		li.addEventListener("pointerenter", () => {
			for (const o of li.parentElement.querySelectorAll(":scope > .t-sub.open")) if (o !== li) o.classList.remove("open");
		});
		this.ul.append(li);
		this.items.push({ li, sub });
		return sub;
	},
	addSeparator() { this.ul.append(h("li", "t-sep")); },
	clear() { this.ul.replaceChildren(); this.items = []; },
	// popup shows the menu at the pointer until it is dismissed, then
	// calls done.
	popup(x, y, done) {
		closeMenus();
		const backdrop = h("div", "t-backdrop");
		backdrop.addEventListener("pointerdown", (e) => { e.preventDefault(); closeMenus(); });
		backdrop.addEventListener("contextmenu", (e) => { e.preventDefault(); closeMenus(); });
		this.ul.classList.add("t-popup", "open");
		doc.body.append(backdrop, this.ul);
		const r = this.ul.getBoundingClientRect();
		this.ul.style.left = Math.max(0, Math.min(x, innerWidth - r.width - 2)) + "px";
		this.ul.style.top = Math.max(0, Math.min(y, innerHeight - r.height - 2)) + "px";
		popup = { menu: this, backdrop, done };
		path = [this];
		this.ul.classList.add("open");
	},
};

const itemMethods = {
	setAccel(s) { this.accel.textContent = s; },
	// setIndicator: 0 none, 1 check, 2 radio.
	setIndicator(n) { this.li.dataset.ind = n; this.check.textContent = ""; this.li.classList.toggle("has-ind", n !== 0); this.refreshMark(); },
	setEnabled(on) { this.li.classList.toggle("dis", !on); },
	setActive(on) { this.li.classList.toggle("on", on); this.refreshMark(); },
	refreshMark() {
		const n = Number(this.li.dataset.ind || 0), on = this.li.classList.contains("on");
		this.check.textContent = n === 0 || !on ? "" : n === 1 ? "✓" : "●";
	},
};

function closeMenus() {
	for (const m of bar.menus) m.ul.classList.remove("open");
	for (const b of doc.querySelectorAll(".t-bar-item.open")) b.classList.remove("open");
	for (const o of doc.querySelectorAll(".t-sub.open")) o.classList.remove("open");
	for (const a of doc.querySelectorAll(".t-mi.act")) a.classList.remove("act");
	path = [];
	if (popup) {
		const p = popup;
		popup = null;
		p.backdrop.remove();
		p.menu.ul.remove();
		p.menu.ul.classList.remove("t-popup", "open");
		p.menu.ul.style.left = p.menu.ul.style.top = "";
		p.done();
	}
}

function barObject() {
	if (!barEl) barEl = menubarSlot.querySelector(".t-bar");
	return Object.assign({}, barMethods);
}

const barMethods = {
	addSub(label) {
		const m = menuObject(true);
		const b = h("div", "t-bar-item");
		const text = labelSpan(label, "t-bar-label");
		b.append(text, m.ul);
		b.mnemonic = text.mnemonic;
		m.button = b;
		// byHover: the menu opened because the pointer came over it
		// while another was open, so a click on it keeps it open.
		let byHover = false;
		const openIt = (hover) => {
			closeMenus();
			b.classList.add("open");
			m.ul.classList.add("open");
			path = [m];
			byHover = !!hover;
		};
		b.addEventListener("pointerdown", (e) => {
			if (m.ul.contains(e.target)) return; // in the menu or a submenu of it
			e.preventDefault();
			if (b.classList.contains("open") && !byHover) closeMenus(); else openIt(false);
		});
		b.addEventListener("pointerenter", () => {
			if (path.length && !popup && path[0] !== m) openIt(true);
		});
		m.open = openIt;
		bar.menus.push(m);
		barEl.append(b);
		return m;
	},
	clear() { closeMenus(); bar.menus = []; barEl.replaceChildren(); },
};

doc.addEventListener("pointerdown", (e) => {
	if (path.length && !e.target.closest(".t-menu") && !e.target.closest(".t-bar-item") && !popup) closeMenus();
}, true);

// keyMenu handles a key while a menu is open or opens one: Alt and a
// mnemonic, F10. It returns whether the key was taken.
function keyMenu(e) {
	if (modalDepth) return false;
	if (!path.length) {
		if (e.key === "F10" && !e.shiftKey && !e.ctrlKey && !e.altKey && bar.menus.length) {
			bar.menus[0].open(); activate(bar.menus[0], 0); return true;
		}
		if (e.altKey && !e.ctrlKey && e.key.length === 1) {
			const k = e.key.toLowerCase();
			const m = bar.menus.find((x) => x.button.mnemonic === k);
			if (m) { m.open(); activate(m, 0); return true; }
		}
		return false;
	}
	const cur = path[path.length - 1];
	const items = [...cur.ul.querySelectorAll(":scope > .t-mi")];
	const act = items.findIndex((li) => li.classList.contains("act"));
	const move = (d) => {
		if (!items.length) return;
		let i = act;
		for (let n = 0; n < items.length; n++) {
			i = (i + d + items.length) % items.length;
			if (!items[i].classList.contains("dis")) break;
		}
		setAct(items, i);
	};
	switch (e.key) {
	case "Escape":
		if (path.length > 1) { path.pop(); cur.ul.parentElement.classList.remove("open"); } else closeMenus();
		return true;
	case "ArrowDown": move(act < 0 ? 0 : 1); return true;
	case "ArrowUp": move(act < 0 ? -1 : -1); return true;
	case "ArrowRight":
		if (act >= 0 && items[act].classList.contains("t-sub")) { openSub(items[act]); return true; }
		if (path[0].isBar) { const i = bar.menus.indexOf(path[0]); const n = bar.menus[(i + 1) % bar.menus.length]; n.open(); activate(n, 0); }
		return true;
	case "ArrowLeft":
		if (path.length > 1) { path.pop(); cur.ul.parentElement.classList.remove("open"); return true; }
		if (path[0].isBar) { const i = bar.menus.indexOf(path[0]); const n = bar.menus[(i - 1 + bar.menus.length) % bar.menus.length]; n.open(); activate(n, 0); }
		return true;
	case "Enter": case " ":
		if (act >= 0) {
			if (items[act].classList.contains("t-sub")) openSub(items[act]); else items[act].click();
		}
		return true;
	}
	if (e.key.length === 1 && !e.ctrlKey && !e.altKey) {
		const k = e.key.toLowerCase();
		const li = items.find((x) => x.mnemonic === k && !x.classList.contains("dis"));
		if (li) { if (li.classList.contains("t-sub")) openSub(li); else li.click(); return true; }
	}
	return true;
}
function setAct(items, i) {
	for (const li of items) li.classList.remove("act");
	if (i >= 0 && items[i]) items[i].classList.add("act");
}
function activate(menu, i) {
	const items = [...menu.ul.querySelectorAll(":scope > .t-mi")];
	setAct(items, items.findIndex((li) => !li.classList.contains("dis")));
}
function openSub(li) {
	li.classList.add("open");
	const sub = li.querySelector(":scope > .t-menu");
	const menu = { ul: sub };
	path.push(menu);
	activate(menu, 0);
}

// ---------------------------------------------------------------- dialogs

// modal puts a dialog box over a backdrop; it returns a function that
// closes it. The dialog's own keys are handled by onKey, and Tab stays
// inside.
function modal(box, onKey) {
	closeMenus();
	modalDepth++;
	const back = h("div", "t-modal");
	back.append(box);
	doc.body.append(back);
	const prev = doc.activeElement;
	const keydown = (e) => {
		if (!back.contains(doc.activeElement) && doc.activeElement !== doc.body) return;
		if (e.key === "Tab") {
			const f = [...box.querySelectorAll("button:not([disabled]), input:not([disabled]), select, textarea")];
			if (!f.length) return;
			const i = f.indexOf(doc.activeElement);
			const n = f[(i + (e.shiftKey ? -1 : 1) + f.length) % f.length];
			e.preventDefault();
			n.focus();
		} else if (onKey) {
			onKey(e);
		}
		e.stopPropagation();
	};
	doc.addEventListener("keydown", keydown, true);
	return () => {
		doc.removeEventListener("keydown", keydown, true);
		back.remove();
		modalDepth--;
		if (prev && prev.focus) prev.focus({ preventScroll: true });
	};
}

function dialogBox(title) {
	const box = h("div", "t-dialog");
	box.setAttribute("role", "dialog");
	box.append(h("div", "t-dlg-title", title));
	return box;
}

const icons = ["ℹ", "?", "⚠", "✖"];

function messageDialog(specJSON, done) {
	const spec = JSON.parse(specJSON);
	const box = dialogBox(spec.title || "Message");
	const body = h("div", "t-dlg-body");
	const icon = h("div", "t-dlg-icon t-icon" + spec.type, icons[spec.type] || "");
	const text = h("div", "t-dlg-text");
	text.append(h("div", spec.description ? "t-dlg-msg strong" : "t-dlg-msg", spec.message));
	if (spec.description) text.append(h("div", "t-dlg-desc", spec.description));
	body.append(icon, text);
	const btns = h("div", "t-dlg-buttons");
	let def = -1;
	const buttons = spec.buttons.map((b, i) => {
		const el = labelSpan(b.label, "");
		const btn = h("button", "t-btn");
		btn.append(el);
		btn.addEventListener("click", () => finish(i));
		btn.mnemonic = el.mnemonic;
		btns.append(btn);
		if (b.default) def = i;
		return btn;
	});
	box.append(body, btns);
	const close = modal(box, (e) => {
		if (e.key === "Enter" && !(doc.activeElement instanceof HTMLButtonElement && buttons.includes(doc.activeElement))) {
			e.preventDefault();
			if (def >= 0) finish(def);
		} else if (e.key === "Escape") {
			e.preventDefault();
			finish(-1);
		} else if (e.altKey && e.key.length === 1) {
			const i = buttons.findIndex((b) => b.mnemonic === e.key.toLowerCase());
			if (i >= 0) { e.preventDefault(); finish(i); }
		}
	});
	let over = false;
	function finish(i) { if (over) return; over = true; close(); done(i); }
	(buttons[def] || buttons[buttons.length - 1] || box).focus?.();
}

function formDialog(specJSON, onChange, done) {
	const spec = JSON.parse(specJSON);
	const box = dialogBox(spec.title || "Options");
	const body = h("div", "t-dlg-form");
	const fields = {};
	for (const f of spec.fields) {
		const row = h("div", "t-row");
		if (f.kind !== "check") row.append(labelSpan(f.label, "t-row-label"));
		if (f.kind === "choice") {
			const g = h("span", "t-choices");
			const name = "choice-" + f.key;
			const radios = f.choices.map((c, i) => {
				const lab = h("label", "t-choice");
				const r = h("input");
				r.type = "radio"; r.name = name; r.value = String(i);
				r.checked = String(i) === f.value;
				r.addEventListener("change", () => onChange(f.key, r.value));
				lab.append(r, " " + c);
				g.append(lab);
				return r;
			});
			row.append(g);
			fields[f.key] = { get: () => (radios.find((r) => r.checked) || { value: f.value }).value, set: (v) => radios.forEach((r) => { r.checked = r.value === v; }) };
		} else if (f.kind === "check") {
			const lab = h("label", "t-check");
			const c = h("input");
			c.type = "checkbox"; c.checked = f.value === "1";
			c.addEventListener("change", () => onChange(f.key, c.checked ? "1" : "0"));
			lab.append(c, " ");
			lab.append(labelSpan(f.label, ""));
			row.append(lab);
			fields[f.key] = { get: () => (c.checked ? "1" : "0"), set: (v) => { c.checked = v === "1"; } };
		} else {
			const t = h("input", "t-entry");
			t.type = "text"; t.value = f.value; t.size = 14; t.spellcheck = false;
			t.addEventListener("input", () => onChange(f.key, t.value));
			row.append(t);
			if (f.hint) row.append(h("span", "t-hint", f.hint));
			fields[f.key] = { get: () => t.value, set: (v) => { t.value = v; } };
		}
		body.append(row);
	}
	const btns = h("div", "t-dlg-buttons");
	const cancel = h("button", "t-btn");
	const accept = h("button", "t-btn default");
	const cl = labelSpan(spec.cancel || "Cancel", ""), al = labelSpan(spec.accept || "OK", "");
	cancel.append(cl); accept.append(al);
	btns.append(cancel, accept);
	box.append(body, btns);
	let over = false;
	const finish = (ok) => {
		if (over) return;
		over = true;
		const values = {};
		for (const k in fields) values[k] = fields[k].get();
		close();
		done(ok ? JSON.stringify(values) : "");
	};
	cancel.addEventListener("click", () => finish(false));
	accept.addEventListener("click", () => finish(true));
	const close = modal(box, (e) => {
		if (e.key === "Enter" && !(doc.activeElement instanceof HTMLButtonElement)) { e.preventDefault(); finish(true); }
		else if (e.key === "Escape") { e.preventDefault(); finish(false); }
		else if (e.altKey && e.key.length === 1) {
			const k = e.key.toLowerCase();
			if (k === cl.mnemonic) { e.preventDefault(); finish(false); }
			else if (k === al.mnemonic) { e.preventDefault(); finish(true); }
		}
	});
	const first = box.querySelector("input.t-entry, input[type=radio]:checked, input");
	(first || accept).focus();
	if (first && first.select) first.select();
	return { set(key, value) { if (fields[key]) fields[key].set(value); } };
}

function openFileDialog(specJSON, done) {
	const spec = JSON.parse(specJSON);
	const box = dialogBox(spec.title || "Open File");
	const body = h("div", "t-dlg-form");
	const input = h("input");
	input.type = "file";
	input.multiple = true;
	input.accept = spec.accept || "";
	const drop = h("div", "t-drop", "Choose a file, or drop it here. Pick the files a model links to together with it.");
	drop.addEventListener("dragover", (e) => { e.preventDefault(); drop.classList.add("over"); });
	drop.addEventListener("dragleave", () => drop.classList.remove("over"));
	drop.addEventListener("drop", (e) => {
		e.preventDefault(); e.stopPropagation();
		drop.classList.remove("over");
		pending = [...e.dataTransfer.files];
		names.textContent = pending.map((f) => f.name).join(", ");
		open.disabled = !pending.length;
	});
	const names = h("div", "t-names");
	body.append(input, drop, names);
	const btns = h("div", "t-dlg-buttons");
	const cancel = h("button", "t-btn", "Cancel");
	const open = h("button", "t-btn default", "Open");
	open.disabled = true;
	btns.append(cancel, open);
	box.append(body, btns);
	let pending = [], over = false;
	input.addEventListener("change", () => {
		pending = [...input.files];
		names.textContent = pending.map((f) => f.name).join(", ");
		open.disabled = !pending.length;
	});
	const finish = async (ok) => {
		if (over) return;
		over = true;
		let paths = [];
		if (ok) paths = await putFiles(pending);
		close();
		done(ok ? JSON.stringify(paths) : "");
	};
	cancel.addEventListener("click", () => finish(false));
	open.addEventListener("click", () => finish(true));
	const close = modal(box, (e) => {
		if (e.key === "Escape") { e.preventDefault(); finish(false); }
		else if (e.key === "Enter" && !open.disabled && doc.activeElement !== cancel) { e.preventDefault(); finish(true); }
	});
	input.focus();
}

function saveFileDialog(specJSON, done) {
	const spec = JSON.parse(specJSON);
	const box = dialogBox(spec.title || "Save File");
	const body = h("div", "t-dlg-form");
	const row = h("div", "t-row");
	const name = h("input", "t-entry wide");
	name.type = "text"; name.value = spec.name; name.spellcheck = false;
	row.append(h("span", "t-row-label", "File name"), name);
	body.append(row);
	let sel = null;
	if (spec.filters.length > 1) {
		sel = h("select");
		spec.filters.forEach((f, i) => { const o = h("option", "", f.name); o.value = i; sel.append(o); });
		sel.value = spec.filter;
		sel.addEventListener("change", () => {
			const ext = spec.filters[sel.value].ext;
			if (ext) name.value = name.value.replace(/\.[^./]*$/, "") + "." + ext;
		});
		const r2 = h("div", "t-row");
		r2.append(h("span", "t-row-label", "File type"), sel);
		body.append(r2);
	}
	body.append(h("div", "t-hint", "The file is kept in the browser, and offered as a download."));
	const btns = h("div", "t-dlg-buttons");
	const cancel = h("button", "t-btn", "Cancel");
	const save = h("button", "t-btn default", "Save");
	btns.append(cancel, save);
	box.append(body, btns);
	let over = false;
	const finish = (ok) => {
		if (over) return;
		over = true;
		const v = name.value.trim();
		close();
		done(ok && v ? JSON.stringify({ name: v, filter: sel ? Number(sel.value) : spec.filter }) : "");
	};
	cancel.addEventListener("click", () => finish(false));
	save.addEventListener("click", () => finish(true));
	const close = modal(box, (e) => {
		if (e.key === "Escape") { e.preventDefault(); finish(false); }
		else if (e.key === "Enter" && doc.activeElement !== cancel) { e.preventDefault(); finish(true); }
	});
	name.focus();
	const dot = name.value.lastIndexOf(".");
	name.setSelectionRange(0, dot > 0 ? dot : name.value.length);
}

// download offers a file of the in-memory file system to the user.
function download(path) {
	const bytes = memfs.get(path);
	if (!bytes) return false;
	const a = h("a");
	a.href = URL.createObjectURL(new Blob([bytes], { type: "application/octet-stream" }));
	a.download = path.split("/").pop();
	doc.body.append(a);
	a.click();
	a.remove();
	setTimeout(() => URL.revokeObjectURL(a.href), 10000);
	(W.downloads = W.downloads || []).push(path);
	toast("Downloaded " + a.download);
	return true;
}

function toast(text) {
	const t = h("div", "t-toast", text);
	doc.body.append(t);
	setTimeout(() => t.remove(), 3500);
}

// showFiles lists the files kept in the browser.
function showFiles() {
	if (modalDepth) return;
	const box = dialogBox("Files in this browser");
	const list = h("div", "t-files");
	const fill = () => {
		list.replaceChildren();
		const names = memfs.list("/home/").sort();
		if (!names.length) list.append(h("div", "t-hint", "No files yet. Save a model, or open one."));
		for (const p of names) {
			const row = h("div", "t-file");
			row.append(h("span", "t-file-name", p.replace("/home/user/", "")), h("span", "t-file-size", (memfs.get(p).length / 1024).toFixed(1) + " KB"));
			const dl = h("button", "t-btn", "Download");
			dl.addEventListener("click", () => download(p));
			const rm = h("button", "t-btn", "Delete");
			rm.addEventListener("click", () => { memfs.remove(p); fill(); });
			row.append(dl, rm);
			list.append(row);
		}
	};
	fill();
	const up = h("input");
	up.type = "file"; up.multiple = true;
	up.addEventListener("change", async () => { await putFiles([...up.files]); fill(); });
	const btns = h("div", "t-dlg-buttons");
	const closeBtn = h("button", "t-btn default", "Close");
	btns.append(closeBtn);
	box.append(list, h("div", "t-dlg-form", ""), btns);
	box.insertBefore(up, btns);
	const close = modal(box, (e) => { if (e.key === "Escape") { e.preventDefault(); close(); } });
	closeBtn.addEventListener("click", () => close());
	closeBtn.focus();
}

// ---------------------------------------------------------------- console

// consoleObject is the window scripts are typed in: what was printed over
// a box where the next lines are typed. cb has the Go functions: enter,
// tab, interrupt, close.
function consoleObject(cb) {
	const box = h("div", "t-console");
	box.style.display = "none";
	const head = h("div", "t-con-head");
	const title = h("span", "t-con-title", "Script Console");
	const status = h("span", "t-con-status", "");
	const stop = h("button", "t-btn small", "Stop");
	const clear = h("button", "t-btn small", "Clear");
	const x = h("button", "t-x", "✕");
	head.append(title, status, stop, clear, x);
	const out = h("pre", "t-con-out");
	const inRow = h("div", "t-con-in");
	const prompt = h("span", "t-con-prompt", ">>> ");
	const input = h("textarea", "t-con-input");
	input.rows = 1; input.spellcheck = false; input.autocomplete = "off";
	input.autocapitalize = "off";
	inRow.append(prompt, input);
	box.append(head, out, inRow);
	consoleSlot.append(box);
	const c = { box, out, input, prompt, status, history: [], hpos: 0, draft: "", busy: false, visible: false };
	const grow = () => { input.rows = Math.min(8, input.value.split("\n").length); };
	input.addEventListener("input", grow);
	input.addEventListener("keydown", (e) => {
		e.stopPropagation();
		const k = e.key;
		if (k === "Enter" && !e.ctrlKey) {
			e.preventDefault();
			if (c.busy) return;
			cb.enter(input.value, e.shiftKey);
		} else if (k === "Tab" && !e.shiftKey) {
			e.preventDefault();
			cb.tab(input.value, input.selectionStart);
		} else if ((k === "Escape") || (k === "c" && e.ctrlKey && input.selectionStart === input.selectionEnd)) {
			e.preventDefault();
			cb.interrupt();
			if (!c.busy) { input.value = ""; grow(); }
		} else if (k === "ArrowUp" && !e.shiftKey && !input.value.slice(0, input.selectionStart).includes("\n")) {
			if (c.hpos > 0) {
				e.preventDefault();
				if (c.hpos === c.history.length) c.draft = input.value;
				input.value = c.history[--c.hpos];
				grow();
			}
		} else if (k === "ArrowDown" && !e.shiftKey && !input.value.slice(input.selectionEnd).includes("\n")) {
			if (c.hpos < c.history.length) {
				e.preventDefault();
				c.hpos++;
				input.value = c.hpos === c.history.length ? c.draft : c.history[c.hpos];
				grow();
			}
		} else if (k === "l" && e.ctrlKey) {
			e.preventDefault();
			out.replaceChildren();
		}
	});
	stop.addEventListener("click", () => cb.interrupt());
	clear.addEventListener("click", () => out.replaceChildren());
	x.addEventListener("click", () => cb.close());
	out.addEventListener("click", () => { if (getSelection().isCollapsed) input.focus(); });
	return Object.assign(c, consoleMethods);
}

const consoleMethods = {
	print(kind, text) {
		const o = this.out;
		const atEnd = o.scrollHeight - o.scrollTop - o.clientHeight < 8;
		const s = h("span", "t-con-k" + kind, text);
		o.append(s);
		// Keep the text from growing for ever.
		while (o.childNodes.length > 5000) o.firstChild.remove();
		if (atEnd) o.scrollTop = o.scrollHeight;
	},
	setPrompt(p) { this.prompt.textContent = p; },
	setBusy(b) {
		this.busy = b;
		this.status.textContent = b ? "running..." : "";
		this.box.classList.toggle("busy", b);
	},
	clear() { this.out.replaceChildren(); },
	setHistory(entries) { this.history = entries.slice(); this.hpos = this.history.length; },
	addHistory(line) { this.history.push(line); this.hpos = this.history.length; },
	setVisible(v) {
		this.visible = v;
		this.box.style.display = v ? "flex" : "none";
		if (v) this.input.focus();
	},
	focus() { this.input.focus(); },
	clearInput() { this.input.value = ""; this.input.rows = 1; },
	insertNewline() {
		const i = this.input;
		i.setRangeText("\n", i.selectionStart, i.selectionEnd, "end");
		i.rows = Math.min(8, i.value.split("\n").length);
	},
	replace(start, end, text) { this.input.setRangeText(text, start, end, "end"); },
};

// ---------------------------------------------------------------- keys

// onKey sends the keys that are not typed into a box to Go. cb(type, key,
// code, shift, ctrl, alt, meta) returns whether the browser is kept from
// acting on the key.
function onKey(cb) {
	const handler = (type) => (e) => {
		if (e.isComposing) return;
		const t = e.target;
		if (t && (t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.tagName === "SELECT" || t.isContentEditable)) return;
		if (modalDepth) return;
		if (type === 0 && keyMenu(e)) { e.preventDefault(); return; }
		if (cb(type, e.key, e.code, e.shiftKey, e.ctrlKey, e.altKey, e.metaKey)) e.preventDefault();
	};
	doc.addEventListener("keydown", handler(0));
	doc.addEventListener("keyup", handler(1));
}

const W = {
	init, mnemonic, onKey, download, toast,
	createWindow: makeWindow,
	menu: () => menuObject(false),
	menuBar: barObject,
	setMenuBar() {},
	console: consoleObject,
	messageDialog, formDialog, openFileDialog, saveFileDialog,
	setTitle(t) { doc.title = t; },
	listenFullscreen(f) { fullscreenListeners.add(f); },
	devicePixelRatio: () => window.devicePixelRatio || 1,
	pageSize: () => [innerWidth, innerHeight],
	exited() {
		const o = h("div", "t-exited");
		o.append(h("div", "", "TeleSAPR has exited."), h("div", "t-hint", "Reload the page to start again. Your files are kept."));
		doc.body.append(o);
	},
	modalDepth: () => modalDepth,
	openMenus: () => path.length,
	onDropFiles: null,
};
return W;
