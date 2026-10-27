// hospedador con forma de navegador para apkc.wasm: no usa el fs de Node, sino un fs en memoria
// con la misma API (24 funciones) que exige src/syscall/fs_js.go. Si apkc corre aquí, puede correr
// en una pestaña con OPFS detras, sin server y sin SDK.
//
// necesita wasm_exec.js junto a este fichero (no se versiona, es de tu GOROOT):
//   cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" .
// uso: node navegador-sim.js <apkc.wasm> <comando de apkc...>   (ej.: create /miapp)
// ver ../README.md y ../docs/evidencia.md para lo que medido y lo que todavia bloquea.
const fs = require("fs");
// los flujos reales hay que capturarlos antes de sustituir globalThis.process: si no, la
// salida de apkc se escribe contra el process falso y no la ve nadie
const procReal = { stdout: process.stdout, stderr: process.stderr };

// --- el sistema de ficheros en memoria -------------------------------------------------
const raiz = new Map();
const modoDir = 0o040000 | 0o755;
const modoFile = 0o100644;
function nuevoNodo(calle, esDir) {
  const ahora = Date.now();
  const n = {
    dir: esDir, data: esDir ? null : new Uint8Array(0), mode: esDir ? modoDir : modoFile,
    mtimeMs: ahora, atimeMs: ahora, ctimeMs: ahora, uid: 1000, gid: 1000, hijos: esDir ? new Set() : null,
  };
  raiz.set(calle, n);
  return n;
}
nuevoNodo("/", true);
nuevoNodo("/tmp", true);   // testing.T.TempDir lo pide por defecto; sin el, los tests fallan
                          // por el shim y no por el código que se quiere medir

const err = (code, calle) => Object.assign(new Error(code + ": " + code + ", " + calle), { code, path: calle });

function limpiar(calle) {
  let p = calle.replace(/\/+$/, "");
  return p === "" ? "/" : p;
}
function absoluta(calle) {
  if (typeof calle !== "string") return calle;
  const p = calle.startsWith("/") ? calle : limpiar(process.cwd() + "/" + calle);
  // .. y . sin resolver un mapa plano: apkc usa rutas relativas simples
  const tramos = [];
  for (const t of p.split("/")) {
    if (t === "" || t === ".") continue;
    if (t === "..") tramos.pop();
    else tramos.push(t);
  }
  return "/" + tramos.join("/");
}
function padre(calle) {
  const i = calle.lastIndexOf("/");
  return i <= 0 ? "/" : calle.slice(0, i);
}
function existe(calle) { return raiz.has(calle); }

// abrir un camino: los directorios intermedios no se crean solos (igual que node)
function resolverAbierto(calle) {
  const partes = calle.split("/").filter(Boolean);
  let actual = "/";
  for (let i = 0; i < partes.length - 1; i++) {
    actual = actual === "/" ? "/" + partes[i] : actual + "/" + partes[i];
    const n = raiz.get(actual);
    if (!n) return { error: err("ENOENT", calle) };
    if (!n.dir) return { error: err("ENOTDIR", calle) };
  }
  return { ok: true };
}

let proximoFd = 10;
const descriptores = new Map();

// stdin, stdout y stderr viven en el objeto fs, no en el árbol: sin esto la salida de apkc
// se pierde en un EBADF mudo y el experimento parece callar cuando es el shim el que calla
const consola = (flujo) => ({
  nodo: { dir: false, data: new Uint8Array(0), mode: modoFile, mtimeMs: Date.now(), atimeMs: Date.now(), ctimeMs: Date.now(), uid: 1000, gid: 1000 },
  pos: 0,
  calle: flujo === 0 ? "/dev/stdin" : flujo === 1 ? "/dev/stdout" : "/dev/stderr",
  flujo,
});
descriptores.set(0, consola(0));
descriptores.set(1, consola(1));
descriptores.set(2, consola(2));

function statDe(n, calle) {
  return {
    // Go copia estos campos uno a uno (fs_js.go: setStat); falta uno y el runtime entra en
    // pánico dentro de os.MkdirAll, que parece un fallo de apkc y es del shim
    dev: 1, ino: [...raiz.keys()].indexOf(calle) + 2,
    mode: n.mode,
    nlink: n.dir ? 2 : 1,
    uid: n.uid, gid: n.gid,
    rdev: 0,
    size: n.dir ? 4096 : n.data.length,
    blksize: 4096, blocks: 8,
    atimeMs: n.atimeMs, mtimeMs: n.mtimeMs, ctimeMs: n.ctimeMs,
    isDirectory: () => n.dir,
    isFile: () => !n.dir,
    isSymbolicLink: () => false,
  };
}

