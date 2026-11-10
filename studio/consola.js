// La consola de apkc: colores ANSI, enlaces `ruta:línea` y el buffer que pinta la página.
// Vive aparte de pestana.js para que el editor no cargue con todo el orquestador.

function textoHtml(s) {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

// Una salida de javac o del compilador trae `ruta:línea:`; en la consola es un enlace que abre
// el fichero y selecciona esa línea. Se aplica al texto ya escapado, así que las comillas de
// los atributos van como &quot; y no pueden romper el HTML.
const RE_SALTO = /\b([A-Za-z0-9_][A-Za-z0-9_./-]*\.(?:java|kt|xml)):(\d+)\b/g;
function enlazarSalidas(html) {
  return html.replace(RE_SALTO, (todo, ruta, linea) =>
    '<a href="#" class="salto" data-ruta="' + ruta + '" data-linea="' + linea + '">' + todo + "</a>");
}

// La consola de apkc escribe colores ANSI (`\u001b[34m...\u001b[0m`). En un <pre> se veían
// los códigos crudos; aquí se convierten en spans con clase y el resto de secuencias CSI
// (borrar línea, mover cursor) se descartan. Es puro para poder probarlo en node.
function consolaHtml(texto) {
  const limpio = String(texto).replace(/\r\n/g, "\n").replace(/\r/g, "\n")
    .replace(/\u001b\][^\u0007]*(?:\u0007|\u001b\\)/g, "");
  const patron = /\u001b\[([0-9;?]*)([A-Za-z])/g;
  let salida = "", desde = 0, clases = [];
  const anadir = (c) => { if (!clases.includes(c)) clases.push(c); };
  const quitar = (...cs) => { clases = clases.filter((c) => !cs.includes(c)); };
  const color = (prefijo, n) => {
    quitar(...clases.filter((c) => c.startsWith(prefijo)));
    anadir(prefijo + n);
  };
  const envolver = (trozo) => {
    if (!trozo) return "";
    const html = enlazarSalidas(textoHtml(trozo));
    return clases.length ? '<span class="' + clases.join(" ") + '">' + html + "</span>" : html;
  };
  let m;
  while ((m = patron.exec(limpio))) {
    salida += envolver(limpio.slice(desde, m.index));
    desde = patron.lastIndex;
    if (m[2] !== "m") continue;                       // otra CSI: no se pinta
    const codigos = m[1] === "" ? [0] : m[1].split(";").map((x) => Number(x) || 0);
    for (let i = 0; i < codigos.length; i++) {
      const c = codigos[i];
      if (c === 0) clases = [];
      else if (c === 1) anadir("consola-fuerte");
      else if (c === 2) anadir("consola-tenue");
      else if (c === 3) anadir("consola-cursiva");
      else if (c === 4) anadir("consola-subrayado");
      else if (c === 22) quitar("consola-fuerte", "consola-tenue");
      else if (c === 23) quitar("consola-cursiva");
      else if (c === 24) quitar("consola-subrayado");
      else if (c === 39) quitar(...clases.filter((x) => x.startsWith("consola-fg-")));
      else if (c === 49) quitar(...clases.filter((x) => x.startsWith("consola-bg-")));
      else if (c >= 30 && c <= 37) color("consola-fg-", c - 30);
      else if (c >= 90 && c <= 97) color("consola-fg-", c - 90 + 8);
      else if (c >= 40 && c <= 47) color("consola-bg-", c - 40);
      else if (c >= 100 && c <= 107) color("consola-bg-", c - 100 + 8);
      else if (c === 38 || c === 48) {
        // colores extendidos: se consume la secuencia para que no quede el número suelto
        const modo = codigos[++i];
        if (modo === 5) i++;
        else if (modo === 2) i += 3;
      }
    }
  }
  salida += envolver(limpio.slice(desde));
  return salida;
}

// Las operaciones con OPFS (leer un apk para el blob, sacar un jar de 26 MB o abrir un
// fichero grande) no son la compilación, pero tardan y antes no decían nada. Se muestra un
// velo con una rueda solo si pasan de 120 ms, para no parpadear en las rápidas.
let cargaSecuencia = 0;
function mostrarCarga(texto) {
  const velo = document.getElementById("cargando");
  const aviso = document.getElementById("cargando-texto");
  if (aviso) aviso.textContent = texto;
  if (velo) velo.hidden = false;
}
function ocultarCarga() {
  const velo = document.getElementById("cargando");
  if (velo) velo.hidden = true;
}
async function conCarga(texto, tarea) {
  const id = ++cargaSecuencia;
  const temporizador = setTimeout(() => { if (id === cargaSecuencia) mostrarCarga(texto); }, 120);
  try {
    return await tarea();
  } finally {
    clearTimeout(temporizador);
    if (id === cargaSecuencia) ocultarCarga();
  }
}

const CONSOLA = {
  texto: "",
  escribir(trozo) {
    this.texto += String(trozo);
    if (this.texto.length > 300000) this.texto = this.texto.slice(-300000);
    pintarConsola();
  },
};

function pintarConsola() {
  const cuadro = document.getElementById("consola-pestana");
  if (!cuadro) return;
  cuadro.innerHTML = consolaHtml(CONSOLA.texto);
  cuadro.scrollTop = cuadro.scrollHeight;
}

export { textoHtml, consolaHtml, enlazarSalidas, mostrarCarga, ocultarCarga, conCarga,
  CONSOLA, pintarConsola };
