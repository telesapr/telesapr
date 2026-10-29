// memfs.js: the in-memory file system telesapr-wasm reads and writes in
// a browser. Go's js/wasm port calls globalThis.fs in the shape of
// node's callback API (syscall/fs_js.go); wasm_exec.js puts in a stub
// that refuses every call unless a fs is there before it loads.
// Files are byte arrays by absolute path; directories are implied.
"use strict";
(() => {
	const files = new Map(); // path -> Uint8Array
	const fds = new Map(); // fd -> {path, pos, flags}
	let nextFD = 10;
	const C = { O_RDONLY: 0, O_WRONLY: 1, O_RDWR: 2, O_CREAT: 64, O_EXCL: 128, O_TRUNC: 512, O_APPEND: 1024, O_DIRECTORY: 65536 };
	const err = (code) => { const e = new Error(code); e.code = code; return e; };
	const norm = (p) => (p.startsWith("/") ? p : "/" + p).replace(/\/+/g, "/");
	const isDir = (p) => { const d = p.endsWith("/") ? p : p + "/"; for (const k of files.keys()) if (k.startsWith(d)) return true; return p === "/"; };
	const stat = (p) => {
		const f = files.get(p), dir = !f && isDir(p);
		if (!f && !dir) return null;
		const t = Date.now();
		return { dev: 0, ino: 0, mode: dir ? 0o40755 : 0o100644, nlink: 1, uid: 0, gid: 0, rdev: 0,
			size: f ? f.length : 0, blksize: 4096, blocks: 0, atimeMs: t, mtimeMs: t, ctimeMs: t,
			isDirectory: () => dir };
	};
	const decoder = new TextDecoder("utf-8");
	let line = "";
	const out = (buf) => {
		line += decoder.decode(buf);
		const nl = line.lastIndexOf("\n");
		if (nl >= 0) { console.log(line.substring(0, nl)); line = line.substring(nl + 1); }
		return buf.length;
	};
	globalThis.memfs = {
		put(path, bytes) { files.set(norm(path), bytes); },
		get(path) { return files.get(norm(path)); },
	};
	globalThis.fs = {
		constants: C,
		writeSync(fd, buf) { return out(buf); },
		open(path, flags, mode, cb) {
			path = norm(path);
			if (!files.has(path)) {
				if (!(flags & C.O_CREAT)) { if (isDir(path)) { const fd = nextFD++; fds.set(fd, { path, pos: 0 }); cb(null, fd); return; } cb(err("ENOENT")); return; }
				files.set(path, new Uint8Array(0));
			} else if (flags & C.O_EXCL && flags & C.O_CREAT) { cb(err("EEXIST")); return; }
			if (flags & C.O_TRUNC) files.set(path, new Uint8Array(0));
			const fd = nextFD++;
			fds.set(fd, { path, pos: flags & C.O_APPEND ? files.get(path).length : 0 });
			cb(null, fd);
		},
		close(fd, cb) { fds.delete(fd); cb(null); },
		fstat(fd, cb) { const f = fds.get(fd); const s = f && stat(f.path); s ? cb(null, s) : cb(err("EBADF")); },
		stat(path, cb) { const s = stat(norm(path)); s ? cb(null, s) : cb(err("ENOENT")); },
		lstat(path, cb) { this.stat(path, cb); },
		readdir(path, cb) {
			const d = norm(path).replace(/\/?$/, "/"), names = new Set();
			for (const k of files.keys()) if (k.startsWith(d)) names.add(k.substring(d.length).split("/")[0]);
			cb(null, [...names]);
		},
		read(fd, buffer, offset, length, position, cb) {
			const f = fds.get(fd); if (!f) { cb(err("EBADF")); return; }
			const data = files.get(f.path) || new Uint8Array(0);
			const at = position === null ? f.pos : position;
			const n = Math.max(0, Math.min(length, data.length - at));
			buffer.set(data.subarray(at, at + n), offset);
			if (position === null) f.pos += n;
			cb(null, n);
		},
		write(fd, buf, offset, length, position, cb) {
			if (fd === 1 || fd === 2) { cb(null, out(buf.subarray(offset, offset + length))); return; }
			const f = fds.get(fd); if (!f) { cb(err("EBADF")); return; }
			const at = position === null ? f.pos : position;
			const old = files.get(f.path), end = Math.max(old.length, at + length);
			const data = end > old.length ? new Uint8Array(end) : old;
			if (data !== old) data.set(old);
			data.set(buf.subarray(offset, offset + length), at);
			files.set(f.path, data);
			if (position === null) f.pos += length;
			cb(null, length);
		},
		mkdir(path, perm, cb) { cb(null); },
		fsync(fd, cb) { cb(null); },
		unlink(path, cb) { files.delete(norm(path)) ? cb(null) : cb(err("ENOENT")); },
		rename(from, to, cb) { const d = files.get(norm(from)); if (!d) { cb(err("ENOENT")); return; } files.delete(norm(from)); files.set(norm(to), d); cb(null); },
		ftruncate(fd, length, cb) { const f = fds.get(fd); const d = files.get(f.path); files.set(f.path, d.slice(0, length)); cb(null); },
		truncate(path, length, cb) { const d = files.get(norm(path)); files.set(norm(path), d.slice(0, length)); cb(null); },
		chmod(p, m, cb) { cb(null); }, chown(p, u, g, cb) { cb(null); }, fchmod(fd, m, cb) { cb(null); },
		fchown(fd, u, g, cb) { cb(null); }, lchown(p, u, g, cb) { cb(null); }, utimes(p, a, m, cb) { cb(null); },
		link(p, l, cb) { cb(err("ENOSYS")); }, symlink(p, l, cb) { cb(err("ENOSYS")); },
		readlink(p, cb) { cb(err("EINVAL")); }, rmdir(p, cb) { cb(null); },
	};
	// path.resolve, which syscall.Open calls, is node's; a browser has
	// none, and every path here is absolute already.
	globalThis.path = globalThis.path || { resolve: (p) => norm(p) };
	globalThis.process = globalThis.process || { pid: 1, ppid: 0, cwd: () => "/", chdir() {}, getuid: () => -1, getgid: () => -1, geteuid: () => -1, getegid: () => -1, getgroups() { throw err("ENOSYS"); }, umask() { return 0o22; } };
})();