const N = { // constantes de node/linux, con los valores que Go espera traducir
  O_RDONLY: 0, O_WRONLY: 1, O_RDWR: 2, O_CREAT: 0o100, O_EXCL: 0o200, O_TRUNC: 0o1000,
  O_APPEND: 0o2000, O_SYNC: 0o4010000, O_DIRECTORY: 0o40000, COPYFILE_EXCL: 1,
  S_IFDIR: 0o40000, S_IFREG: 0o100000, S_IFLNK: 0o120000,
};

function llamar(cb) {
  return (errores, valor) => setImmediate(() => (errores ? cb(errores) : cb(null, valor)));
}

const fsSim = {
  constants: N,
  open(calle, flags, modo, cb) {
    const r = cb;
    if (typeof modo === "function") return this.open(calle, flags, 0o666, modo);
    const a = absoluta(String(calle));
    const camino = resolverAbierto(a);
    if (camino.error) return void llamar(r)(camino.error);
    const existente = raiz.get(a);
    if (existente && (flags & N.O_EXCL) && (flags & N.O_CREAT)) return void llamar(r)(err("EEXIST", a));
    if (!existente) {
      if (!(flags & N.O_CREAT)) return void llamar(r)(err("ENOENT", a));
      nuevoNodo(a, false);
    }
    const n = raiz.get(a);
    if (n.dir && (flags & (N.O_WRONLY | N.O_RDWR))) return void llamar(r)(err("EISDIR", a));
    if (!n.dir && (flags & N.O_TRUNC)) n.data = new Uint8Array(0);
    const fd = proximoFd++;
    descriptores.set(fd, { nodo: n, pos: flags & N.O_APPEND ? n.data.length : 0, calle: a });
    llamar(r)(null, fd);
  },
  close(fd, cb) { descriptores.delete(fd); if (cb) cb(null); },
  read(fd, buf, offset, length, pos, cb) {
    const d = descriptores.get(fd);
    if (!d) return void llamar(cb)(err("EBADF", "fd" + fd));
    if (d.flujo === 0) return void llamar(cb)(null, 0);   // stdin vacio: apkc usa el valor por defecto
    const desde = pos === null || pos === undefined ? d.pos : pos;
    const fin = Math.min(d.nodo.data.length, desde + length);
    const n = Math.max(0, fin - desde);
    buf.set(d.nodo.data.subarray(desde, fin), offset);
    if (pos === null || pos === undefined) d.pos = fin;
    llamar(cb)(null, n);
  },
  write(fd, buf, offset, length, pos, cb) {
    const d = descriptores.get(fd);
    if (!d) return void llamar(cb)(err("EBADF", "fd" + fd));
    const trozo = buf.subarray(offset, offset + length);
    if (d.flujo) {                                     // 1 y 2 van a la consola de verdad
      (d.flujo === 1 ? procReal.stdout : procReal.stderr).write(Buffer.from(trozo));
      return void llamar(cb)(null, trozo.length);
    }
    const desde = pos === null || pos === undefined ? d.nodo.data.length : pos;
    const salida = new Uint8Array(Math.max(d.nodo.data.length, desde + trozo.length));
    salida.set(d.nodo.data);
    salida.set(trozo, desde);
    d.nodo.data = salida;
    d.nodo.mtimeMs = Date.now();
    if (pos === null || pos === undefined) d.pos = desde + trozo.length;
    llamar(cb)(null, trozo.length);
  },
  fstat(fd, cb) {
    const d = descriptores.get(fd);
    if (!d) return void llamar(cb)(err("EBADF", "fd" + fd));
    llamar(cb)(null, statDe(d.nodo, d.calle));
  },
  stat(calle, cb) {
    const a = absoluta(String(calle));
    const n = raiz.get(a);
    if (!n) return void llamar(cb)(err("ENOENT", a));
    llamar(cb)(null, statDe(n, a));
  },
  lstat(calle, cb) { return fsSim.stat(calle, cb); },
  readdir(calle, cb) {
    const a = absoluta(String(calle));
    const n = raiz.get(a);
    if (!n) return void llamar(cb)(err("ENOENT", a));
    const nombre = a === "/" ? "" : a.slice(a.lastIndexOf("/") + 1);
    llamar(cb)(null, [...raiz.keys()]
      .filter((k) => k !== a && padre(k) === a)
      .map((k) => k.slice(k.lastIndexOf("/") + 1))
      .concat([]));
    void nombre;
  },
  mkdir(calle, modo, cb) {
    if (typeof modo === "function") return fsSim.mkdir(calle, 0o777, modo);
    const a = absoluta(String(calle));
    if (existe(a)) return void llamar(cb)(err("EEXIST", a));
    const p = padre(a);
    if (!existe(p)) return void llamar(cb)(err("ENOENT", a));
    if (!raiz.get(p).dir) return void llamar(cb)(err("ENOTDIR", a));
    nuevoNodo(a, true);
    if (cb) cb(null);
  },
  rmdir(calle, cb) {
    const a = absoluta(String(calle));
    const n = raiz.get(a);
    if (!n) return void llamar(cb)(err("ENOENT", a));
    if (!n.dir) return void llamar(cb)(err("ENOTDIR", a));
    if ([...raiz.keys()].some((k) => padre(k) === a)) return void llamar(cb)(err("ENOTEMPTY", a));
    raiz.delete(a);
    if (cb) cb(null);
  },
  unlink(calle, cb) {
    const a = absoluta(String(calle));
    if (!raiz.get(a)) return void llamar(cb)(err("ENOENT", a));
    raiz.delete(a);
    if (cb) cb(null);
  },
  rename(a, b, cb) {
    const de = absoluta(String(a)), hasta = absoluta(String(b));
    const n = raiz.get(de);
    if (!n) return void llamar(cb)(err("ENOENT", de));
    // mover el subtree entero: apkc publica un cubo con rename, y un directorio con ficheros
    // dentro que se quede atras es un build a medias
    const hijos = [...raiz.entries()].filter(([k]) => k === de || k.startsWith(de + "/"));
    for (const [k, v] of hijos) { raiz.delete(k); raiz.set(hasta + k.slice(de.length), v); }
    if (cb) cb(null);
  },
  // la capa de syscalls usa las versiones asincronas; el RUNTIME de Go escribe panicos y
  // traces con writeSync, y si no existe el programa muere al intentar avisar de un error
  writeSync(fd, buf, offset = 0, length = buf.length - offset, position = null) {
    const d = descriptores.get(fd);
    if (!d) throw err("EBADF", "fd" + fd);
    const trozo = buf.subarray(offset, offset + length);
    if (d.flujo) {
      (d.flujo === 1 ? procReal.stdout : procReal.stderr).write(Buffer.from(trozo));
      return trozo.length;
    }
    const desde = position === null ? d.nodo.data.length : position;
    const salida = new Uint8Array(Math.max(d.nodo.data.length, desde + trozo.length));
    salida.set(d.nodo.data);
    salida.set(trozo, desde);
    d.nodo.data = salida;
    if (position === null) d.pos = desde + trozo.length;
    return trozo.length;
  },
  ftruncate(fd, largo, cb) {
    const d = descriptores.get(fd);
    if (!d) return void llamar(cb)(err("EBADF", "fd" + fd));
    const s = new Uint8Array(largo); s.set(d.nodo.data.subarray(0, Math.min(largo, d.nodo.data.length)));
    d.nodo.data = s;
    if (cb) cb(null);
  },
  truncate(calle, largo, cb) {
    const n = raiz.get(absoluta(String(calle)));
    if (!n) return void llamar(cb)(err("ENOENT", String(calle)));
    const s = new Uint8Array(largo); s.set(n.data.subarray(0, Math.min(largo, n.data.length)));
    n.data = s;
    if (cb) cb(null);
  },
  utimes(calle, atime, mtime, cb) {
    const n = raiz.get(absoluta(String(calle)));
    if (!n) return void llamar(cb)(err("ENOENT", String(calle)));
    n.atimeMs = Math.round(atime * 1000); n.mtimeMs = Math.round(mtime * 1000);
    if (cb) cb(null);
  },
  chmod(calle, modo, cb) { const n = raiz.get(absoluta(String(calle))); if (n) n.mode = (n.dir ? 0o40000 : 0o100000) | (modo & 0o7777); if (cb) cb(n ? null : err("ENOENT", String(calle))); },
  fchmod(fd, modo, cb) { const d = descriptores.get(fd); if (d) d.nodo.mode = (d.nodo.dir ? 0o40000 : 0o100000) | (modo & 0o7777); if (cb) cb(null); },
  chown(calle, u, g, cb) { const n = raiz.get(absoluta(String(calle))); if (n) { n.uid = u; n.gid = g; } if (cb) cb(n ? null : err("ENOENT", String(calle))); },
  fchown(fd, u, g, cb) { const d = descriptores.get(fd); if (d) { d.nodo.uid = u; d.nodo.gid = g; } if (cb) cb(null); },
  lchown(calle, u, g, cb) { fsSim.chown(calle, u, g, cb); },
  fsync(fd, cb) { if (cb) cb(null); },
  symlink(dest, origen, cb) { const a = absoluta(String(origen)); raiz.set(a, { dir: false, data: new Uint8Array(Buffer.from(String(dest))), mode: 0o120000 | 0o777, mtimeMs: Date.now(), atimeMs: Date.now(), ctimeMs: Date.now(), uid: 1000, gid: 1000 }); if (cb) cb(null); },
  readlink(calle, cb) { const n = raiz.get(absoluta(String(calle))); if (!n) return void llamar(cb)(err("ENOENT", String(calle))); llamar(cb)(null, Buffer.from(n.data).toString()); },
  link(a, b, cb) { const n = raiz.get(absoluta(String(a))); if (n) raiz.set(absoluta(String(b)), n); if (cb) cb(n ? null : err("ENOENT", String(a))); },
  realpath(calle, cb) { const a = absoluta(String(calle)); if (cb) cb(raiz.get(a) ? null : err("ENOENT", a), a); },
};

