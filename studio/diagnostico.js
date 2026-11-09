// El panel de diagnóstico de Inicio: pega la salida de apkc y la convierte en qué pasó y qué
// hacer. Las reglas viven en app.js (puras y probadas contra salidas reales en prueba.js); aquí
// solo se pinta. El texto pegado no sale del navegador: los contadores quedan en localStorage.
import { analyze, informe, registrar } from "./app.js";

(() => {
  const caja = document.getElementById("detalle-diagnostico");
  if (!caja) return;
  const entrada = document.getElementById("salida-apkc");
  const boton = document.getElementById("analizar-salida");
  const limpiar = document.getElementById("limpiar-salida");
  const resultado = document.getElementById("resultado-analisis");
  const enlaceInforme = document.getElementById("descargar-informe");
  if (!entrada || !boton || !resultado) return;

  const nodo = (etiqueta, clase, texto) => {
    const n = document.createElement(etiqueta);
    if (clase) n.className = clase;
    if (texto !== undefined) n.textContent = texto;
    return n;
  };

  let informeUrl = null;
  function ofrecerInforme() {
    if (!enlaceInforme) return;
    if (informeUrl) URL.revokeObjectURL(informeUrl);
    informeUrl = URL.createObjectURL(new Blob([informe()], { type: "text/markdown;charset=utf-8" }));
    enlaceInforme.href = informeUrl;
    enlaceInforme.download = "informe-apkc.md";
    enlaceInforme.hidden = false;
  }

  function pintar() {
    const texto = entrada.value || "";
    resultado.textContent = "";
    resultado.hidden = false;
    if (!texto.trim()) {
      resultado.appendChild(nodo("p", "nota",
        "Pega la salida de apkc (la consola del panel o la de tu terminal) y pulsa Analizar."));
      return;
    }
    const { hits, desconocidas } = analyze(texto);
    if (!hits.length && !desconocidas.length) {
      resultado.appendChild(nodo("p", "nota",
        "No reconozco ningún mensaje de apkc en lo pegado. Prueba con la salida completa del comando."));
      return;
    }
    resultado.appendChild(nodo("p", "nota",
      hits.length + (hits.length === 1 ? " regla reconocida" : " reglas reconocidas") +
      (desconocidas.length ? " · " + desconocidas.length + " línea(s) sin explicar" : "")));

    for (const h of hits) {
      registrar("regla", h.id);
      const ficha = nodo("article", "hallazgo");
      const titulo = nodo("h4");
      titulo.appendChild(nodo("span", "capa capa-" + h.capa, h.capa));
      titulo.appendChild(nodo("code", null, h.id));
      ficha.appendChild(titulo);
      ficha.appendChild(nodo("p", "que", h.que));
      ficha.appendChild(nodo("p", "hacer", "Qué hacer: " + h.hacer));
      ficha.appendChild(nodo("pre", "linea", h.linea));
      resultado.appendChild(ficha);
    }

    if (desconocidas.length) {
      const bloque = nodo("div", "desconocidas");
      bloque.appendChild(nodo("h4", null, "Líneas que no conozco"));
      const lista = nodo("ul");
      for (const linea of desconocidas) {
        const li = nodo("li");
        li.appendChild(nodo("code", null, linea));
        lista.appendChild(li);
      }
      bloque.appendChild(lista);
      bloque.appendChild(nodo("p", "nota",
        "No las invento: este panel solo explica lo que tiene una regla medida. Si el error se " +
        "repite, ese texto es el que hay que llevar a quien mantiene apkc."));
      resultado.appendChild(bloque);
    }
    ofrecerInforme();
  }

  boton.addEventListener("click", (e) => {
    e.preventDefault();
    try { pintar(); }
    catch (err) {
      resultado.hidden = false;
      resultado.textContent = "no se pudo analizar: " + (err && err.message ? err.message : err);
    }
  });

  if (limpiar) limpiar.addEventListener("click", () => {
    entrada.value = "";
    resultado.textContent = "";
    resultado.hidden = true;
    if (enlaceInforme) enlaceInforme.hidden = true;
  });

  // Ctrl+Enter dentro del área analiza; el atajo global de la página no se toca.
  entrada.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) { e.preventDefault(); boton.click(); }
  });
})();
