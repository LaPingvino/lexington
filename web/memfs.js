// memfs.js: an in-memory file system for Go programs compiled to
// WebAssembly (GOOS=js), loaded before wasm_exec.js. Go's os package
// calls a Node.js-style globalThis.fs; in a browser there is none, so
// this provides the calls Go makes (syscall/fs_js.go) on files kept in
// memory. Lexington's readers and writers work on file paths, and
// run unchanged on top of it.
(() => {
	if (globalThis.fs && globalThis.fs.__memfs) return;
	const S_IFDIR = 0o040000, S_IFREG = 0o100000;
	const C = { O_RDONLY: 0, O_WRONLY: 1, O_RDWR: 2, O_CREAT: 64, O_EXCL: 128, O_TRUNC: 512, O_APPEND: 1024, O_DIRECTORY: 65536 };
	const nodes = new Map(); // path -> {dir, data: Uint8Array, mtime}
	const fds = new Map(); // fd -> {path, pos, flags}
	let nextFd = 100, ino = 1;
	const now = () => Date.now();
	nodes.set("/", { dir: true, mtime: now(), ino: ino++ });

	const err = (code) => { const e = new Error(code); e.code = code; return e; };
	const norm = (p) => {
		const out = [];
		for (const part of String(p).split("/")) {
			if (part === "" || part === ".") continue;
			if (part === "..") out.pop(); else out.push(part);
		}
		return "/" + out.join("/");
	};
	const parent = (p) => norm(p.replace(/\/[^/]*$/, "") || "/");
	const stat = (n) => ({
		dev: 1, ino: n.ino, mode: (n.dir ? S_IFDIR | 0o755 : S_IFREG | 0o644), nlink: 1, uid: 0, gid: 0, rdev: 0,
		size: n.dir ? 0 : n.data.length, blksize: 4096, blocks: n.dir ? 0 : Math.ceil(n.data.length / 512),
		atimeMs: n.mtime, mtimeMs: n.mtime, ctimeMs: n.mtime,
		isDirectory: () => !!n.dir,
	});
	const grow = (n, size) => {
		if (n.data.length >= size) return;
		const d = new Uint8Array(Math.max(size, n.data.length * 2));
		d.set(n.data); n.data = d.subarray(0, size);
	};
	let outputBuf = "";
	const decoder = new TextDecoder("utf-8");

	const fs = {
		__memfs: true,
		constants: C,
		// standard output and error go to the console (as wasm_exec.js does)
		writeSync(fd, buf) {
			if (fd === 1 || fd === 2) {
				outputBuf += decoder.decode(buf);
				const nl = outputBuf.lastIndexOf("\n");
				if (nl !== -1) { console.log(outputBuf.substring(0, nl)); outputBuf = outputBuf.substring(nl + 1); }
				return buf.length;
			}
			const f = fds.get(fd); if (!f) throw err("EBADF");
			const n = nodes.get(f.path);
			const pos = (f.flags & C.O_APPEND) ? n.data.length : f.pos;
			grow(n, pos + buf.length); n.data.set(buf, pos); n.mtime = now();
			f.pos = pos + buf.length;
			return buf.length;
		},
		write(fd, buf, offset, length, position, cb) {
			try {
				const chunk = buf.subarray(offset, offset + length);
				if (position !== null && position !== undefined && fd > 2) {
					const f = fds.get(fd); if (!f) throw err("EBADF");
					const n = nodes.get(f.path); grow(n, position + length); n.data.set(chunk, position); n.mtime = now();
					cb(null, length); return;
				}
				cb(null, fs.writeSync(fd, chunk));
			} catch (e) { cb(e); }
		},
		read(fd, buffer, offset, length, position, cb) {
			const f = fds.get(fd); if (!f) return cb(err("EBADF"));
			const n = nodes.get(f.path); if (n.dir) return cb(err("EISDIR"));
			const pos = (position !== null && position !== undefined) ? position : f.pos;
			const chunk = n.data.subarray(pos, Math.min(pos + length, n.data.length));
			buffer.set(chunk, offset);
			if (position === null || position === undefined) f.pos += chunk.length;
			cb(null, chunk.length);
		},
		open(path, flags, mode, cb) {
			const p = norm(path); let n = nodes.get(p);
			if (!n) {
				if (!(flags & C.O_CREAT)) return cb(err("ENOENT"));
				const dir = nodes.get(parent(p)); if (!dir || !dir.dir) return cb(err("ENOENT"));
				n = { dir: false, data: new Uint8Array(0), mtime: now(), ino: ino++ }; nodes.set(p, n);
			} else if ((flags & C.O_CREAT) && (flags & C.O_EXCL)) return cb(err("EEXIST"));
			if (!n.dir && (flags & C.O_TRUNC)) n.data = new Uint8Array(0);
			const fd = nextFd++; fds.set(fd, { path: p, pos: 0, flags }); cb(null, fd);
		},
		close(fd, cb) { fds.delete(fd); cb(null); },
		fstat(fd, cb) { const f = fds.get(fd); if (!f) return cb(err("EBADF")); cb(null, stat(nodes.get(f.path))); },
		stat(path, cb) { const n = nodes.get(norm(path)); n ? cb(null, stat(n)) : cb(err("ENOENT")); },
		lstat(path, cb) { fs.stat(path, cb); },
		mkdir(path, perm, cb) {
			const p = norm(path);
			if (nodes.has(p)) return cb(err("EEXIST"));
			const dir = nodes.get(parent(p)); if (!dir || !dir.dir) return cb(err("ENOENT"));
			nodes.set(p, { dir: true, mtime: now(), ino: ino++ }); cb(null);
		},
		readdir(path, cb) {
			const p = norm(path); const n = nodes.get(p);
			if (!n) return cb(err("ENOENT")); if (!n.dir) return cb(err("ENOTDIR"));
			const prefix = p === "/" ? "/" : p + "/";
			const names = [...nodes.keys()].filter((k) => k !== p && k.startsWith(prefix) && !k.slice(prefix.length).includes("/"))
				.map((k) => k.slice(prefix.length));
			cb(null, names);
		},
		unlink(path, cb) {
			const p = norm(path); const n = nodes.get(p);
			if (!n) return cb(err("ENOENT")); if (n.dir) return cb(err("EISDIR"));
			nodes.delete(p); cb(null);
		},
		rmdir(path, cb) {
			const p = norm(path); const n = nodes.get(p);
			if (!n) return cb(err("ENOENT")); if (!n.dir) return cb(err("ENOTDIR"));
			const prefix = p + "/";
			if ([...nodes.keys()].some((k) => k.startsWith(prefix))) return cb(err("ENOTEMPTY"));
			nodes.delete(p); cb(null);
		},
		rename(from, to, cb) {
			const a = norm(from), b = norm(to); const n = nodes.get(a); if (!n) return cb(err("ENOENT"));
			for (const k of [...nodes.keys()]) {
				if (k === a || k.startsWith(a + "/")) { nodes.set(b + k.slice(a.length), nodes.get(k)); nodes.delete(k); }
			}
			cb(null);
		},
		ftruncate(fd, length, cb) {
			const f = fds.get(fd); if (!f) return cb(err("EBADF"));
			const n = nodes.get(f.path); grow(n, length); n.data = n.data.subarray(0, length); cb(null);
		},
		truncate(path, length, cb) {
			const n = nodes.get(norm(path)); if (!n) return cb(err("ENOENT"));
			grow(n, length); n.data = n.data.subarray(0, length); cb(null);
		},
		fsync(fd, cb) { cb(null); },
		chmod(path, mode, cb) { cb(null); }, fchmod(fd, mode, cb) { cb(null); },
		chown(path, uid, gid, cb) { cb(null); }, fchown(fd, uid, gid, cb) { cb(null); }, lchown(path, uid, gid, cb) { cb(null); },
		utimes(path, atime, mtime, cb) { const n = nodes.get(norm(path)); if (n) n.mtime = mtime * 1000; cb(null); },
		link(path, link, cb) { cb(err("ENOSYS")); }, symlink(path, link, cb) { cb(err("ENOSYS")); },
		readlink(path, cb) { cb(err("EINVAL")); },
		// for the page: put files in and take them out
		writeFile(path, bytes) {
			const p = norm(path); const parts = p.split("/").filter(Boolean); let cur = "";
			for (const part of parts.slice(0, -1)) { cur += "/" + part; if (!nodes.has(cur)) nodes.set(cur, { dir: true, mtime: now(), ino: ino++ }); }
			nodes.set(p, { dir: false, data: new Uint8Array(bytes), mtime: now(), ino: ino++ });
		},
	};
	fs.mkdir("/tmp", 0o755, () => {});
	globalThis.fs = fs;
	if (!globalThis.process || !globalThis.process.versions || !globalThis.process.versions.node) {
		globalThis.process = Object.assign(globalThis.process || {}, {
			getuid: () => -1, getgid: () => -1, geteuid: () => -1, getegid: () => -1, getgroups: () => [],
			pid: -1, ppid: -1, umask: () => 0o022, cwd: () => "/", chdir: () => {},
		});
	}
})();
