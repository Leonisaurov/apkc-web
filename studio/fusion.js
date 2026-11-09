// Fusión de tres vías para texto: base (última versión de OPFS), nuestro (buffer local) y suyo
// (lo que guardó otra ventana). Los cambios que no se pisan se combinan; si dos ramas tocan las
// mismas líneas se devuelve conflicto y la interfaz decide.
//
// Es puro y se prueba en node; no conoce DOM, OPFS ni el editor.

const MAX_LINEAS = 2000;

function lineas(texto) {
  return String(texto === undefined || texto === null ? "" : texto).split("\n");
}

function tablaLCS(base, otro) {
  const n = base.length, m = otro.length, ancho = m + 1;
  const lcs = new Uint32Array((n + 1) * ancho);
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      lcs[i * ancho + j] = base[i] === otro[j]
        ? lcs[(i + 1) * ancho + j + 1] + 1
        : Math.max(lcs[(i + 1) * ancho + j], lcs[i * ancho + j + 1]);
    }
  }
  return { lcs, ancho };
}

// Devuelve tramos {ini, fin, lineas}: sustituye base[ini..fin) por lineas.
function cambios(base, otro) {
  const { lcs, ancho } = tablaLCS(base, otro);
  let i = 0, j = 0, actual = null;
  const salida = [];
  const cerrar = () => {
    if (!actual) return;
    actual.fin = actual.ini + (actual.borradas || 0);
    salida.push(actual);
    actual = null;
  };
  while (i < base.length || j < otro.length) {
    if (i < base.length && j < otro.length && base[i] === otro[j]) {
      cerrar();
      i++; j++;
    } else if (j >= otro.length ||
      (i < base.length && lcs[(i + 1) * ancho + j] >= lcs[i * ancho + j + 1])) {
      if (!actual) actual = { ini: i, fin: i, lineas: [], borradas: 0 };
      actual.borradas++;
      i++;
    } else {
      if (!actual) actual = { ini: i, fin: i, lineas: [], borradas: 0 };
      actual.lineas.push(otro[j]);
      j++;
    }
  }
  cerrar();
  return salida;
}

function combinar(base, nuestros, suyos) {
  const todos = [
    ...nuestros.map((h) => ({ ...h, lado: "nuestro" })),
    ...suyos.map((h) => ({ ...h, lado: "suyo" })),
  ].sort((a, b) => (a.ini - b.ini) || (a.fin - b.fin));

  const salida = [];
  const conflictos = [];
  let pos = 0;
  let insercionAnterior = null;

  for (const h of todos) {
    if (h.ini < pos) {
      conflictos.push({ ini: h.ini, fin: h.fin, lado: h.lado });
      continue;
    }
    if (insercionAnterior && insercionAnterior.ini === h.ini && h.fin > h.ini) {
      conflictos.push({ ini: h.ini, fin: h.fin, lado: h.lado });
      continue;
    }
    if (h.ini === pos && h.fin === pos && insercionAnterior && insercionAnterior.ini === h.ini) {
      if (JSON.stringify(insercionAnterior.lineas) === JSON.stringify(h.lineas)) continue;
      conflictos.push({ ini: h.ini, fin: h.fin, lado: h.lado });
      continue;
    }
    salida.push(...base.slice(pos, h.ini), ...h.lineas);
    pos = h.fin;
    insercionAnterior = h.fin === h.ini ? h : null;
  }
  if (conflictos.length) return { conflicto: true, conflictos };
  salida.push(...base.slice(pos));
  return { conflicto: false, texto: salida.join("\n") };
}

export function fusionar(base, nuestro, suyo) {
  const b = String(base === undefined || base === null ? "" : base);
  const o = String(nuestro === undefined || nuestro === null ? "" : nuestro);
  const s = String(suyo === undefined || suyo === null ? "" : suyo);
  if (o === b) return { estado: "suyo", texto: s };
  if (s === b) return { estado: "nuestro", texto: o };
  if (o === s) return { estado: "igual", texto: o };

  const lineasBase = lineas(b), lineasNuestras = lineas(o), lineasSuyas = lineas(s);
  if (lineasBase.length > MAX_LINEAS || lineasNuestras.length > MAX_LINEAS ||
      lineasSuyas.length > MAX_LINEAS) {
    return { estado: "conflicto", texto: o, conflictos: [{ ini: 0, fin: lineasBase.length }] };
  }

  const hn = cambios(lineasBase, lineasNuestras);
  const hs = cambios(lineasBase, lineasSuyas);
  if (!hn.length) return { estado: "suyo", texto: s };
  if (!hs.length) return { estado: "nuestro", texto: o };

  const combinado = combinar(lineasBase, hn, hs);
  if (combinado.conflicto) {
    return { estado: "conflicto", texto: o, conflictos: combinado.conflictos };
  }
  return { estado: "auto", texto: combinado.texto };
}

export default fusionar;
