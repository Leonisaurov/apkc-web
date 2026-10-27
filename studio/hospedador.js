// El worker que hace correr apkc.wasm en la pestaña: da el fs con forma de Node que exige
// Go (fs-fragmento.js, respaldado en OPFS), carga el binario y ejecuta un comando cada vez.
//
// Por qué worker y no hilo principal: los access handles sincrónicos de OPFS solo existen en
// un DedicatedWorker, y es lo que permite escribir en una posición concreta como hacen las
// syscalls de Go. Sin esto la página tendría un fs de mentira en memoria y perdería el
// proyecto al recargar.
//
// wasm_exec.js viene del GOROOT de quien construyó el binario y lo sirve servidor.js desde
// /wasm_exec.js: no se copia ni se versiona (regla 2 del repo).
import { crearFs } from "./fs-fragmento.js";

let sistema = null;            // { fs, montar, extraer, lista, ... } — se asigna AL FINAL del init
let bytesModule = null;        // los bytes del wasm, una sola bajada; la instancia no se reutiliza
let errorWasmExec = null;      // el motivo por el que no se pudo cargar wasm_exec.js
let ocupado = false;
let cola = [];
let entregarRaiz = null;
const raizPrometida = new Promise((resuelve) => { entregarRaiz = resuelve; });

async function iniciar() {
  if (sistema) return;
  if (!crossOriginIsolated) {
    // sin COOP/COEP el handle sincrónico no está disponible; el síntoma es un TypeError dentro
    // del fs, que parece un fallo de apkc
    throw new Error("la página no es crossOriginIsolated: hay que abrirla por http con las cabeceras de servidor.js");
  }
  // navigator.storage vive en el hilo principal y en el worker es undefined (comprobado en este
  // chromium), mientras que createSyncAccessHandle solo existe en el worker: la página baja el
  // mango de OPFS y nos lo pasa por mensaje, que es lo que permite espejar los bytes
  const raiz = await raizPrometida;
  const nuevo = crearFs(raiz);

  // el fs y el process que Go consulta: hay que sustituirlos antes de ejecutar nada
  globalThis.fs = nuevo.fs;
  globalThis.path = {
    resolve: (...partes) => {
      const uno = partes.length ? partes.join("/") : nuevo.cwd();
      return uno.startsWith("/") ? uno : "/" + uno.replace(/^\/+/, "");
    },
    isAbsolute: (p) => String(p).startsWith("/"),
    dirname: (p) => { const a = String(p), i = a.lastIndexOf("/"); return i <= 0 ? "/" : a.slice(0, i); },
    basename: (p) => String(p).slice(String(p).lastIndexOf("/") + 1),
    sep: "/",
    join: (...p) => "/" + p.join("/").replace(/\/+/g, "/").replace(/^\/+/, ""),
  };
  globalThis.process = {
    cwd: () => nuevo.cwd(),
    chdir: (p) => nuevo.chdir(p),
    argv: [], env: {}, exitCode: 0,
    getuid: () => 1000, getgid: () => 1000, geteuid: () => 1000, getegid: () => 1000,
    getgroups: () => [], umask: () => 0o22,
    platform: "browser", arch: "wasm",
    on: () => {}, exit: () => {},
    stdout: { write: (b) => nuevo.consola.escribir(String(b)) },
    stderr: { write: (b) => nuevo.consola.escribir(String(b)) },
  };
  // performance y crypto NO se reasignan: en un worker de módulo ya son globales del
  // WorkerGlobalScope y son de solo lectura (asignarlos lanza "has only a getter", y ese error
  // tapaba por completo el arranque). El shim de node sí necesita ponerlos, porque allí no hay
  // entorno de navegador.
  await nuevo.sincronizar();          // el árbol que ya estaba en OPFS, dentro de la memoria
  await cargarWasmExec();

  // se asigna aqui y no al principio: mientras el arranque esté a medias, un mensaje que llegue
  // tiene que volver a pasar por iniciar(). Asignar antes del import dejaba `sistema` puesto sin
  // Go cargado, y el fallo aparecía veinte mensajes tarde como "Go is not defined"
  sistema = nuevo;
}

// wasm_exec.js es un script (asigna globalThis.Go dentro de un IIFE), no un módulo: se importa
// igual porque no exporta nada y su efecto es esa asignación; importScripts() no existe en un
// worker de tipo module. El motivo de un fallo se guarda para que el sondeo lo pueda decir.
async function cargarWasmExec() {
  if (typeof globalThis.Go === "function") return;
  try {
    await import("/wasm_exec.js");
  } catch (e) {
    errorWasmExec = "el import de /wasm_exec.js falló: " + String((e && e.message) || e);
    throw e;
  }
  if (typeof globalThis.Go !== "function") {
    errorWasmExec = "/wasm_exec.js se importó pero no definió globalThis.Go";
    throw new Error(errorWasmExec);
  }
  errorWasmExec = null;
}

// un comando por vez: el binario de Go en wasm no es reentrante, así que lo que llegue mientras
// corre espera su turno en lugar de pisar el fs compartido
function enCola(tarea) {
  // la cola se despierta al encolar, no solo al terminar: la primera tarea de la página se
  // quedaba dentro para siempre si nadie la empujaba (la prueba colgaba sin decir nada)
  return new Promise((resuelve) => { cola.push({ tarea, resuelve }); descolocar(); });
}

