// Resaltado y detección de XML del editor: funciones puras, sin DOM, para poder probarlas
// en node y para que el orquestador no cargue con la tabla de patrones.
import { textoHtml } from "./consola.js";

// Un fuente Java puede llevar genéricos (`List<String>`, `Map<K, V>`) y la heurística vieja
// los tomaba por etiquetas XML: el fichero entero se pintaba con las reglas equivocadas. Lo
// que distingue a un recurso Android es que, quitando BOM, espacios y comentarios, empieza
// por una etiqueta; un .java empieza por package, import, un comentario o una anotación.
function esXml(texto) {
  const t = texto.slice(0, 400);
  const limpio = t.replace(/^\uFEFF/, "").trimStart().replace(/^(?:<!--[^]*?-->\s*)+/, "");
  if (/^<\?xml/i.test(limpio)) return true;
  return /^<\/?[A-Za-z][\w.:-]*/.test(limpio);
}

const PALABRAS_JAVA = "abstract|assert|break|byte|case|catch|char|class|const|continue|default|do|double|else|enum|extends|final|finally|float|for|goto|if|implements|import|instanceof|int|interface|long|native|new|package|private|protected|public|record|return|sealed|short|static|strictfp|super|switch|synchronized|this|throw|throws|transient|try|var|void|volatile|while|yield|null|true|false";

// Un solo pasado de regex por línea, no una cadena de replace: encadenar replace sobre el HTML
// que el paso anterior ya inyectó coloreaba los nombres de las propias clases CSS (`class`
// dentro de `<span class=...>`), que es exactamente el fallo que hacía del resaltador un
// decorado roto en cuanto había un comentario.
// Los literales no cruzan de línea: si un fichero se queda sin cerrar una comilla, un `[^]*?`
// se comería el resto del archivo y el editor quedaría gris de ahí en adelante.
const PAST_JAVA = new RegExp(
  "(\\/\\/[^\\n]*|/\\*[^]*?\\*/|&quot;[^\\n]*?&quot;|'[^'\\n]*')" +
  "|(@[A-Za-z_]\\w*)" +
  "|\\b(" + PALABRAS_JAVA + ")\\b" +
  "|\\b(0[xX][0-9a-fA-F]+|\\d+(?:\\.\\d+)?[fFdDlL]?)\\b", "g");
const PAST_XML = /(&lt;!--[^]*?--&gt;)|(&lt;\/?)([\w.:-]+)|(&quot;[^\n]*?&quot;)|([\w.:-]+)(?==)/g;

function resaltar(texto, modo) {
  const escapado = textoHtml(texto);
  if (modo === "xml") {
    return escapado.replace(PAST_XML, (todo, comentario, apertura, etiqueta, valor, atributo) => {
      if (comentario) return '<span class="comentario">' + comentario + "</span>";
      if (etiqueta !== undefined) return apertura + '<span class="etiqueta">' + etiqueta + "</span>";
      if (valor) return '<span class="cadena">' + valor + "</span>";
      if (atributo) return '<span class="atributo">' + atributo + "</span>";
      return todo;
    });
  }
  return escapado.replace(PAST_JAVA, (todo, literal, anotacion, palabra, numero) => {
    if (literal) {
      const clase = literal.startsWith("&quot;") || literal.startsWith("'") ? "cadena" : "comentario";
      return '<span class="' + clase + '">' + literal + "</span>";
    }
    if (anotacion) return '<span class="anotacion">' + anotacion + "</span>";
    if (palabra) return '<span class="palabra">' + palabra + "</span>";
    if (numero) return '<span class="numero">' + numero + "</span>";
    return todo;
  });
}

export { esXml, resaltar };
