// Exportar e importar el proyecto como `.zip`, sin dependencias y sin backend.
//
// El zip de salida es el mismo escritor STORE de `app.js` (el que ya usan las pruebas), ahora
// con entradas binarias: un PNG de `res/drawable` tiene que salir byte a byte. El de entrada
// se lee del directorio central y admite STORE y DEFLATE; `DecompressionStream("deflate-raw")`
// es del navegador, no de una librería.
import { makeZip, crc32 } from "./app.js";

const FIRMA_LOCAL = 0x04034b50;
const FIRMA_CENTRAL = 0x02014b50;
const FIRMA_EOCD = 0x06054b50;

// Una ruta del zip vale si es relativa, usa `/` y no tiene tramos que salgan del proyecto.
export function rutaSegura(nombre) {
  const ruta = String(nombre || "").replace(/\/+$/, "");
  if (!ruta || ruta.startsWith("/") || ruta.includes("\\") || ruta.includes("\0")) return false;
  return ruta.split("/").every((tramo) => tramo && tramo !== "." && tramo !== "..");
}

// El paquete del manifiesto es el nombre natural del zip y de la carpeta al descomprimir.
export function paqueteDelManifiesto(texto) {
  const m = String(texto || "").match(/\bpackage\s*=\s*"([^"]+)"/);
  return m ? m[1] : "";
}

export function nombreDeZip(paquete) {
  const limpio = String(paquete || "").replace(/[^A-Za-z0-9._-]+/g, "-").replace(/^-+|-+$/g, "");
  return (limpio || "proyecto") + ".zip";
}

// --- ajustes del proyecto -------------------------------------------------------------------
// Los ajustes viven en el propio AndroidManifest.xml: no hay un fichero paralelo que se pueda
// desincronizar ni un estado que perder al recargar. `conAjustes` solo toca los atributos que
// pide y deja el resto del XML como estaba.
const atributoDe = (texto, nombre) => {
  const m = String(texto || "").match(new RegExp("\\b" + nombre + '\\s*=\\s*"([^"]*)"'));
  return m ? m[1] : "";
};

export function ajustesDelManifiesto(texto) {
  return {
    package: atributoDe(texto, "package"),
    versionCode: atributoDe(texto, "android:versionCode"),
    versionName: atributoDe(texto, "android:versionName"),
    minSdk: atributoDe(texto, "android:minSdkVersion"),
    targetSdk: atributoDe(texto, "android:targetSdkVersion"),
  };
}

function conAtributo(texto, etiqueta, nombre, valor) {
  const encontrada = String(texto).match(etiqueta);
  if (!encontrada) return texto;
  const actual = encontrada[0];
  const conValor = actual.replace(
    new RegExp("(\\b" + nombre + '\\s*=\\s*")[^"]*(")'), `$1${valor}$2`);
  if (conValor !== actual) return String(texto).replace(actual, conValor);
  return String(texto).replace(actual, actual.replace(/\s*(\/?)>$/, ` ${nombre}="${valor}"$1>`));
}