function descolocar() {
  if (ocupado || !cola.length) return;
  ocupado = true;
  const { tarea, resuelve } = cola.shift();
  tarea()
    .then(resuelve, (e) => resuelve({ ok: false, error: String((e && e.message) || e) }))
    .then(() => { ocupado = false; descolocar(); });
}

async function ejecutar(argv, env) {
  const go = new Go();
  go.argv = ["apkc", ...argv];
  go.env = { HOME: "/home/usuario", ANDROID_HOME: "/android", JAVA_HOME: "/jvm", ...(env || {}) };
  let codigo = null;
  // run() espera _exitPromise y quien lo resuelve es la implementación POR DEFECTO de this.exit:
  // sustituir go.exit sin resolverla deja la promesa sin terminar (el mismo error que en el shim)
  go.exit = (c) => {
    codigo = c;
    // descartar los temporizadores pendientes, como en el shim: reanidar un programa que ya
    // salio lanza "Go program has already exited" dentro del worker
    if (go._scheduledTimeouts) {
      for (const id of go._scheduledTimeouts.values()) clearTimeout(id);
      go._scheduledTimeouts.clear();
    }
    go._resolveExitPromise();
  };

  if (!bytesModule) {
    const respuesta = await fetch("/apkc.wasm");
    if (!respuesta.ok) throw new Error("apkc.wasm no está servido (levanta servidor.js con APKC_WASM)");
    bytesModule = new Uint8Array(await respuesta.arrayBuffer());
  }
  // la instancia se crea CADA VEZ: un binario de Go en wasm no puede volver a correrse sobre la
  // misma instancia después de salir, así que lo que se guarda son los bytes, no la instancia
  const { instance } = await WebAssembly.instantiate(bytesModule.slice(0), go.importObject);
  await go.run(instance);
  return { ok: true, codigo: codigo === null ? 1 : codigo, consola: sistema.consola.volcar() };
}

self.onmessage = async (evento) => {
  const m = evento.data || {};
  try {
    // antes de cualquier init: iniciar() espera precisamente este mensaje, y procesarlo después
    // del await sería un bloqueo mutuo (la página no recibiría nunca respuesta)
    if (m.tipo === "raiz") {
      entregarRaiz(m.raiz);
      return void postMessage({ id: m.id, tipo: "raiz-recibida" });
    }
    await iniciar();
    if (m.tipo === "montar") {
      const r = await enCola(() => sistema.montar(m.ruta, new Uint8Array(m.datos)));
      // un fallo de la cola NO son bytes: entregarlo como si lo fuera convierte un error del
      // fs en un TypeError de TextDecoder tres pasos más lejos, que es imposible de leer
      if (r && r.ok === false) return void postMessage({ id: m.id, tipo: "error", error: r.error });
      return void postMessage({ id: m.id, tipo: "montado", ruta: m.ruta, bytes: r });
    }
    if (m.tipo === "leer") {
      const r = await enCola(() => sistema.extraer(m.ruta));
      if (r && r.ok === false) return void postMessage({ id: m.id, tipo: "error", error: r.error });
      // la tarea devuelve LOS BYTES, no un envoltorio: pedir r.bytes aquí era devolver siempre
      // null y que la página viera ficheros vacíos
      return void postMessage({ id: m.id, tipo: "contenido", ruta: m.ruta, bytes: r || null });
    }
    if (m.tipo === "cwd") {               // apkc build no recibe ruta: trabaja donde esté el proceso
      try { sistema.chdir(m.ruta); } catch (e) { return void postMessage({ id: m.id, tipo: "error", error: String(e && e.message || e) }); }
      return void postMessage({ id: m.id, tipo: "cwd", ruta: sistema.cwd() });
    }
    if (m.tipo === "limpiar") {
      const r = await enCola(() => sistema.limpiar(m.ruta));
      return void postMessage({ id: m.id, tipo: "limpiado", ruta: m.ruta, ...r });
    }
    if (m.tipo === "arbol") {
      return void postMessage({ id: m.id, tipo: "arbol", nodos: sistema.lista(), aislada: crossOriginIsolated });
    }
    if (m.tipo === "ejecutar") {
      const r = await enCola(() => ejecutar(m.argv, m.env));
      // el resultado de la cola puede ser {ok:false,error}: eso es terminación con fallo, no
      // un fallo del hospedador; codigo=-1 lo deja claro para quien lea la consola
      const conCodigo = r && r.ok === false ? { ...r, codigo: -1 } : r;
      return void postMessage({ id: m.id, tipo: "terminado", ...conCodigo });
    }
    if (m.tipo === "sondeo") {
      return void postMessage({
        id: m.id, tipo: "sondeo",
        haySistema: !!sistema, tipoGo: typeof globalThis.Go, aislada: crossOriginIsolated,
        errorWasmExec, bytesWasm: bytesModule ? bytesModule.length : 0,
      });
    }
    postMessage({ id: m.id, tipo: "error", error: "mensaje sin tipo: " + m.tipo });
  } catch (e) {
    postMessage({ id: m.id, tipo: "error", error: String((e && e.message) || e) });
  }
};

postMessage({ tipo: "listo" });
