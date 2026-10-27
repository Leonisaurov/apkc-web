// El fs que exige src/syscall/fs_js.go, con la forma de Node y los bytes detrás de OPFS.
//
// Por qué OPFS y no memoria: en memoria todo lo que escribe apkc se pierde al cerrar la
// pestaña, y un proyecto de un par de docenas de ficheros no puede editarse entre sesiones.
// OPFS es el único sitio del navegador con acceso aleatorio síncrono desde un worker, que
// es justo lo que Go pide (read/write con posición explícita).
//
// Las 24 funciones no se inventaron: son las que llama fs_js.go, y el mismo hospedador con
// forma de navegador del repo (../shim/navegador-sim.js) las pasó 98 veces. Aquí cambia el
// respaldo, no la API. Cualquier omisión la detecta la suite de apkc corriendo en la pestaña,
// que es la prueba que la conduce.
//
// La memoria manda y OPFS es el espejo: cada nodo tiene su acceso sincrónico si vive en el
// árbol, y close/fsync lo vuelven visible. Los flujos 0,1,2 no se espejan: son la consola.

const MODO_DIR = 0o040000 | 0o755;
const MODO_FICHERO = 0o100644;

// constantes de Node/Linux, con los valores que Go espera traducir
export const N = {
  O_RDONLY: 0, O_WRONLY: 1, O_RDWR: 2, O_CREAT: 0o100, O_EXCL: 0o200, O_TRUNC: 0o1000,
  O_APPEND: 0o2000, O_SYNC: 0o4010000, O_DIRECTORY: 0o40000, COPYFILE_EXCL: 1,
  S_IFDIR: 0o40000, S_IFREG: 0o100000, S_IFLNK: 0o120000,
};