export function conAjustes(texto, cambios) {
  let s = String(texto || "");
  if (!/<manifest\b/.test(s)) throw new Error("el texto no parece un AndroidManifest.xml");
  const entero = (v, min, max, nombre) => {
    const n = Number(v);
    if (!Number.isInteger(n) || n < min || n > max) {
      throw new Error(nombre + " tiene que ser un entero entre " + min + " y " + max);
    }
    return n;
  };
  const versionCode = entero(cambios.versionCode, 1, 2147483647, "versionCode");
  const minSdk = entero(cambios.minSdk, 21, 35, "minSdk");
  const targetSdk = entero(cambios.targetSdk, 21, 35, "targetSdk");
  if (targetSdk < minSdk) throw new Error("targetSdk no puede ser menor que minSdk");
  const versionName = String(cambios.versionName || "").trim();
  if (!versionName || /["<>&]/.test(versionName) || versionName.length > 64) {
    throw new Error("versionName no puede ir vacío ni llevar «, <, > o &");
  }
  if (!/<uses-sdk\b/.test(s)) {
    s = s.replace(/(<manifest\b[^>]*>)/, "$1\n    <uses-sdk />");
  }
  s = conAtributo(s, /<manifest\b[^>]*>/, "android:versionCode", versionCode);
  s = conAtributo(s, /<manifest\b[^>]*>/, "android:versionName", versionName);
  s = conAtributo(s, /<uses-sdk\b[^>]*>/, "android:minSdkVersion", minSdk);
  s = conAtributo(s, /<uses-sdk\b[^>]*>/, "android:targetSdkVersion", targetSdk);
  // maxSdkVersion fija la app a una API máxima para siempre y no lo pide nadie al publicar
  s = s.replace(/\s*android:maxSdkVersion\s*=\s*"[^"]*"/, "");
  return s;
}

// entradas: [[ruta, Uint8Array | string], ...]; las carpetas van con `/` al final.
export function zipDelArbol(entradas) {
  for (const [ruta] of entradas) {
    if (!rutaSegura(ruta)) throw new Error("ruta no exportable: " + ruta);
  }
  return makeZip(entradas);
}

function dato(bytes) {
  return new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
}

function buscaEocd(vista) {
  const desde = Math.max(0, vista.byteLength - 65557);
  for (let i = vista.byteLength - 22; i >= desde; i--) {
    if (vista.getUint32(i, true) === FIRMA_EOCD) return i;
  }
  return -1;
}

async function inflar(bytes) {
  const flujo = new Blob([bytes]).stream().pipeThrough(new DecompressionStream("deflate-raw"));
  return new Uint8Array(await new Response(flujo).arrayBuffer());
}

// Devuelve [{ ruta, dir, datos }]. Los directorios llevan datos null.
export async function leerZip(bytes) {
  const vista = dato(bytes);
  const eocd = buscaEocd(vista);
  if (eocd < 0) throw new Error("no es un zip: falta el fin del directorio central");
  const total = vista.getUint16(eocd + 10, true);
  const inicioCentral = vista.getUint32(eocd + 16, true);
  if (total === 0xffff || inicioCentral === 0xffffffff) {
    throw new Error("el zip usa ZIP64, que esta página no lee; vuelve a exportarlo sin ZIP64");
  }
  const salida = [];
  let at = inicioCentral;
  for (let n = 0; n < total; n++) {
    if (at + 46 > vista.byteLength || vista.getUint32(at, true) !== FIRMA_CENTRAL) {
      throw new Error("el directorio central del zip está roto en la entrada " + (n + 1));
    }
    const banderas = vista.getUint16(at + 8, true);
    const metodo = vista.getUint16(at + 10, true);
    const crc = vista.getUint32(at + 16, true);
    const comprimido = vista.getUint32(at + 20, true);
    const tamano = vista.getUint32(at + 24, true);
    const largoNombre = vista.getUint16(at + 28, true);
    const largoExtra = vista.getUint16(at + 30, true);
    const largoComentario = vista.getUint16(at + 32, true);
    const local = vista.getUint32(at + 42, true);
    const nombre = new TextDecoder().decode(bytes.subarray(at + 46, at + 46 + largoNombre));
    at += 46 + largoNombre + largoExtra + largoComentario;

    if (banderas & 0x0001) throw new Error("el zip está cifrado: " + nombre);
    if (!rutaSegura(nombre)) throw new Error("ruta insegura en el zip: " + nombre);
    if (nombre.endsWith("/")) {
      salida.push({ ruta: nombre, dir: true, datos: null });
      continue;
    }
    if (local + 30 > vista.byteLength || vista.getUint32(local, true) !== FIRMA_LOCAL) {
      throw new Error("la cabecera local del zip está rota: " + nombre);
    }
    const nombreLocal = vista.getUint16(local + 26, true);
    const extraLocal = vista.getUint16(local + 28, true);
    const desde = local + 30 + nombreLocal + extraLocal;
    const crudo = bytes.subarray(desde, desde + comprimido);
    if (crudo.length !== comprimido) throw new Error("el zip está truncado: " + nombre);

    let datos;
    if (metodo === 0) datos = crudo;
    else if (metodo === 8) datos = await inflar(crudo);
    else throw new Error("método de compresión no soportado (" + metodo + "): " + nombre);

    if (tamano !== datos.length) {
      throw new Error("el tamaño de " + nombre + " no cuadra: " + datos.length + " de " + tamano);
    }
    if (crc32(datos) !== crc) throw new Error("el CRC de " + nombre + " no cuadra");

    salida.push({ ruta: nombre, dir: false, datos });
  }
  return salida;
}
