// Árbol de deshacer por documento. Cada nodo guarda un delta contra su padre (o un checkpoint
// completo cada cierto nivel), de modo que 200 versiones de un fichero no son 200 copias.
//
// - Se agrupa una edición cada pausa de tecleo; no hay un nodo por tecla.
// - Deshacer + escribir crea una rama nueva; las ramas anteriores se conservan.
// - Persistencia en IndexedDB, por ventana y ruta. La ventana sale de sessionStorage en pestana.js.
// - Podar elimina hojas antiguas, nunca la raíz ni los ancestros del nodo actual.
//
// Es un script clásico para no romper el orden actual del sitio; exporta lo puro para node.
export const Historial = (() => {
  const VERSION = 1;
  const MAX_NODOS = 240;
  const MAX_BYTES = 4 * 1024 * 1024;
  const CHECKPOINT_CADA = 24;

  function crear(ruta, texto) {
    const ahora = Date.now();
    const raiz = {
      id: "n1", p: null, h: [], c: String(texto || ""), ts: ahora,
      au: "local", a: [0, 0], s: [0, 0],
    };
    return {
      v: VERSION, ruta, raiz: "n1", actual: "n1", next: 2,
      nodos: { n1: raiz }, cache: raiz.c,
    };
  }

  // Delta mínimo entre dos textos: un solo tramo, que cubre la edición agrupada habitual.
  function delta(antes, despues) {
    const a = String(antes || ""), b = String(despues || "");
    let i = 0;
    const max = Math.min(a.length, b.length);
    while (i < max && a.charCodeAt(i) === b.charCodeAt(i)) i++;
    let finA = a.length, finB = b.length;
    while (finA > i && finB > i && a.charCodeAt(finA - 1) === b.charCodeAt(finB - 1)) {
      finA--; finB--;
    }
    return { i, b: finA - i, s: b.slice(i, finB) };
  }

  function profundidad(arbol, id) {
    let n = arbol.nodos[id], d = 0;
    while (n && n.p) { n = arbol.nodos[n.p]; d++; }
    return d;
  }

  // Reconstruye el texto de un nodo subiendo hasta el checkpoint más cercano y aplicando deltas.
  function textoDe(arbol, id) {
    if (!arbol || !arbol.nodos[id]) return "";
    const camino = [];
    let n = arbol.nodos[id];
    while (n) {
      camino.push(n);
      if (n.c !== undefined) break;
      n = arbol.nodos[n.p];
    }
    let texto = n && n.c !== undefined ? n.c : "";
    for (let i = camino.length - 1; i >= 0; i--) {
      const nodo = camino[i];
      if (nodo.c !== undefined || !nodo.d) continue;
      texto = texto.slice(0, nodo.d.i) + nodo.d.s + texto.slice(nodo.d.i + nodo.d.b);
    }
    return texto;
  }

  function agregar(arbol, texto, opciones) {
    if (!arbol) return null;
    const padre = arbol.nodos[arbol.actual];
    if (!padre) return null;
    const nuevoTexto = String(texto === undefined ? "" : texto);
    const antes = arbol.cache !== undefined ? arbol.cache : textoDe(arbol, arbol.actual);
    if (antes === nuevoTexto) return null;
    const d = delta(antes, nuevoTexto);
    const nivel = profundidad(arbol, arbol.actual);
    const checkpoint = d.s.length > Math.max(2048, antes.length * 0.6) ||
      (nivel > 0 && nivel % CHECKPOINT_CADA === 0);
    const id = "n" + arbol.next++;
    const nodo = {
      id, p: arbol.actual, h: [], ts: Date.now(),
      au: (opciones && opciones.autor) || "local",
      a: (opciones && opciones.seleccion) || [0, 0],
      s: (opciones && opciones.scroll) || [0, 0],
    };
    if (checkpoint) nodo.c = nuevoTexto;
    else nodo.d = d;
    padre.h.push(id);
    arbol.nodos[id] = nodo;
    arbol.actual = id;
    arbol.cache = nuevoTexto;
    podar(arbol);
    return id;
  }

  function deshacer(arbol) {
    if (!arbol) return null;
    const nodo = arbol.nodos[arbol.actual];
    if (!nodo || !nodo.p) return null;
    arbol.actual = nodo.p;
    arbol.cache = textoDe(arbol, arbol.actual);
    return arbol.nodos[arbol.actual];
  }

  function rehacer(arbol) {
    if (!arbol) return null;
    const nodo = arbol.nodos[arbol.actual];
    if (!nodo || !nodo.h.length) return null;
    arbol.actual = nodo.h[nodo.h.length - 1];
    arbol.cache = textoDe(arbol, arbol.actual);
    return arbol.nodos[arbol.actual];
  }

  function restaurar(arbol, id) {
    if (!arbol || !arbol.nodos[id]) return null;
    arbol.actual = id;
    arbol.cache = textoDe(arbol, id);
    return arbol.nodos[id];
  }

  function recorrer(arbol) {
    if (!arbol || !arbol.nodos[arbol.raiz]) return [];
    const salida = [];
    const visitar = (id, nivel) => {
      const nodo = arbol.nodos[id];
      if (!nodo) return;
      salida.push({ ...nodo, nivel });
      for (const hijo of nodo.h || []) visitar(hijo, nivel + 1);
    };
    visitar(arbol.raiz, 0);
    return salida;
  }

  function caminoActual(arbol) {
    const camino = new Set();
    let n = arbol && arbol.nodos[arbol.actual];
    while (n) { camino.add(n.id); n = n.p ? arbol.nodos[n.p] : null; }
    return camino;
  }

  function peso(arbol) {
    let bytes = 0;
    for (const nodo of Object.values(arbol.nodos || {})) {
      bytes += 48;
      if (nodo.c) bytes += nodo.c.length * 2;
      else if (nodo.d) bytes += (nodo.d.s || "").length * 2 + 16;
    }
    return bytes;
  }

  // Si toda la historia es una cadena larga, no hay hojas que podar. Se re-enraíza en un
  // checkpoint de la propia cadena: se pierde lo más antiguo, no lo reciente.
  function rebase(arbol) {
    const camino = [];
    let n = arbol.nodos[arbol.actual];
    while (n) { camino.push(n); n = n.p ? arbol.nodos[n.p] : null; }
    camino.reverse();
    if (camino.length <= MAX_NODOS - 10) return;
    const desde = Math.max(1, camino.length - (MAX_NODOS - 10));
    const nuevo = camino[desde];
    if (!nuevo.c) {
      nuevo.c = textoDe(arbol, nuevo.id);
      delete nuevo.d;
    }
    if (nuevo.p) {
      const padre = arbol.nodos[nuevo.p];
      if (padre) padre.h = padre.h.filter((id) => id !== nuevo.id);
    }
    nuevo.p = null;
    arbol.raiz = nuevo.id;
    const vivos = new Set();
    const visitar = (id) => {
      const nodo = arbol.nodos[id];
      if (!nodo) return;
      vivos.add(id);
      for (const hijo of nodo.h || []) visitar(hijo);
    };
    visitar(nuevo.id);
    for (const id of Object.keys(arbol.nodos)) if (!vivos.has(id)) delete arbol.nodos[id];
  }

  function podar(arbol) {
    if (!arbol) return;
    rebase(arbol);
    const camino = caminoActual(arbol);
    let nodos = Object.keys(arbol.nodos).length;
    let bytes = peso(arbol);
    while (nodos > MAX_NODOS || bytes > MAX_BYTES) {
      let victima = null;
      for (const nodo of Object.values(arbol.nodos)) {
        if (!nodo.h.length && nodo.p && !camino.has(nodo.id)) {
          if (!victima || nodo.ts < victima.ts) victima = nodo;
        }
      }
      if (!victima) break;
      const padre = arbol.nodos[victima.p];
      if (padre) padre.h = padre.h.filter((id) => id !== victima.id);
      delete arbol.nodos[victima.id];
      nodos--;
      bytes = peso(arbol);
    }
  }

  // --- IndexedDB ---------------------------------------------------------------------------
  let dbPromesa = null;
  function abrirDB() {
    if (dbPromesa) return dbPromesa;
    dbPromesa = new Promise((resolver) => {
      try {
        if (typeof indexedDB === "undefined") return resolver(null);
        const peticion = indexedDB.open("apkc-studio-historial", 1);
        peticion.onupgradeneeded = () => {
          const db = peticion.result;
          if (!db.objectStoreNames.contains("arboles")) {
            db.createObjectStore("arboles", { keyPath: "clave" });
          }
        };
        peticion.onsuccess = () => resolver(peticion.result);
        peticion.onerror = () => resolver(null);
      } catch (e) { resolver(null); }
    });
    return dbPromesa;
  }

  const clave = (ventana, ruta) => String(ventana || "v0") + "|" + String(ruta || "");
  const transaccion = (db, modo) => db.transaction("arboles", modo).objectStore("arboles");

  async function cargar(ventana, ruta) {
    const db = await abrirDB();
    if (!db) return null;
    return new Promise((resolver) => {
      try {
        const peticion = transaccion(db, "readonly").get(clave(ventana, ruta));
        peticion.onsuccess = () => {
          const dato = peticion.result;
          const arbol = dato && dato.arbol;
          if (!arbol || arbol.v !== VERSION || !arbol.nodos || !arbol.nodos[arbol.actual]) {
            resolver(null); return;
          }
          // cache no se persiste: se recalcula al usar el árbol
          delete arbol.cache;
          resolver(arbol);
        };
        peticion.onerror = () => resolver(null);
      } catch (e) { resolver(null); }
    });
  }

  async function guardar(ventana, arbol) {
    const db = await abrirDB();
    if (!db || !arbol) return false;
    return new Promise((resolver) => {
      try {
        const copia = { ...arbol };
        delete copia.cache;                    // se reconstruye; no ocupa espacio
        const peticion = transaccion(db, "readwrite").put({
          clave: clave(ventana, arbol.ruta), ruta: arbol.ruta,
          ventana, actualizado: Date.now(), arbol: copia,
        });
        peticion.onsuccess = () => resolver(true);
        peticion.onerror = () => resolver(false);
      } catch (e) { resolver(false); }
    });
  }

  async function borrar(ventana, ruta) {
    const db = await abrirDB();
    if (!db) return false;
    return new Promise((resolver) => {
      try {
        const peticion = transaccion(db, "readwrite").delete(clave(ventana, ruta));
        peticion.onsuccess = () => resolver(true);
        peticion.onerror = () => resolver(false);
      } catch (e) { resolver(false); }
    });
  }

  async function renombrar(ventana, de, a) {
    const arbol = await cargar(ventana, de);
    if (!arbol) return false;
    arbol.ruta = a;
    const guardado = await guardar(ventana, arbol);
    if (guardado) await borrar(ventana, de);
    return guardado;
  }

  // Mueve todas las historias de una carpeta o fichero en una sola pasada, sin depender de que
  // el documento esté abierto.
  async function renombrarBajo(ventana, de, a) {
    const db = await abrirDB();
    if (!db) return false;
    const prefijo = String(ventana) + "|" + String(de);
    return new Promise((resolver) => {
      try {
        const almacen = transaccion(db, "readwrite");
        const peticion = almacen.getAll();
        peticion.onsuccess = () => {
          for (const dato of peticion.result || []) {
            if (!String(dato.clave).startsWith(prefijo)) continue;
            const sufijo = String(dato.clave).slice(prefijo.length);
            const nuevaRuta = String(a) + sufijo;
            dato.clave = String(ventana) + "|" + nuevaRuta;
            dato.ruta = nuevaRuta;
            dato.arbol.ruta = nuevaRuta;
            almacen.put(dato);
            almacen.delete(String(ventana) + "|" + String(de) + sufijo);
          }
        };
        const tx = almacen.transaction;
        tx.oncomplete = () => resolver(true);
        tx.onerror = () => resolver(false);
      } catch (e) { resolver(false); }
    });
  }

  async function limpiar(ventana) {
    const db = await abrirDB();
    if (!db) return false;
    return new Promise((resolver) => {
      try {
        const almacen = transaccion(db, "readwrite");
        const peticion = almacen.getAllKeys();
        peticion.onsuccess = () => {
          for (const k of peticion.result || []) {
            if (String(k).startsWith(String(ventana) + "|")) almacen.delete(k);
          }
        };
        const tx = almacen.transaction;
        tx.oncomplete = () => resolver(true);
        tx.onerror = () => resolver(false);
      } catch (e) { resolver(false); }
    });
  }

  return {
    crear, delta, agregar, deshacer, rehacer, restaurar,
    textoDe, recorrer, caminoActual, podar, peso,
    cargar, guardar, borrar, renombrar, renombrarBajo, limpiar,
    MAX_NODOS, MAX_BYTES,
  };
})();

if (typeof window !== "undefined") window.Historial = Historial;