// --- el resto del entorno de navegador ---------------------------------------------------
let cwd = "/";
const pathSim = {
  resolve: (...partes) => absoluta(partes.length ? partes.join("/") : cwd),
  isAbsolute: (p) => String(p).startsWith("/"),
  dirname: (p) => padre(absoluta(String(p))),
  basename: (p) => { const a = absoluta(String(p)); return a.slice(a.lastIndexOf("/") + 1); },
  sep: "/",
  join: (...p) => absoluta(p.join("/")),
};

// experimento: argv y cwd REALES, antes de sustituir globalThis.process por el falso
const argvReal = process.argv.slice();
const cwdReal = process.cwd();

globalThis.fs = fsSim;
globalThis.path = pathSim;
globalThis.process = {
  cwd: () => cwd,
  chdir: (p) => { const a = absoluta(String(p)); if (!raiz.get(a)) throw err("ENOENT", a); cwd = a; },
  argv: [],
  env: {},
  getuid: () => 1000, getgid: () => 1000, geteuid: () => 1000, getegid: () => 1000,
  getgroups: () => [], umask: () => 0o22,
  platform: "browser", arch: "wasm",
  on: () => {}, exit: (c) => { console.log("apkc salio con", c); },
};
globalThis.TextEncoder = TextEncoder;
globalThis.TextDecoder = TextDecoder;
globalThis.performance = performance;
globalThis.crypto = crypto;
globalThis.require = require;                    // la sigue necesitando wasm_exec.js
globalThis.__fsSim = () => raiz;                 // para inspeccionar el resultado desde fuera

require("./wasm_exec.js");
const go = new Go();
go.argv = ["apkc", ...argvReal.slice(3)];
go.env = { HOME: "/home/usuario", ANDROID_HOME: "/android", JAVA_HOME: "/jvm" };
go.exit = (c) => { console.log("(apkc devolvio", c, ")"); };
WebAssembly.instantiate(fs.readFileSync(argvReal[2]), go.importObject)
  .then((r) => go.run(r.instance))
  .catch((e) => console.log("FALLO al instanciar:", e.message));

setTimeout(() => {
  const tree = [...raiz.keys()].filter((k) => k !== "/").sort();
  console.log("--- ficheros creados en la memoria simulada:", tree.length);
  for (const k of tree.slice(0, 25)) console.log("   ", k, raiz.get(k).dir ? "<dir>" : raiz.get(k).data.length + " bytes");
}, 4000);