export function crearFs(carpeta) {
  const raiz = new Map();
  const descriptores = new Map();
  let proximoFd = 10;
  let salidaConsola = "";
  const consola = {
    escribir: (trozo) => { salidaConsola += trozo; },
    volcar: () => { const s = salidaConsola; salidaConsola = ""; return s; },
  };

  function nuevoNodo(calle, esDir) {
    const ahora = Date.now();
    const n = {
      dir: esDir, data: esDir ? null : new Uint8Array(0), mode: esDir ? MODO_DIR : MODO_FICHERO,
      mtimeMs: ahora, atimeMs: ahora, ctimeMs: ahora, uid: 1000, gid: 1000, acceso: null,
    };
    raiz.set(calle, n);
    return n;
  }
  nuevoNodo("/", true);
  nuevoNodo("/tmp", true);
  // /dev/null lo pide os/exec (y apkc build, al enrutar la salida de una herramienta). Sin
  // nodo, el error que sale es "open /dev/null: No such file or directory" y tapa el muro de
  // verdad (que js/wasm no puede crear tuberías). Los tres flujos, por la misma razón.
  for (const nombre of ["/dev", "/dev/null", "/dev/stdin", "/dev/stdout", "/dev/stderr"]) {
    nuevoNodo(nombre, nombre === "/dev");
  }          // testing.T.TempDir lo pide; sin él fallan los tests por el hospedador

  const err = (code, calle) => Object.assign(new Error(code + ": " + code + ", " + calle), { code, path: calle });
  const limpiar = (calle) => { const p = calle.replace(/\/+$/, ""); return p === "" ? "/" : p; };

  let cwd = "/";
  function absoluta(calle) {
    if (typeof calle !== "string") return calle;
    const p = calle.startsWith("/") ? calle : limpiar(cwd + "/" + calle);
    const tramos = [];
    for (const t of p.split("/")) {
      if (t === "" || t === ".") continue;
      if (t === "..") tramos.pop();
      else tramos.push(t);
    }
    return "/" + tramos.join("/");
  }
  const padre = (calle) => { const i = calle.lastIndexOf("/"); return i <= 0 ? "/" : calle.slice(0, i); };
  const tramosDe = (a) => a.split("/").filter(Boolean);

  // --- OPFS ---------------------------------------------------------------------------
  async function directorio(tramos, crear) {
    let d = carpeta;
    for (const t of tramos) d = await d.getDirectoryHandle(t, { create: !!crear });
    return d;
  }
  async function ficheroOPFS(a, crear) {
    const tramos = tramosDe(a);
    const nombre = tramos.pop();
    try {
      const d = await directorio(tramos, crear);
      return await d.getFileHandle(nombre, { create: !!crear });
    } catch (e) {
      if (e && (e.name === "NotFoundError" || e.name === "NotAllowedError" || e.name === "TypeMismatchError")) return null;
      throw e;
    }
  }
  async function borrarOPFS(a) {
    const tramos = tramosDe(a);
    const nombre = tramos.pop();
    try { (await directorio(tramos, false)).removeEntry(nombre); } catch (e) { /* no estaba: nada que borrar */ }
  }

  // un acceso sincrónico por fichero abierto: es lo que permite escribir en una posición
  // concreta, que es lo que Go hace (read/write con pos explícita)
  async function asegurarAcceso(n, a, paraEscribir) {
    if (n.acceso || n.dir) return;
    const fh = await ficheroOPFS(a, paraEscribir);
    if (!fh) return;
    // createSyncAccessHandle() es ASÍNCRONA (lo que es sincrónico son sus métodos:
    // read, write, getSize, flush). Guardar la promesa sin aguardarla dejaba el acceso
    // inservible: las escrituras se tragaban en el try/catch y el fichero nunca se espejaba
    try { n.acceso = await fh.createSyncAccessHandle(); } catch (e) { n.acceso = null; }
    if (!n.acceso) return;
    if (n.sinCargar !== true) {
      const largo = n.acceso.getSize();
      if (largo > 0) {
        const buf = new Uint8Array(largo);
        n.acceso.read(buf, { at: 0 });
        n.data = buf;
      }
    }
  }

  function escribirEspejo(n, trozo, desde) {
    if (!n.acceso) return;
    try { n.acceso.write(trozo, { at: desde }); } catch (e) { /* la memoria manda; close reintenta */ }
  }

  // los flujos reales viven fuera del árbol: sin esto la salida de apkc se pierde en un
  // EBADF mudo y parece que el programa calla cuando es el hospedador el que calla
  const flujo = (n) => ({
    dir: false, data: new Uint8Array(0), mode: MODO_FICHERO, mtimeMs: Date.now(),
    atimeMs: Date.now(), ctimeMs: Date.now(), uid: 1000, gid: 1000, acceso: null, flujo: n,
  });
  // el descriptor lleva el número de flujo EN LA CASILLA QUE MIRAN write y writeSync: sin él,
  // la salida de apkc se acumulaba en un nodo suelto y la consola de la página no veía nada
  descriptores.set(0, { nodo: flujo(0), pos: 0, calle: "/dev/stdin", flujo: 0 });
  descriptores.set(1, { nodo: flujo(1), pos: 0, calle: "/dev/stdout", flujo: 1 });
  descriptores.set(2, { nodo: flujo(2), pos: 0, calle: "/dev/stderr", flujo: 2 });

  function statDe(n, calle) {
    return {
      // Go copia estos doce campos uno a uno (fs_js.go: setStat); falta uno y el runtime
      // entra en pánico dentro de os.MkdirAll, que parece un fallo de apkc y es del hospedador
      dev: 1, ino: [...raiz.keys()].indexOf(calle) + 2, mode: n.mode, nlink: n.dir ? 2 : 1,
      uid: n.uid, gid: n.gid, rdev: 0,
      size: n.dir ? 4096 : n.data.length, blksize: 4096, blocks: 8,
      atimeMs: n.atimeMs, mtimeMs: n.mtimeMs, ctimeMs: n.ctimeMs,
      isDirectory: () => n.dir, isFile: () => !n.dir, isSymbolicLink: () => false,
    };
  }

  const llamar = (cb) => (error, valor) => {
    if (typeof cb !== "function") { if (error) throw error; return; }
    if (typeof setImmediate === "function") setImmediate(() => (error ? cb(error) : cb(null, valor)));
    else Promise.resolve().then(() => (error ? cb(error) : cb(null, valor)));
  };

  function caminoAbierto(a) {
    const partes = tramosDe(a);
    let actual = "/";
    for (let i = 0; i < partes.length - 1; i++) {
      actual = actual === "/" ? "/" + partes[i] : actual + "/" + partes[i];
      const n = raiz.get(actual);
      if (!n) return { error: err("ENOENT", a) };
      if (!n.dir) return { error: err("ENOTDIR", a) };
    }
    return { ok: true };
  }

  // --- las 24 funciones que llama fs_js.go --------------------------------------------
  const fsSim = {
    constants: N,

    async open(calle, flags, modo, cb) {
      if (typeof modo === "function") return fsSim.open(calle, flags, 0o666, modo);
      const r = llamar(cb);
      try {
        const a = absoluta(String(calle));
        const mal = caminoAbierto(a);
        if (mal.error) return void r(mal.error);
        let n = raiz.get(a);
        if (n && (flags & N.O_EXCL) && (flags & N.O_CREAT)) return void r(err("EEXIST", a));
        if (!n) {
          if (!(flags & N.O_CREAT)) {
            // puede estar en OPFS y no en la memoria (otra sesión, u otra pestaña)
            const dh = await directorio(tramosDe(a).slice(0, -1), false);
            let existeEnDisco = false;
            try { await dh.getFileHandle(tramosDe(a).pop()); existeEnDisco = true; } catch (e) { /* no está */ }
            if (!existeEnDisco) return void r(err("ENOENT", a));
            n = nuevoNodo(a, false);
          } else {
            n = nuevoNodo(a, false);
            n.sinCargar = true;
          }
        }
        if (n.dir && (flags & (N.O_WRONLY | N.O_RDWR))) return void r(err("EISDIR", a));
        if (!n.sinCargar) await asegurarAcceso(n, a, !!(flags & (N.O_WRONLY | N.O_RDWR | N.O_APPEND | N.O_TRUNC)));
        if (!n.dir && (flags & N.O_TRUNC)) {
          n.data = new Uint8Array(0);
          if (n.acceso) { try { n.acceso.truncate(0); } catch (e) { /* en memoria ya está truncado */ } }
        }
        const fd = proximoFd++;
        descriptores.set(fd, { nodo: n, pos: (flags & N.O_APPEND) ? n.data.length : 0, calle: a, flujo: n.flujo });
        r(null, fd);
      } catch (e) { r(e); }
    },

    close(fd, cb) {
      const r = llamar(cb);
      const d = descriptores.get(fd);
      descriptores.delete(fd);
      if (!d || !d.nodo || d.nodo.flujo) return void r(null);
      const n = d.nodo, a = d.calle;
      (async () => {
        if (!n.acceso) {                       // nunca llegó a tener espejo: ahora o nunca
          await asegurarAcceso(n, a, true);
        }
        if (n.acceso) {
          try { n.acceso.write(n.data, { at: 0 }); n.acceso.truncate(n.data.length); n.acceso.flush(); } catch (e) { /* sin espejo */ }
          try { n.acceso.close(); } catch (e) { /* ya cerrado */ }
          n.acceso = null;
        }
        r(null);
      })();
    },

    read(fd, buf, offset, length, pos, cb) {
      const d = descriptores.get(fd);
      if (!d) return void llamar(cb)(err("EBADF", "fd" + fd));
      if (d.flujo === 0) return void llamar(cb)(null, 0);
      const desde = (pos === null || pos === undefined) ? d.pos : pos;
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
      if (d.flujo) {
        (d.flujo === 1 ? consola.escribir : consola.escribir)(new TextDecoder().decode(trozo));
        return void llamar(cb)(null, trozo.length);
      }
      const desde = (pos === null || pos === undefined) ? d.nodo.data.length : pos;
      const salida = new Uint8Array(Math.max(d.nodo.data.length, desde + trozo.length));
      salida.set(d.nodo.data);
      salida.set(trozo, desde);
      d.nodo.data = salida;
      d.nodo.mtimeMs = Date.now();
      if (pos === null || pos === undefined) d.pos = desde + trozo.length;
      llamar(cb)(null, trozo.length);
    },

    // el RUNTIME de Go escribe panics y traces con la versión síncrona: si no existe, el
    // programa muere al intentar avisar de su propio error (wasm_exec.js:255)
    writeSync(fd, buf, offset = 0, length = buf.length - offset, position = null) {
      const d = descriptores.get(fd);
      if (!d) throw err("EBADF", "fd" + fd);
      const trozo = buf.subarray(offset, offset + length);
      if (d.flujo) { consola.escribir(new TextDecoder().decode(trozo)); return trozo.length; }
      const desde = position === null ? d.nodo.data.length : position;
      const salida = new Uint8Array(Math.max(d.nodo.data.length, desde + trozo.length));
      salida.set(d.nodo.data);
      salida.set(trozo, desde);
      d.nodo.data = salida;
      if (position === null) d.pos = desde + trozo.length;
      return trozo.length;
    },

    fstat(fd, cb) {
      const d = descriptores.get(fd);
      if (!d) return void llamar(cb)(err("EBADF", "fd" + fd));
      llamar(cb)(null, statDe(d.nodo, d.calle));
    },

    async stat(calle, cb) {
      const a = absoluta(String(calle));
      const n = raiz.get(a);
      if (n) return void llamar(cb)(null, statDe(n, a));
      // sin nodo en memoria: si está en OPFS existe, y los tests de apkc lo piden
      try {
        const tramos = tramosDe(a);
        if (await ficheroOPFS(a, false)) return void llamar(cb)(null, statDe({ dir: false, data: new Uint8Array(0), mode: MODO_FICHERO, mtimeMs: Date.now(), atimeMs: Date.now(), ctimeMs: Date.now(), uid: 1000, gid: 1000 }, a));
        await directorio(tramos, false);
        return void llamar(cb)(null, statDe({ dir: true, mode: MODO_DIR, mtimeMs: Date.now(), atimeMs: Date.now(), ctimeMs: Date.now(), uid: 1000, gid: 1000 }, a));
      } catch (e) { llamar(cb)(err("ENOENT", a)); }
    },

    lstat(calle, cb) { return fsSim.stat(calle, cb); },

    async readdir(calle, cb) {
      const a = absoluta(String(calle));
      if (!raiz.get(a)) return void llamar(cb)(err("ENOENT", a));
      const nombres = new Set([...raiz.keys()].filter((k) => k !== a && padre(k) === a).map((k) => k.slice(k.lastIndexOf("/") + 1)));
      try {
        const d = await directorio(tramosDe(a), false);
        // entries() rinde [nombre, mango], NO [nombre, "directory"]: el tipo vive en mango.kind.
        // Comparar contra una cadena trataba todo de fichero, y un directorio convertido en
        // fichero de 0 bytes es un chdir que dice "Not a directory" (así se perdió el proyecto
        // al recargar).
        for await (const [nombre, mango] of d.entries()) {
          if (!nombres.has(nombre)) {
            const hijo = a === "/" ? "/" + nombre : a + "/" + nombre;
            if (!raiz.get(hijo)) nuevoNodo(hijo, mango.kind === "directory");
            nombres.add(nombre);
          }
        }
      } catch (e) { /* el directorio no está en el disco: lo que hay es la memoria */ }
      llamar(cb)(null, [...nombres]);
    },

    async mkdir(calle, modo, cb) {
      if (typeof modo === "function") return fsSim.mkdir(calle, 0o777, modo);
      const a = absoluta(String(calle));
      if (raiz.get(a)) return void llamar(cb)(err("EEXIST", a));
      const p = padre(a);
      if (!raiz.get(p)) return void llamar(cb)(err("ENOENT", a));
      if (!raiz.get(p).dir) return void llamar(cb)(err("ENOTDIR", a));
      try { await directorio(tramosDe(a), true); } catch (e) { return void llamar(cb)(e); }
      nuevoNodo(a, true);
      llamar(cb)(null);
    },

    async rmdir(calle, cb) {
      const a = absoluta(String(calle));
      const n = raiz.get(a);
      if (!n) return void llamar(cb)(err("ENOENT", a));
      if (!n.dir) return void llamar(cb)(err("ENOTDIR", a));
      if ([...raiz.keys()].some((k) => padre(k) === a)) return void llamar(cb)(err("ENOTEMPTY", a));
      raiz.delete(a);
      await borrarOPFS(a + "/");
      llamar(cb)(null);
    },

    async unlink(calle, cb) {
      const a = absoluta(String(calle));
      if (!raiz.get(a)) return void llamar(cb)(err("ENOENT", a));
      raiz.delete(a);
      await borrarOPFS(a);
      llamar(cb)(null);
    },

    async rename(de0, hasta0, cb) {
      const de = absoluta(String(de0)), hasta = absoluta(String(hasta0));
      const n = raiz.get(de);
      if (!n) return void llamar(cb)(err("ENOENT", de));
      // mover el subtree entero: apkc publica un cubo con rename, y dejar ficheros detrás es
      // un build a medias
      const hijos = [...raiz.entries()].filter(([k]) => k === de || k.startsWith(de + "/"));
      for (const [k, v] of hijos) {
        raiz.delete(k);
        const nuevo = hasta + k.slice(de.length);
        raiz.set(nuevo, v);
        v.calle = nuevo;
        if (!v.dir) {
          // el contenido ya está escrito en el sitio viejo; el nombre nuevo lo abre otra
          // sesión, así que se espeja aqui y se borra el anterior
          try {
            const fh = await ficheroOPFS(nuevo, true);
            const sh = await fh.createSyncAccessHandle();
            sh.write(v.data, { at: 0 }); sh.truncate(v.data.length); sh.flush(); sh.close();
          } catch (e) { /* se perderá el espejo de este fichero, no la operación */ }
          v.acceso = null;
          await borrarOPFS(k);
        }
      }
      if (n.dir) await borrarOPFS(de + "/");
      llamar(cb)(null);
    },

    ftruncate(fd, largo, cb) {
      const d = descriptores.get(fd);
      if (!d) return void llamar(cb)(err("EBADF", "fd" + fd));
      const s = new Uint8Array(largo);
      s.set(d.nodo.data.subarray(0, Math.min(largo, d.nodo.data.length)));
      d.nodo.data = s;
      if (d.nodo.acceso) { try { d.nodo.acceso.truncate(largo); } catch (e) { /* en memoria ya está */ } }
      llamar(cb)(null);
    },

    truncate(calle, largo, cb) {
      const n = raiz.get(absoluta(String(calle)));
      if (!n) return void llamar(cb)(err("ENOENT", String(calle)));
      const s = new Uint8Array(largo);
      s.set(n.data.subarray(0, Math.min(largo, n.data.length)));
      n.data = s;
      llamar(cb)(null);
    },

    utimes(calle, atime, mtime, cb) {
      const n = raiz.get(absoluta(String(calle)));
      if (!n) return void llamar(cb)(err("ENOENT", String(calle)));
      n.atimeMs = Math.round(atime * 1000); n.mtimeMs = Math.round(mtime * 1000);
      llamar(cb)(null);
    },

    chmod(calle, modo, cb) {
      const n = raiz.get(absoluta(String(calle)));
      if (n) n.mode = (n.dir ? MODO_DIR & 0o170000 : MODO_FICHERO & 0o170000) | (modo & 0o7777);
      llamar(cb)(n ? null : err("ENOENT", String(calle)));
    },
    fchmod(fd, modo, cb) {
      const d = descriptores.get(fd);
      if (d) d.nodo.mode = (d.nodo.dir ? 0o040000 : 0o100000) | (modo & 0o7777);
      llamar(cb)(null);
    },
    chown(calle, u, g, cb) {
      const n = raiz.get(absoluta(String(calle)));
      if (n) { n.uid = u; n.gid = g; }
      llamar(cb)(n ? null : err("ENOENT", String(calle)));
    },
    fchown(fd, u, g, cb) {
      const d = descriptores.get(fd);
      if (d) { d.nodo.uid = u; d.nodo.gid = g; }
      llamar(cb)(null);
    },
    lchown(calle, u, g, cb) { fsSim.chown(calle, u, g, cb); },
    fsync(fd, cb) {
      const d = descriptores.get(fd);
      if (!d) return void llamar(cb)(err("EBADF", "fd" + fd));
      if (d.nodo.acceso) { try { d.nodo.acceso.flush(); } catch (e) { /* la memoria sigue siendo la verdad */ } }
      llamar(cb)(null);
    },
    symlink(dest, origen, cb) {
      const a = absoluta(String(origen));
      raiz.set(a, { dir: false, data: new Uint8Array(new TextEncoder().encode(String(dest))), mode: 0o120000 | 0o777, mtimeMs: Date.now(), atimeMs: Date.now(), ctimeMs: Date.now(), uid: 1000, gid: 1000, acceso: null });
      llamar(cb)(null);
    },
    readlink(calle, cb) {
      const n = raiz.get(absoluta(String(calle)));
      if (!n) return void llamar(cb)(err("ENOENT", String(calle)));
      llamar(cb)(null, new TextDecoder().decode(n.data));
    },
    link(a0, b0, cb) {
      const n = raiz.get(absoluta(String(a0)));
      if (n) raiz.set(absoluta(String(b0)), n);
      llamar(cb)(n ? null : err("ENOENT", String(a0)));
    },
    realpath(calle, cb) {
      const a = absoluta(String(calle));
      llamar(cb)(raiz.get(a) ? null : err("ENOENT", a), a);
    },
  };

  // --- lo que añade el hospedador sobre la API de Node --------------------------------
  const extras = {
    cwd: () => cwd,
    chdir: (p) => {
      const a = absoluta(String(p));
      if (!raiz.get(a)) throw err("ENOENT", a);
      cwd = a;
    },
    // meter en el árbol un fichero que viene de fuera (el apk a firmar, el fuente editado)
    async montar(rutaSimulada, bytes) {
      const a = absoluta(String(rutaSimulada));
      await directorio(tramosDe(a).slice(0, -1), true);
      const n = nuevoNodo(a, false);
      n.sinCargar = true;
      n.data = bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes);
      const fh = await ficheroOPFS(a, true);
      const sh = await fh.createSyncAccessHandle();
      sh.write(n.data, { at: 0 }); sh.truncate(n.data.length); sh.flush(); sh.close();
      return n.data.length;
    },
    // sacar del árbol (el apk firmado, para descargarlo)
    async extraer(rutaSimulada) {
      const a = absoluta(String(rutaSimulada));
      const n = raiz.get(a);
      if (n && !n.dir && !n.descubierto) return n.data;
      const fh = await ficheroOPFS(a, false);
      if (!fh) return null;
      const sh = await fh.createSyncAccessHandle();
      const buf = new Uint8Array(sh.getSize());
      sh.read(buf, { at: 0 });
      sh.close();
      return buf;
    },
    // Borrar una rama entera (memoria y OPFS), de verdad y de una vez: borrar entrada a entrada
    // deja los directorios no vacíos atrás (removeEntry los rechaza), y un /proyecto heredado de
    // la sesión anterior hace que el create siguiente no escriba nada. recursive:true es lo que
    // hace falta, y se borra primero de la memoria para que el árbol no mienta mientras tanto.
    async limpiar(prefijo) {
      const a = absoluta(String(prefijo));
      const tramos = tramosDe(a);
      const nombre = tramos.pop();
      const afectados = [...raiz.keys()].filter((k) => k === a || k.startsWith(a + "/"));
      for (const k of afectados) raiz.delete(k);
      let borradoDelDisco = false;
      try {
        const carpetaPadre = await directorio(tramos, false);
        await carpetaPadre.removeEntry(nombre, { recursive: true });
        borradoDelDisco = true;
      } catch (e) { /* no estaba en el disco: solo se quitó de la memoria */ }
      return { nodos: afectados.length, borradoDelDisco };
    },
    async existe(rutaSimulada) { return !!raiz.get(absoluta(String(rutaSimulada))); },
    lista: () => [...raiz.entries()].filter(([k]) => k !== "/").map(([k, v]) => ({
      ruta: k, dir: v.dir, bytes: v.dir ? 0 : v.data.length, mtimeMs: v.mtimeMs,
    })).sort((x, y) => (x.ruta < y.ruta ? -1 : 1)),
    // una copia de seguridad del árbol, para la prueba de que sobrevive a recargar
    async volcar() {
      const fuera = {};
      for (const [k, v] of raiz.entries()) if (!v.dir && !v.flujo) fuera[k] = v.data.length;
      return fuera;
    },
    // Barrer OPFS y materializar el árbol en memoria. Hace falta porque el proceso dentro de
    // wasm consulta `process.cwd` y `chdir` de forma SÍNCRONA: si el directorio solo está en el
    // disco (porque la página se recargó), chdir diría ENOENT y apkc contestaría "esto no es un
    // proyecto" teniendo el proyecto delante.
    async sincronizar() {
      async function caminar(carpeta, prefijo) {
        for await (const [nombre, mango] of carpeta.entries()) {
          const ruta = prefijo === "/" ? "/" + nombre : prefijo + "/" + nombre;
          const esDir = mango.kind === "directory";
          if (raiz.get(ruta)) {
            if (esDir) await caminar(mango, ruta);
            continue;
          }
          if (esDir) {
            nuevoNodo(ruta, true);
            await caminar(mango, ruta);
          } else {
            // el contenido aún NO está: se lee al abrir o al extraer. Se marca, porque un nodo
            // de 0 bytes descubierto y un nodo de 0 bytes recién truncado no son lo mismo
            const hallado = nuevoNodo(ruta, false);
            hallado.descubierto = true;
          }
        }
      }
      await caminar(carpeta, "/");
    },
    consola,
    fs: fsSim,
  };
  return extras;
}
